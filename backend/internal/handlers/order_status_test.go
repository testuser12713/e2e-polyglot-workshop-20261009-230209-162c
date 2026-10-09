package handlers_test

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"

	"workshop/internal/config"
	"workshop/internal/deps"
	"workshop/internal/queue"
	"workshop/internal/server"
	"workshop/internal/store"
)

// osTestAlphabet is the order-number alphabet: uppercase A-Z and digits.
const osTestAlphabet = "ABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"

// osTestCode returns n random uppercase-alphanumeric characters, so every run
// seeds fresh rows without colliding with rows an earlier run left behind. The
// production database starts empty, but the tests must not depend on that.
func osTestCode(n int) string {
	raw := make([]byte, n)
	if _, err := rand.Read(raw); err != nil {
		panic(fmt.Sprintf("read random bytes: %v", err))
	}
	out := make([]byte, n)
	for i, b := range raw {
		out[i] = osTestAlphabet[int(b)%len(osTestAlphabet)]
	}
	return string(out)
}

func osTestEmail(local string) string {
	return local + "-" + osTestCode(10) + "@example.test"
}

// osTestEnv is the real routing table wired to the real PostgreSQL and Valkey
// from the environment, with one signed-in employee.
type osTestEnv struct {
	srv   *httptest.Server
	pool  *pgxpool.Pool
	rdb   *redis.Client
	token string
}

func newOSTestEnv(t *testing.T) *osTestEnv {
	t.Helper()

	dbURL := os.Getenv("DATABASE_URL")
	valkeyURL := os.Getenv("VALKEY_URL")
	if dbURL == "" || valkeyURL == "" {
		t.Skip("DATABASE_URL and VALKEY_URL must be set to exercise the status endpoint")
	}

	ctx := context.Background()

	pool, err := store.NewPool(ctx, dbURL)
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

	q, err := queue.New(ctx, valkeyURL)
	if err != nil {
		t.Fatalf("connect to Valkey: %v", err)
	}
	t.Cleanup(func() { _ = q.Close() })

	deps.Set(&deps.Deps{
		DB:    pool,
		Queue: q,
		Cfg: &config.Config{
			DatabaseURL: dbURL,
			ValkeyURL:   valkeyURL,
			WebOrigin:   config.DefaultWebOrigin,
			APIPort:     "0",
		},
	})

	srv := httptest.NewServer(server.New(deps.Get()))
	t.Cleanup(srv.Close)

	opt, err := redis.ParseURL(valkeyURL)
	if err != nil {
		t.Fatalf("parse VALKEY_URL: %v", err)
	}
	rdb := redis.NewClient(opt)
	t.Cleanup(func() { _ = rdb.Close() })

	// A signed-in employee, so the request passes the real auth middleware.
	token := "status-test-token-" + osTestCode(16)
	sum := sha256.Sum256([]byte(token))
	email := osTestEmail("status-employee")

	var employeeID string
	if err := pool.QueryRow(ctx,
		`INSERT INTO employees (email, password_hash, name) VALUES ($1, $2, $3) RETURNING id::text`,
		email, "not-used", "Testerin",
	).Scan(&employeeID); err != nil {
		t.Fatalf("insert employee: %v", err)
	}
	if _, err := pool.Exec(ctx,
		`INSERT INTO sessions (token_hash, employee_id, expires_at) VALUES ($1, $2, $3)`,
		hex.EncodeToString(sum[:]), employeeID, time.Now().Add(time.Hour),
	); err != nil {
		t.Fatalf("insert session: %v", err)
	}

	return &osTestEnv{srv: srv, pool: pool, rdb: rdb, token: token}
}

// createOrder inserts a customer, a vehicle and an order in its initial
// "requested" state and returns the order number.
func (e *osTestEnv) createOrder(t *testing.T) string {
	t.Helper()
	ctx := context.Background()

	var customerID string
	if err := e.pool.QueryRow(ctx,
		`INSERT INTO customers (name, email, phone) VALUES ($1, $2, $3) RETURNING id::text`,
		"Testkunde", osTestEmail("customer"), "0000",
	).Scan(&customerID); err != nil {
		t.Fatalf("insert customer: %v", err)
	}

	var vehicleID string
	if err := e.pool.QueryRow(ctx,
		`INSERT INTO vehicles (customer_id, plate, brand, model, mileage_km)
		 VALUES ($1, $2, $3, $4, $5) RETURNING id::text`,
		customerID, "OS-"+osTestCode(6), "VW", "Golf", 1000,
	).Scan(&vehicleID); err != nil {
		t.Fatalf("insert vehicle: %v", err)
	}

	orderNumber := "AU-" + osTestCode(6)
	if _, err := e.pool.Exec(ctx,
		`INSERT INTO orders (order_number, customer_id, vehicle_id, status, desired_date, description)
		 VALUES ($1, $2, $3, 'requested', CURRENT_DATE, $4)`,
		orderNumber, customerID, vehicleID, "Testschaden",
	); err != nil {
		t.Fatalf("insert order: %v", err)
	}
	return orderNumber
}

func (e *osTestEnv) postStatus(t *testing.T, orderNumber, status string) *http.Response {
	t.Helper()
	body := strings.NewReader(fmt.Sprintf(`{"status":%q}`, status))
	req, err := http.NewRequest(http.MethodPost, e.srv.URL+"/api/orders/"+orderNumber+"/status", body)
	if err != nil {
		t.Fatalf("build request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+e.token)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("POST status: %v", err)
	}
	return resp
}

func (e *osTestEnv) storedStatus(t *testing.T, orderNumber string) string {
	t.Helper()
	var status string
	if err := e.pool.QueryRow(context.Background(),
		`SELECT status FROM orders WHERE order_number = $1`, orderNumber,
	).Scan(&status); err != nil {
		t.Fatalf("read stored status: %v", err)
	}
	return status
}

func (e *osTestEnv) historyCount(t *testing.T, orderNumber string) int {
	t.Helper()
	var count int
	if err := e.pool.QueryRow(context.Background(),
		`SELECT count(*) FROM order_history h
		 JOIN orders o ON o.id = h.order_id
		 WHERE o.order_number = $1`, orderNumber,
	).Scan(&count); err != nil {
		t.Fatalf("count history: %v", err)
	}
	return count
}

// invoiceMessagesFor counts the queue messages that carry this order number, so
// it is exact even when the shared list already holds other orders' messages.
func (e *osTestEnv) invoiceMessagesFor(t *testing.T, orderNumber string) int {
	t.Helper()
	values, err := e.rdb.LRange(context.Background(), queue.InvoicesKey, 0, -1).Result()
	if err != nil {
		t.Fatalf("read %s list: %v", queue.InvoicesKey, err)
	}
	count := 0
	for _, raw := range values {
		var msg queue.InvoiceMessage
		if err := json.Unmarshal([]byte(raw), &msg); err == nil && msg.OrderNumber == orderNumber {
			count++
		}
	}
	return count
}

type osTestStatusBody struct {
	OrderNumber string               `json:"order_number"`
	Status      string               `json:"status"`
	History     []store.StatusChange `json:"history"`
}

// TestOrderStatusAllowedChain drives requested -> confirmed -> in_progress ->
// done -> picked_up: every step is accepted, recorded in the history with a
// timestamp, and only the step to done enqueues exactly one invoice message.
func TestOrderStatusAllowedChain(t *testing.T) {
	env := newOSTestEnv(t)
	orderNumber := env.createOrder(t)

	chain := []string{"confirmed", "in_progress", "done", "picked_up"}
	for i, target := range chain {
		resp := env.postStatus(t, orderNumber, target)
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("step %d (%s): got %d, want 200", i, target, resp.StatusCode)
		}

		var body osTestStatusBody
		if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
			resp.Body.Close()
			t.Fatalf("step %d (%s): decode body: %v", i, target, err)
		}
		resp.Body.Close()

		if body.OrderNumber != orderNumber {
			t.Fatalf("step %d: order_number = %q, want %q", i, body.OrderNumber, orderNumber)
		}
		if body.Status != target {
			t.Fatalf("step %d: status = %q, want %q", i, body.Status, target)
		}
		if len(body.History) != i+1 {
			t.Fatalf("step %d: history has %d entries, want %d", i, len(body.History), i+1)
		}
		last := body.History[len(body.History)-1]
		if last.Status != target {
			t.Fatalf("step %d: history last status = %q, want %q", i, last.Status, target)
		}
		if last.ChangedAt.IsZero() || last.ChangedAt.After(time.Now().Add(time.Minute)) {
			t.Fatalf("step %d: history timestamp is not a plausible now: %v", i, last.ChangedAt)
		}

		if target == "done" {
			if n := env.invoiceMessagesFor(t, orderNumber); n != 1 {
				t.Fatalf("after %s: queue holds %d messages for %s, want exactly 1", target, n, orderNumber)
			}
		}
	}

	if env.storedStatus(t, orderNumber) != "picked_up" {
		t.Fatalf("final stored status = %q, want picked_up", env.storedStatus(t, orderNumber))
	}
	if n := env.invoiceMessagesFor(t, orderNumber); n != 1 {
		t.Fatalf("after the full chain: queue holds %d messages for %s, want exactly 1", n, orderNumber)
	}
}

// TestOrderStatusIllegalJumpKeepsStatus proves requested -> done is rejected
// with 409 invalid_transition, leaves the stored status at requested, writes no
// history entry and enqueues nothing (AC-04).
func TestOrderStatusIllegalJumpKeepsStatus(t *testing.T) {
	env := newOSTestEnv(t)
	orderNumber := env.createOrder(t)

	resp := env.postStatus(t, orderNumber, "done")
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusConflict {
		t.Fatalf("requested -> done: got %d, want 409", resp.StatusCode)
	}

	var errBody struct {
		Error struct {
			Code    string `json:"code"`
			Message string `json:"message"`
		} `json:"error"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&errBody); err != nil {
		t.Fatalf("decode error body: %v", err)
	}
	if errBody.Error.Code != "invalid_transition" {
		t.Fatalf("error code = %q, want invalid_transition", errBody.Error.Code)
	}

	if got := env.storedStatus(t, orderNumber); got != "requested" {
		t.Fatalf("stored status = %q, want requested (unchanged)", got)
	}
	if got := env.historyCount(t, orderNumber); got != 0 {
		t.Fatalf("history entries = %d, want 0", got)
	}
	if n := env.invoiceMessagesFor(t, orderNumber); n != 0 {
		t.Fatalf("queue holds %d messages for %s, want 0", n, orderNumber)
	}
}

// TestOrderStatusRejectsUnknownValue proves a value outside the five statuses is
// a malformed request (400) and changes nothing.
func TestOrderStatusRejectsUnknownValue(t *testing.T) {
	env := newOSTestEnv(t)
	orderNumber := env.createOrder(t)

	resp := env.postStatus(t, orderNumber, "cancelled")
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("unknown status: got %d, want 400", resp.StatusCode)
	}
	if got := env.storedStatus(t, orderNumber); got != "requested" {
		t.Fatalf("stored status = %q, want requested (unchanged)", got)
	}
}

// TestOrderStatusUnknownOrderAnswersNotFound proves an unknown order number is a
// 404 rather than a silent success.
func TestOrderStatusUnknownOrderAnswersNotFound(t *testing.T) {
	env := newOSTestEnv(t)

	resp := env.postStatus(t, "AU-ZZZZZZ", "confirmed")
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("unknown order: got %d, want 404", resp.StatusCode)
	}
}
