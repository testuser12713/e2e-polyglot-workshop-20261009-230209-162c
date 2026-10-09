package handlers_test

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"math"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"

	"workshop/internal/config"
	"workshop/internal/deps"
	"workshop/internal/handlers"
	"workshop/internal/store"
)

// newItemTestPool connects to the PostgreSQL from the environment, applies the
// real schema and installs the dependencies the handler reads. It skips when the
// database is not available in this shell; the office runs it with one.
func newItemTestPool(t *testing.T) *pgxpool.Pool {
	t.Helper()

	databaseURL := os.Getenv("DATABASE_URL")
	if databaseURL == "" {
		t.Skip("DATABASE_URL must be set to run the order item tests")
	}

	ctx := context.Background()
	pool, err := store.NewPool(ctx, databaseURL)
	if err != nil {
		t.Fatalf("connect to PostgreSQL: %v", err)
	}
	t.Cleanup(pool.Close)

	schema, err := os.ReadFile("../../migrations/0001_init.sql")
	if err != nil {
		t.Fatalf("read migration: %v", err)
	}
	if err := store.ApplyMigrations(ctx, pool, string(schema)); err != nil {
		t.Fatalf("apply migration: %v", err)
	}

	deps.Set(&deps.Deps{
		DB: pool,
		Cfg: &config.Config{
			DatabaseURL: databaseURL,
			WebOrigin:   config.DefaultWebOrigin,
			APIPort:     "0",
		},
	})
	return pool
}

type seededOrder struct {
	orderNumber string
	customerID  string
	vehicleID   string
	orderID     string
}

// seedOrder creates one customer, vehicle and order and removes them again when
// the test ends. Only the rows this test created are deleted.
func seedOrder(t *testing.T, pool *pgxpool.Pool) seededOrder {
	t.Helper()

	ctx := context.Background()
	suffix := randomSuffix(t)
	var seed seededOrder

	err := pool.QueryRow(ctx,
		`INSERT INTO customers (name, email, phone) VALUES ($1, $2, $3) RETURNING id::text`,
		"Testkunde", "test-"+suffix+"@example.com", "0000",
	).Scan(&seed.customerID)
	if err != nil {
		t.Fatalf("seed customer: %v", err)
	}

	err = pool.QueryRow(ctx,
		`INSERT INTO vehicles (customer_id, plate, brand, model, mileage_km)
		 VALUES ($1, $2, $3, $4, $5) RETURNING id::text`,
		seed.customerID, "TEST-"+suffix, "VW", "Golf", 1000,
	).Scan(&seed.vehicleID)
	if err != nil {
		t.Fatalf("seed vehicle: %v", err)
	}

	seed.orderNumber = "AU-" + strings.ToUpper(suffix[:6])
	err = pool.QueryRow(ctx,
		`INSERT INTO orders (order_number, customer_id, vehicle_id, status, desired_date, description)
		 VALUES ($1, $2, $3, 'requested', CURRENT_DATE, $4) RETURNING id::text`,
		seed.orderNumber, seed.customerID, seed.vehicleID, "Testauftrag",
	).Scan(&seed.orderID)
	if err != nil {
		t.Fatalf("seed order: %v", err)
	}

	t.Cleanup(func() {
		c := context.Background()
		_, _ = pool.Exec(c, `DELETE FROM orders WHERE id = $1`, seed.orderID)
		_, _ = pool.Exec(c, `DELETE FROM vehicles WHERE id = $1`, seed.vehicleID)
		_, _ = pool.Exec(c, `DELETE FROM customers WHERE id = $1`, seed.customerID)
	})
	return seed
}

func randomSuffix(t *testing.T) string {
	t.Helper()
	b := make([]byte, 8)
	if _, err := rand.Read(b); err != nil {
		t.Fatalf("random suffix: %v", err)
	}
	return hex.EncodeToString(b)
}

// itemMux wires only the route under test so the test never depends on the
// session of the still-unmerged login ticket.
func itemMux() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("POST /api/orders/{nr}/items", handlers.CreateOrderItem)
	return mux
}

func postItem(t *testing.T, nr, body string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodPost, "/api/orders/"+nr+"/items", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	itemMux().ServeHTTP(rec, req)
	return rec
}

type itemBody struct {
	ID             string   `json:"id"`
	Kind           string   `json:"kind"`
	Description    string   `json:"description"`
	Hours          *float64 `json:"hours"`
	Quantity       *int     `json:"quantity"`
	UnitPriceCents *int     `json:"unit_price_cents"`
}

func decodeItem(t *testing.T, rec *httptest.ResponseRecorder) itemBody {
	t.Helper()
	var item itemBody
	if err := json.Unmarshal(rec.Body.Bytes(), &item); err != nil {
		t.Fatalf("decode item: %v (body %q)", err, rec.Body.String())
	}
	return item
}

func decodeErrorCode(t *testing.T, rec *httptest.ResponseRecorder) string {
	t.Helper()
	var body struct {
		Error struct {
			Code    string `json:"code"`
			Message string `json:"message"`
		} `json:"error"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode error body: %v (body %q)", err, rec.Body.String())
	}
	if body.Error.Code == "" || body.Error.Message == "" {
		t.Fatalf("error envelope incomplete: %q", rec.Body.String())
	}
	return body.Error.Code
}

// TestCreateLaborPosition proves a labor position is stored with its hours and
// returned with its id, and that quantity/unit price stay null (AC-06).
func TestCreateLaborPosition(t *testing.T) {
	pool := newItemTestPool(t)
	seed := seedOrder(t, pool)

	rec := postItem(t, seed.orderNumber, `{"kind":"labor","description":"Ölwechsel","hours":1.5}`)
	if rec.Code != http.StatusCreated {
		t.Fatalf("POST labor item: got %d, want 201 (body %q)", rec.Code, rec.Body.String())
	}

	item := decodeItem(t, rec)
	if item.ID == "" {
		t.Fatal("labor item has no id")
	}
	if item.Kind != "labor" {
		t.Errorf("kind: got %q, want labor", item.Kind)
	}
	if item.Hours == nil || math.Abs(*item.Hours-1.5) > 1e-9 {
		t.Errorf("hours: got %v, want 1.5", item.Hours)
	}
	if item.Quantity != nil || item.UnitPriceCents != nil {
		t.Errorf("labor item must not carry quantity/unit_price_cents, got %v/%v", item.Quantity, item.UnitPriceCents)
	}

	ctx := context.Background()
	var (
		hours float64
		qty   *int
		unit  *int
	)
	err := pool.QueryRow(ctx,
		`SELECT hours::float8, quantity, unit_price_cents
		 FROM order_items WHERE id = $1 AND order_id = $2`,
		item.ID, seed.orderID,
	).Scan(&hours, &qty, &unit)
	if err != nil {
		t.Fatalf("read back labor item: %v", err)
	}
	if math.Abs(hours-1.5) > 1e-9 || qty != nil || unit != nil {
		t.Errorf("stored labor row wrong: hours=%v qty=%v unit=%v", hours, qty, unit)
	}
}

// TestCreatePartPosition proves a part position is stored with quantity and a
// unit price in whole cents and returned with its id; hours stays null (AC-06).
func TestCreatePartPosition(t *testing.T) {
	pool := newItemTestPool(t)
	seed := seedOrder(t, pool)

	rec := postItem(t, seed.orderNumber, `{"kind":"part","description":"Bremsbelag","quantity":2,"unit_price_cents":4599}`)
	if rec.Code != http.StatusCreated {
		t.Fatalf("POST part item: got %d, want 201 (body %q)", rec.Code, rec.Body.String())
	}

	item := decodeItem(t, rec)
	if item.ID == "" {
		t.Fatal("part item has no id")
	}
	if item.Kind != "part" {
		t.Errorf("kind: got %q, want part", item.Kind)
	}
	if item.Quantity == nil || *item.Quantity != 2 {
		t.Errorf("quantity: got %v, want 2", item.Quantity)
	}
	if item.UnitPriceCents == nil || *item.UnitPriceCents != 4599 {
		t.Errorf("unit_price_cents: got %v, want 4599", item.UnitPriceCents)
	}
	if item.Hours != nil {
		t.Errorf("part item must not carry hours, got %v", item.Hours)
	}

	ctx := context.Background()
	var (
		qty   int
		unit  int
		hours *float64
	)
	err := pool.QueryRow(ctx,
		`SELECT quantity, unit_price_cents, hours::float8
		 FROM order_items WHERE id = $1 AND order_id = $2`,
		item.ID, seed.orderID,
	).Scan(&qty, &unit, &hours)
	if err != nil {
		t.Fatalf("read back part item: %v", err)
	}
	if qty != 2 || unit != 4599 {
		t.Errorf("stored part row wrong: quantity=%d unit_price_cents=%d, want 2/4599", qty, unit)
	}
}

// TestOrderDetailReadsBothPositions proves both captured positions belong to the
// order with their kind-specific amounts, so the order detail can carry them.
func TestOrderDetailReadsBothPositions(t *testing.T) {
	pool := newItemTestPool(t)
	seed := seedOrder(t, pool)

	if rec := postItem(t, seed.orderNumber, `{"kind":"labor","description":"Arbeitslohn","hours":2}`); rec.Code != http.StatusCreated {
		t.Fatalf("POST labor: got %d, want 201", rec.Code)
	}
	if rec := postItem(t, seed.orderNumber, `{"kind":"part","description":"Zündkerze","quantity":4,"unit_price_cents":1250}`); rec.Code != http.StatusCreated {
		t.Fatalf("POST part: got %d, want 201", rec.Code)
	}

	ctx := context.Background()
	rows, err := pool.Query(ctx,
		`SELECT kind, hours::float8, quantity, unit_price_cents FROM order_items WHERE order_id = $1 ORDER BY kind`,
		seed.orderID)
	if err != nil {
		t.Fatalf("read order positions: %v", err)
	}
	defer rows.Close()

	var laborHours *float64
	var partQty, partUnit *int
	count := 0
	for rows.Next() {
		var (
			kind  string
			hours *float64
			qty   *int
			unit  *int
		)
		if err := rows.Scan(&kind, &hours, &qty, &unit); err != nil {
			t.Fatalf("scan position: %v", err)
		}
		count++
		switch kind {
		case "labor":
			laborHours = hours
		case "part":
			partQty, partUnit = qty, unit
		}
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("iterate positions: %v", err)
	}

	if count != 2 {
		t.Fatalf("order has %d positions, want 2", count)
	}
	if laborHours == nil || math.Abs(*laborHours-2) > 1e-9 {
		t.Errorf("labor hours in detail: got %v, want 2", laborHours)
	}
	if partQty == nil || *partQty != 4 || partUnit == nil || *partUnit != 1250 {
		t.Errorf("part in detail: got qty=%v unit=%v, want 4/1250", partQty, partUnit)
	}
}

// TestCreateOrderItemRejectsInvalidPayloads proves a payload that does not fit
// its kind answers 400 and is never stored.
func TestCreateOrderItemRejectsInvalidPayloads(t *testing.T) {
	pool := newItemTestPool(t)
	seed := seedOrder(t, pool)

	cases := []struct {
		name string
		body string
	}{
		{"unknown kind", `{"kind":"material","description":"Reifen"}`},
		{"labor without hours", `{"kind":"labor","description":"Arbeit"}`},
		{"labor with zero hours", `{"kind":"labor","description":"Arbeit","hours":0}`},
		{"labor with negative hours", `{"kind":"labor","description":"Arbeit","hours":-1}`},
		{"missing description", `{"kind":"labor","description":"","hours":1}`},
		{"part without quantity", `{"kind":"part","description":"Teil","unit_price_cents":100}`},
		{"part without unit price", `{"kind":"part","description":"Teil","quantity":1}`},
		{"part with zero quantity", `{"kind":"part","description":"Teil","quantity":0,"unit_price_cents":100}`},
		{"part with fractional cents", `{"kind":"part","description":"Teil","quantity":1,"unit_price_cents":12.5}`},
		{"labor with textual hours", `{"kind":"labor","description":"Arbeit","hours":"1.5"}`},
		{"malformed json", `{`},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			rec := postItem(t, seed.orderNumber, tc.body)
			if rec.Code != http.StatusBadRequest {
				t.Fatalf("got %d, want 400 (body %q)", rec.Code, rec.Body.String())
			}
			if code := decodeErrorCode(t, rec); code != "bad_request" {
				t.Errorf("error code: got %q, want bad_request", code)
			}
		})
	}

	ctx := context.Background()
	var count int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM order_items WHERE order_id = $1`, seed.orderID).Scan(&count); err != nil {
		t.Fatalf("count positions: %v", err)
	}
	if count != 0 {
		t.Errorf("invalid payloads were stored: %d rows", count)
	}
}

// TestCreateOrderItemUnknownOrder proves a position for a missing order answers
// 404 with the stable error envelope.
func TestCreateOrderItemUnknownOrder(t *testing.T) {
	newItemTestPool(t)

	rec := postItem(t, "AU-ZZZZZZ", `{"kind":"labor","description":"Arbeit","hours":1}`)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("unknown order: got %d, want 404 (body %q)", rec.Code, rec.Body.String())
	}
	if code := decodeErrorCode(t, rec); code != "not_found" {
		t.Errorf("error code: got %q, want not_found", code)
	}
}
