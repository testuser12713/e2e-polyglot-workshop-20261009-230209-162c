package handlers_test

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"workshop/internal/config"
	"workshop/internal/deps"
	"workshop/internal/server"
	"workshop/internal/store"
)

// dashboardBody is the response shape this test asserts on.
type dashboardBody struct {
	OpenOrders        int   `json:"open_orders"`
	FinishedToday     int   `json:"finished_today"`
	MonthRevenueCents int64 `json:"month_revenue_cents"`
}

// reportEnv is an isolated server: its own PostgreSQL schema, so this test owns
// every row it seeds and never counts a sibling ticket's orders or invoices.
type reportEnv struct {
	srv   *httptest.Server
	pool  *pgxpool.Pool
	token string
}

// newReportEnv creates a throwaway schema, applies the real migration into it,
// installs the shared dependencies and starts the real routing table. It skips
// when DATABASE_URL is unset; the office runs it with a live PostgreSQL.
func newReportEnv(t *testing.T) *reportEnv {
	t.Helper()

	databaseURL := os.Getenv("DATABASE_URL")
	if databaseURL == "" {
		t.Skip("DATABASE_URL must be set to run the dashboard report test")
	}
	ctx := context.Background()

	adminCfg, err := pgxpool.ParseConfig(databaseURL)
	if err != nil {
		t.Fatalf("parse DATABASE_URL: %v", err)
	}
	adminPool, err := pgxpool.NewWithConfig(ctx, adminCfg)
	if err != nil {
		t.Fatalf("admin pool: %v", err)
	}

	schema := fmt.Sprintf("report_test_%d", time.Now().UnixNano())
	if _, err := adminPool.Exec(ctx, "CREATE SCHEMA "+schema); err != nil {
		adminPool.Close()
		t.Fatalf("create schema: %v", err)
	}
	t.Cleanup(func() {
		_, _ = adminPool.Exec(context.Background(), "DROP SCHEMA IF EXISTS "+schema+" CASCADE")
		adminPool.Close()
	})

	cfg, err := pgxpool.ParseConfig(databaseURL)
	if err != nil {
		t.Fatalf("parse DATABASE_URL: %v", err)
	}
	if cfg.ConnConfig.RuntimeParams == nil {
		cfg.ConnConfig.RuntimeParams = map[string]string{}
	}
	cfg.ConnConfig.RuntimeParams["search_path"] = schema
	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		t.Fatalf("connect to schema %s: %v", schema, err)
	}
	t.Cleanup(pool.Close)

	schemaSQL, err := os.ReadFile("../../migrations/0001_init.sql")
	if err != nil {
		t.Fatalf("read migration: %v", err)
	}
	if err := store.ApplyMigrations(ctx, pool, string(schemaSQL)); err != nil {
		t.Fatalf("apply migration: %v", err)
	}

	d := &deps.Deps{
		DB: pool,
		Cfg: &config.Config{
			DatabaseURL: databaseURL,
			WebOrigin:   config.DefaultWebOrigin,
			APIPort:     "0",
		},
	}
	deps.Set(d)

	srv := httptest.NewServer(server.New(d))
	t.Cleanup(srv.Close)

	env := &reportEnv{srv: srv, pool: pool}
	env.token = env.seedSession(t)
	return env
}

func (e *reportEnv) seedSession(t *testing.T) string {
	t.Helper()
	ctx := context.Background()

	const rawToken = "report-test-session-token"
	sum := sha256.Sum256([]byte(rawToken))
	tokenHash := hex.EncodeToString(sum[:])

	var employeeID string
	if err := e.pool.QueryRow(ctx,
		`INSERT INTO employees (email, password_hash, name) VALUES ($1, $2, $3) RETURNING id::text`,
		"tester@example.com", "not-a-real-hash", "Tester",
	).Scan(&employeeID); err != nil {
		t.Fatalf("insert employee: %v", err)
	}
	if _, err := e.pool.Exec(ctx,
		`INSERT INTO sessions (token_hash, employee_id, expires_at) VALUES ($1, $2, $3)`,
		tokenHash, employeeID, time.Now().Add(time.Hour),
	); err != nil {
		t.Fatalf("insert session: %v", err)
	}
	return rawToken
}

func (e *reportEnv) seedVehicle(t *testing.T) string {
	t.Helper()
	ctx := context.Background()

	var customerID string
	if err := e.pool.QueryRow(ctx,
		`INSERT INTO customers (name, email, phone) VALUES ($1, $2, $3) RETURNING id::text`,
		"Anna Test", "anna@example.com", "0123456789",
	).Scan(&customerID); err != nil {
		t.Fatalf("insert customer: %v", err)
	}

	var vehicleID string
	if err := e.pool.QueryRow(ctx,
		`INSERT INTO vehicles (customer_id, plate, brand, model, mileage_km)
		 VALUES ($1, $2, $3, $4, $5) RETURNING id::text`,
		customerID, "B-TS-1234", "VW", "Golf", 120000,
	).Scan(&vehicleID); err != nil {
		t.Fatalf("insert vehicle: %v", err)
	}
	return vehicleID
}

func (e *reportEnv) seedOrder(t *testing.T, vehicleID, number, status string) string {
	t.Helper()
	ctx := context.Background()

	var orderID string
	if err := e.pool.QueryRow(ctx,
		`INSERT INTO orders (order_number, vehicle_id, status, desired_date, description)
		 VALUES ($1, $2, $3, $4, $5) RETURNING id::text`,
		number, vehicleID, status, time.Now(), "Testauftrag",
	).Scan(&orderID); err != nil {
		t.Fatalf("insert order %s: %v", number, err)
	}
	return orderID
}

func (e *reportEnv) seedHistory(t *testing.T, orderID, status string, at time.Time) {
	t.Helper()
	if _, err := e.pool.Exec(context.Background(),
		`INSERT INTO order_history (order_id, status, changed_at) VALUES ($1, $2, $3)`,
		orderID, status, at,
	); err != nil {
		t.Fatalf("insert history: %v", err)
	}
}

func (e *reportEnv) seedInvoice(t *testing.T, orderID, number string, grossCents int, issuedAt time.Time) {
	t.Helper()
	netCents := grossCents * 100 / 119
	vatCents := grossCents - netCents
	if _, err := e.pool.Exec(context.Background(),
		`INSERT INTO invoices (order_id, invoice_number, net_cents, vat_cents, gross_cents, issued_at)
		 VALUES ($1, $2, $3, $4, $5, $6)`,
		orderID, number, netCents, vatCents, grossCents, issuedAt,
	); err != nil {
		t.Fatalf("insert invoice: %v", err)
	}
}

func (e *reportEnv) getDashboard(t *testing.T) dashboardBody {
	t.Helper()
	req, err := http.NewRequest(http.MethodGet, e.srv.URL+"/api/reports/dashboard", nil)
	if err != nil {
		t.Fatalf("build request: %v", err)
	}
	req.Header.Set("Authorization", "Bearer "+e.token)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("GET /api/reports/dashboard: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("GET /api/reports/dashboard: got %d, want 200", resp.StatusCode)
	}

	var body dashboardBody
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatalf("decode dashboard body: %v", err)
	}
	return body
}

// TestDashboardReportCountsOrdersAndInvoices seeds orders across every status and
// invoices in this and a previous month, then asserts the three figures (AC-17).
func TestDashboardReportCountsOrdersAndInvoices(t *testing.T) {
	e := newReportEnv(t)
	vehicle := e.seedVehicle(t)

	// Four orders not yet picked up.
	e.seedOrder(t, vehicle, "AU-REP001", "requested")
	e.seedOrder(t, vehicle, "AU-REP002", "confirmed")
	e.seedOrder(t, vehicle, "AU-REP003", "in_progress")
	doneToday := e.seedOrder(t, vehicle, "AU-REP004", "done")
	// Picked up today: closed, but still finished today.
	pickedToday := e.seedOrder(t, vehicle, "AU-REP005", "picked_up")
	// Picked up long ago and finished in a previous month.
	pickedOld := e.seedOrder(t, vehicle, "AU-REP006", "picked_up")

	now := time.Now()
	old := now.AddDate(0, 0, -40)
	e.seedHistory(t, doneToday, "done", now)
	e.seedHistory(t, pickedToday, "done", now)
	e.seedHistory(t, pickedOld, "done", old)

	e.seedInvoice(t, doneToday, "RE-0001", 5000, now)
	e.seedInvoice(t, pickedToday, "RE-0002", 11900, now)
	e.seedInvoice(t, pickedOld, "RE-0003", 250000, old)

	got := e.getDashboard(t)
	if got.OpenOrders != 4 {
		t.Errorf("open_orders: got %d, want 4", got.OpenOrders)
	}
	if got.FinishedToday != 2 {
		t.Errorf("finished_today: got %d, want 2", got.FinishedToday)
	}
	if got.MonthRevenueCents != 16900 {
		t.Errorf("month_revenue_cents: got %d, want 16900", got.MonthRevenueCents)
	}
}

// TestDashboardReportMonthWithoutInvoicesSeesZero proves a current month with no
// invoice reports 0 cents, even while an older invoice exists (AC-17).
func TestDashboardReportMonthWithoutInvoicesSeesZero(t *testing.T) {
	e := newReportEnv(t)
	vehicle := e.seedVehicle(t)

	order := e.seedOrder(t, vehicle, "AU-REP010", "confirmed")
	old := time.Now().AddDate(0, 0, -40)
	e.seedInvoice(t, order, "RE-0100", 42000, old)

	got := e.getDashboard(t)
	if got.MonthRevenueCents != 0 {
		t.Errorf("month_revenue_cents: got %d, want 0", got.MonthRevenueCents)
	}
	if got.OpenOrders != 1 {
		t.Errorf("open_orders: got %d, want 1", got.OpenOrders)
	}
	if got.FinishedToday != 0 {
		t.Errorf("finished_today: got %d, want 0", got.FinishedToday)
	}
}

// TestDashboardReportEmptyDatabaseSeesZeros proves an empty database answers
// three zeros rather than NULL or an error.
func TestDashboardReportEmptyDatabaseSeesZeros(t *testing.T) {
	e := newReportEnv(t)

	got := e.getDashboard(t)
	if got.OpenOrders != 0 || got.FinishedToday != 0 || got.MonthRevenueCents != 0 {
		t.Errorf("empty database: got %+v, want all zeros", got)
	}
}
