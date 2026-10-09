package handlers_test

import (
	"context"
	"crypto/rand"
	"encoding/json"
	"math/big"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"workshop/internal/deps"
	"workshop/internal/handlers"
	"workshop/internal/store"
)

// --- wire shapes (the shared interface), decoded independently of the store
// types so the test pins the exact JSON field names. ---

type listEntry struct {
	OrderNumber  string    `json:"order_number"`
	Status       string    `json:"status"`
	DesiredDate  time.Time `json:"desired_date"`
	Plate        string    `json:"plate"`
	CustomerName string    `json:"customer_name"`
}

type customerDTO struct {
	ID    string `json:"id"`
	Name  string `json:"name"`
	Email string `json:"email"`
	Phone string `json:"phone"`
}

type vehicleDTO struct {
	ID         string `json:"id"`
	CustomerID string `json:"customer_id"`
	Plate      string `json:"plate"`
	Brand      string `json:"brand"`
	Model      string `json:"model"`
	MileageKM  int    `json:"mileage_km"`
}

type itemDTO struct {
	ID             string   `json:"id"`
	Kind           string   `json:"kind"`
	Description    string   `json:"description"`
	Hours          *float64 `json:"hours"`
	Quantity       *int     `json:"quantity"`
	UnitPriceCents *int     `json:"unit_price_cents"`
}

type historyDTO struct {
	Status    string    `json:"status"`
	ChangedAt time.Time `json:"changed_at"`
}

type invoiceItemDTO struct {
	Description    string  `json:"description"`
	Quantity       float64 `json:"quantity"`
	UnitPriceCents int     `json:"unit_price_cents"`
}

type invoiceDTO struct {
	InvoiceNumber string           `json:"invoice_number"`
	IssuedAt      time.Time        `json:"issued_at"`
	NetCents      int              `json:"net_cents"`
	VatCents      int              `json:"vat_cents"`
	GrossCents    int              `json:"gross_cents"`
	Items         []invoiceItemDTO `json:"items"`
}

type orderDetailDTO struct {
	OrderNumber string       `json:"order_number"`
	Status      string       `json:"status"`
	DesiredDate time.Time    `json:"desired_date"`
	Description string       `json:"description"`
	Customer    customerDTO  `json:"customer"`
	Vehicle     vehicleDTO   `json:"vehicle"`
	Items       []itemDTO    `json:"items"`
	History     []historyDTO `json:"history"`
	Invoice     *invoiceDTO  `json:"invoice"`
}

// readFixture describes the rows one test seeds for itself.
type readFixture struct {
	customerID string
	vehicleAID string
	vehicleBID string
	plateA     string
	plateB     string
	plateATag  string
	plateBTag  string
	orderA     string // in_progress, plateA, with items/history/invoice
	orderB     string // requested, plateA
	orderC     string // requested, plateB, no invoice
}

// randCode returns n uppercase A-Z/0-9 characters.
func randCode(t *testing.T, n int) string {
	t.Helper()
	const chars = "ABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	b := make([]byte, n)
	for i := range b {
		v, err := rand.Int(rand.Reader, big.NewInt(int64(len(chars))))
		if err != nil {
			t.Fatalf("rand: %v", err)
		}
		b[i] = chars[v.Int64()]
	}
	return string(b)
}

// readTestDB connects to PostgreSQL, applies the schema and installs the shared
// deps. It skips when DATABASE_URL is not set.
func readTestDB(t *testing.T) *pgxpool.Pool {
	t.Helper()

	databaseURL := os.Getenv("DATABASE_URL")
	if databaseURL == "" {
		t.Skip("DATABASE_URL must be set to run the order read tests")
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

	deps.Set(&deps.Deps{DB: pool})
	return pool
}

// seedOrders inserts an isolated set of rows and removes them again when the
// test ends. The unique tag keeps this slice independent of every other test.
func seedOrders(t *testing.T, pool *pgxpool.Pool) readFixture {
	t.Helper()
	ctx := context.Background()

	fx := readFixture{
		plateATag: randCode(t, 6),
		plateBTag: randCode(t, 6),
		orderA:    "AU-" + randCode(t, 6),
		orderB:    "AU-" + randCode(t, 6),
		orderC:    "AU-" + randCode(t, 6),
	}
	fx.plateA = "AB-" + fx.plateATag
	fx.plateB = "XY-" + fx.plateBTag

	if err := pool.QueryRow(ctx,
		`INSERT INTO customers (name, email, phone) VALUES ($1, $2, $3) RETURNING id::text`,
		"Testkunde "+fx.plateATag, "kunde-"+fx.plateATag+"@example.com", "0123-"+fx.plateATag,
	).Scan(&fx.customerID); err != nil {
		t.Fatalf("insert customer: %v", err)
	}
	if err := pool.QueryRow(ctx,
		`INSERT INTO vehicles (customer_id, plate, brand, model, mileage_km) VALUES ($1, $2, $3, $4, $5) RETURNING id::text`,
		fx.customerID, fx.plateA, "VW", "Golf", 120000,
	).Scan(&fx.vehicleAID); err != nil {
		t.Fatalf("insert vehicle A: %v", err)
	}
	if err := pool.QueryRow(ctx,
		`INSERT INTO vehicles (customer_id, plate, brand, model, mileage_km) VALUES ($1, $2, $3, $4, $5) RETURNING id::text`,
		fx.customerID, fx.plateB, "Opel", "Corsa", 90000,
	).Scan(&fx.vehicleBID); err != nil {
		t.Fatalf("insert vehicle B: %v", err)
	}

	desired := time.Date(2026, time.November, 2, 0, 0, 0, 0, time.UTC)
	insertOrder := func(orderNumber, vehicleID, status, description string) string {
		t.Helper()
		var id string
		if err := pool.QueryRow(ctx,
			`INSERT INTO orders (order_number, customer_id, vehicle_id, status, desired_date, description)
			 VALUES ($1, $2, $3, $4, $5, $6) RETURNING id::text`,
			orderNumber, fx.customerID, vehicleID, status, desired, description,
		).Scan(&id); err != nil {
			t.Fatalf("insert order %s: %v", orderNumber, err)
		}
		return id
	}
	orderAID := insertOrder(fx.orderA, fx.vehicleAID, "in_progress", "Bremsen quietschen")
	insertOrder(fx.orderB, fx.vehicleAID, "requested", "Ölwechsel")
	insertOrder(fx.orderC, fx.vehicleBID, "requested", "Reifenwechsel")

	if _, err := pool.Exec(ctx,
		`INSERT INTO order_items (order_id, kind, description, hours, quantity, unit_price_cents)
		 VALUES ($1, 'labor', $2, $3, NULL, NULL)`,
		orderAID, "Bremsen prüfen", 2.5,
	); err != nil {
		t.Fatalf("insert labor item: %v", err)
	}
	if _, err := pool.Exec(ctx,
		`INSERT INTO order_items (order_id, kind, description, hours, quantity, unit_price_cents)
		 VALUES ($1, 'part', $2, NULL, $3, $4)`,
		orderAID, "Bremsbelag", 3, 4999,
	); err != nil {
		t.Fatalf("insert part item: %v", err)
	}

	if _, err := pool.Exec(ctx,
		`INSERT INTO order_history (order_id, status, changed_at) VALUES
		 ($1, 'requested', now() - interval '3 days'),
		 ($1, 'confirmed', now() - interval '2 days'),
		 ($1, 'in_progress', now() - interval '1 day')`,
		orderAID,
	); err != nil {
		t.Fatalf("insert history: %v", err)
	}

	var invoiceID string
	if err := pool.QueryRow(ctx,
		`INSERT INTO invoices (order_id, invoice_number, issued_at, net_cents, vat_cents, gross_cents)
		 VALUES ($1, $2, now(), $3, $4, $5) RETURNING id::text`,
		orderAID, "RE-"+randCode(t, 8), 20000, 3800, 23800,
	).Scan(&invoiceID); err != nil {
		t.Fatalf("insert invoice: %v", err)
	}
	if _, err := pool.Exec(ctx,
		`INSERT INTO invoice_items (invoice_id, description, quantity, unit_price_cents) VALUES
		 ($1, 'Bremsen prüfen', 2.5, 5000),
		 ($1, 'Bremsbelag', 3, 4999)`,
		invoiceID,
	); err != nil {
		t.Fatalf("insert invoice items: %v", err)
	}

	t.Cleanup(func() {
		cleanupCtx := context.Background()
		_, _ = pool.Exec(cleanupCtx, `DELETE FROM orders WHERE order_number = ANY($1)`,
			[]string{fx.orderA, fx.orderB, fx.orderC})
		_, _ = pool.Exec(cleanupCtx, `DELETE FROM vehicles WHERE id = ANY($1)`,
			[]string{fx.vehicleAID, fx.vehicleBID})
		_, _ = pool.Exec(cleanupCtx, `DELETE FROM customers WHERE id = $1`, fx.customerID)
	})
	return fx
}

func getList(t *testing.T, rawQuery string) []listEntry {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, "/api/orders?"+rawQuery, nil)
	rec := httptest.NewRecorder()
	handlers.ListOrders(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("GET /api/orders?%s: got %d, want 200 (body %s)", rawQuery, rec.Code, rec.Body.String())
	}
	var entries []listEntry
	if err := json.Unmarshal(rec.Body.Bytes(), &entries); err != nil {
		t.Fatalf("decode list: %v (body %s)", err, rec.Body.String())
	}
	return entries
}

func getDetail(t *testing.T, orderNumber string) *orderDetailDTO {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, "/api/orders/"+orderNumber, nil)
	req.SetPathValue("nr", orderNumber)
	rec := httptest.NewRecorder()
	handlers.GetOrder(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("GET /api/orders/%s: got %d, want 200 (body %s)", orderNumber, rec.Code, rec.Body.String())
	}
	var detail orderDetailDTO
	if err := json.Unmarshal(rec.Body.Bytes(), &detail); err != nil {
		t.Fatalf("decode detail: %v (body %s)", err, rec.Body.String())
	}
	return &detail
}

// TestOrderListFiltersByStatusAndPlate covers AC-15: the list returns only the
// orders matching the requested status and plate substring.
func TestOrderListFiltersByStatusAndPlate(t *testing.T) {
	pool := readTestDB(t)
	fx := seedOrders(t, pool)

	t.Run("status filter", func(t *testing.T) {
		entries := getList(t, "status=in_progress&plate=")
		if len(entries) == 0 {
			t.Fatal("status=in_progress returned no orders")
		}
		seenA := false
		for _, e := range entries {
			if e.Status != "in_progress" {
				t.Errorf("entry %s has status %q, want in_progress", e.OrderNumber, e.Status)
			}
			if e.OrderNumber == fx.orderA {
				seenA = true
				if e.Plate != fx.plateA {
					t.Errorf("order A plate: got %q, want %q", e.Plate, fx.plateA)
				}
				if e.CustomerName == "" {
					t.Error("order A customer_name is empty")
				}
				if e.DesiredDate.Year() != 2026 || e.DesiredDate.Month() != time.November || e.DesiredDate.Day() != 2 {
					t.Errorf("order A desired_date: got %v", e.DesiredDate)
				}
			}
			if e.OrderNumber == fx.orderB || e.OrderNumber == fx.orderC {
				t.Errorf("non-matching order %s leaked into the status filter", e.OrderNumber)
			}
		}
		if !seenA {
			t.Errorf("order A (%s) missing from status=in_progress result", fx.orderA)
		}
	})

	t.Run("plate search", func(t *testing.T) {
		entries := getList(t, "status=&plate="+url.QueryEscape(fx.plateATag))
		if len(entries) == 0 {
			t.Fatalf("plate=%s returned no orders", fx.plateATag)
		}
		seenA, seenB := false, false
		for _, e := range entries {
			if !contains(e.Plate, fx.plateATag) {
				t.Errorf("entry %s has plate %q, which does not contain %q", e.OrderNumber, e.Plate, fx.plateATag)
			}
			switch e.OrderNumber {
			case fx.orderA:
				seenA = true
			case fx.orderB:
				seenB = true
			case fx.orderC:
				t.Errorf("order C with plate %q leaked into the search for %q", fx.plateB, fx.plateATag)
			}
		}
		if !seenA || !seenB {
			t.Errorf("plate search missing orders: orderA=%v orderB=%v", seenA, seenB)
		}
	})
}

// TestOrderDetailAssembly covers the assembled detail: customer, vehicle, items,
// history and the invoice — and a 404 for an unknown order number.
func TestOrderDetailAssembly(t *testing.T) {
	pool := readTestDB(t)
	fx := seedOrders(t, pool)

	t.Run("full order with invoice", func(t *testing.T) {
		d := getDetail(t, fx.orderA)

		if d.OrderNumber != fx.orderA {
			t.Errorf("order_number: got %q, want %q", d.OrderNumber, fx.orderA)
		}
		if d.Status != "in_progress" {
			t.Errorf("status: got %q, want in_progress", d.Status)
		}
		if d.Description != "Bremsen quietschen" {
			t.Errorf("description: got %q", d.Description)
		}
		if d.DesiredDate.Year() != 2026 || d.DesiredDate.Month() != time.November || d.DesiredDate.Day() != 2 {
			t.Errorf("desired_date: got %v", d.DesiredDate)
		}
		if d.Customer.ID != fx.customerID || d.Customer.Name == "" || d.Customer.Email == "" || d.Customer.Phone == "" {
			t.Errorf("customer not assembled: %+v", d.Customer)
		}
		if d.Vehicle.Plate != fx.plateA || d.Vehicle.Brand != "VW" || d.Vehicle.Model != "Golf" || d.Vehicle.MileageKM != 120000 {
			t.Errorf("vehicle not assembled: %+v", d.Vehicle)
		}
		if d.Vehicle.CustomerID != fx.customerID {
			t.Errorf("vehicle.customer_id: got %q, want %q", d.Vehicle.CustomerID, fx.customerID)
		}

		if len(d.Items) != 2 {
			t.Fatalf("items: got %d, want 2 (%+v)", len(d.Items), d.Items)
		}
		var labor, part *itemDTO
		for i := range d.Items {
			switch d.Items[i].Kind {
			case "labor":
				labor = &d.Items[i]
			case "part":
				part = &d.Items[i]
			}
		}
		if labor == nil || labor.Hours == nil || *labor.Hours != 2.5 {
			t.Errorf("labor item hours: %+v", labor)
		}
		if labor != nil && (labor.Quantity != nil || labor.UnitPriceCents != nil) {
			t.Errorf("labor item must have null quantity/unit_price_cents: %+v", labor)
		}
		if part == nil || part.Quantity == nil || *part.Quantity != 3 || part.UnitPriceCents == nil || *part.UnitPriceCents != 4999 {
			t.Errorf("part item: %+v", part)
		}
		if part != nil && part.Hours != nil {
			t.Errorf("part item must have null hours: %+v", part)
		}

		if len(d.History) != 3 {
			t.Fatalf("history: got %d, want 3 (%+v)", len(d.History), d.History)
		}
		wantStatuses := []string{"requested", "confirmed", "in_progress"}
		for i, want := range wantStatuses {
			if d.History[i].Status != want {
				t.Errorf("history[%d]: got %q, want %q", i, d.History[i].Status, want)
			}
			if d.History[i].ChangedAt.IsZero() {
				t.Errorf("history[%d] changed_at is zero", i)
			}
		}

		if d.Invoice == nil {
			t.Fatal("invoice: got null, want the issued invoice")
		}
		if d.Invoice.NetCents != 20000 || d.Invoice.VatCents != 3800 || d.Invoice.GrossCents != 23800 {
			t.Errorf("invoice amounts: %+v", *d.Invoice)
		}
		if d.Invoice.InvoiceNumber == "" || d.Invoice.IssuedAt.IsZero() {
			t.Errorf("invoice identity: %+v", *d.Invoice)
		}
		if len(d.Invoice.Items) != 2 {
			t.Fatalf("invoice items: got %d, want 2 (%+v)", len(d.Invoice.Items), d.Invoice.Items)
		}
	})

	t.Run("order without invoice", func(t *testing.T) {
		d := getDetail(t, fx.orderC)
		if d.Invoice != nil {
			t.Errorf("invoice: got %+v, want null", *d.Invoice)
		}
		if len(d.Items) != 0 {
			t.Errorf("items: got %+v, want empty", d.Items)
		}
		if len(d.History) != 0 {
			t.Errorf("history: got %+v, want empty", d.History)
		}
	})

	t.Run("unknown order number answers 404", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/orders/AU-NOPE99", nil)
		req.SetPathValue("nr", "AU-NOPE99")
		rec := httptest.NewRecorder()
		handlers.GetOrder(rec, req)

		if rec.Code != http.StatusNotFound {
			t.Fatalf("unknown order: got %d, want 404 (body %s)", rec.Code, rec.Body.String())
		}
		var body struct {
			Error struct {
				Code    string `json:"code"`
				Message string `json:"message"`
			} `json:"error"`
		}
		if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
			t.Fatalf("decode error body: %v", err)
		}
		if body.Error.Code != "not_found" || body.Error.Message == "" {
			t.Errorf("error envelope: %+v", body.Error)
		}
	})
}

// contains is a plain substring check (case-insensitive is not needed: the tag
// is uppercase and pgx returns the plate verbatim).
func contains(haystack, needle string) bool {
	for i := 0; i+len(needle) <= len(haystack); i++ {
		if haystack[i:i+len(needle)] == needle {
			return true
		}
	}
	return false
}
