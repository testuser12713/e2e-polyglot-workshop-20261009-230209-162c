package handlers_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"workshop/internal/config"
	"workshop/internal/deps"
	"workshop/internal/httpx"
	"workshop/internal/queue"
	"workshop/internal/server"
	"workshop/internal/store"
)

// newTestServer builds the real routing table against the real PostgreSQL and
// Valkey from the environment, applying the schema first. It skips when the
// databases are not available in this shell; the office runs it with them.
func newTestServer(t *testing.T) *httptest.Server {
	t.Helper()

	databaseURL := os.Getenv("DATABASE_URL")
	valkeyURL := os.Getenv("VALKEY_URL")
	if databaseURL == "" || valkeyURL == "" {
		t.Skip("DATABASE_URL and VALKEY_URL must be set to run the skeleton test")
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

	q, err := queue.New(ctx, valkeyURL)
	if err != nil {
		t.Fatalf("connect to Valkey: %v", err)
	}
	t.Cleanup(func() { _ = q.Close() })

	d := &deps.Deps{
		DB:    pool,
		Queue: q,
		Cfg: &config.Config{
			DatabaseURL: databaseURL,
			ValkeyURL:   valkeyURL,
			WebOrigin:   config.DefaultWebOrigin,
			APIPort:     "0",
		},
	}
	deps.Set(d)

	srv := httptest.NewServer(server.New(d))
	t.Cleanup(srv.Close)
	return srv
}

// TestHealthzAnswersOK proves the health endpoint exercises the real database and
// answers 200.
func TestHealthzAnswersOK(t *testing.T) {
	srv := newTestServer(t)

	resp, err := http.Get(srv.URL + "/healthz")
	if err != nil {
		t.Fatalf("GET /healthz: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("GET /healthz: got %d, want 200", resp.StatusCode)
	}
}

// TestContractedRoutesAreRegistered proves every contracted path is wired to a
// handler: it never answers 404. It deliberately does NOT assert a stub's
// temporary 501 — that answer changes the moment the owning ticket merges — only
// that the route exists under the agreed path and verb.
func TestContractedRoutesAreRegistered(t *testing.T) {
	srv := newTestServer(t)

	routes := []struct {
		method string
		path   string
	}{
		{http.MethodPost, "/api/auth/login"},
		{http.MethodPost, "/api/auth/logout"},
		{http.MethodPost, "/api/customers"},
		{http.MethodGet, "/api/customers/00000000-0000-0000-0000-000000000000"},
		{http.MethodPost, "/api/vehicles"},
		{http.MethodGet, "/api/vehicles?plate=XX-XX-123"},
		{http.MethodPost, "/api/orders"},
		{http.MethodGet, "/api/orders?status=&plate="},
		{http.MethodGet, "/api/orders/AU-ABC123"},
		{http.MethodPost, "/api/orders/AU-ABC123/status"},
		{http.MethodPost, "/api/orders/AU-ABC123/items"},
		{http.MethodGet, "/api/public/orders/AU-ABC123?plate=XX-XX-123"},
		{http.MethodGet, "/api/public/orders/AU-ABC123/invoice?plate=XX-XX-123"},
		{http.MethodGet, "/api/reports/dashboard"},
	}

	for _, rt := range routes {
		t.Run(rt.method+" "+rt.path, func(t *testing.T) {
			req, err := http.NewRequest(rt.method, srv.URL+rt.path, nil)
			if err != nil {
				t.Fatalf("build request: %v", err)
			}
			resp, err := http.DefaultClient.Do(req)
			if err != nil {
				t.Fatalf("%s %s: %v", rt.method, rt.path, err)
			}
			defer resp.Body.Close()

			if resp.StatusCode == http.StatusNotFound {
				t.Fatalf("%s %s is not registered (404)", rt.method, rt.path)
			}
		})
	}
}

// TestCORSReflectsConfiguredOriginWithoutWildcard proves the API only emits CORS
// headers for the configured web origin and never a wildcard with credentials
// (AC-24).
func TestCORSReflectsConfiguredOriginWithoutWildcard(t *testing.T) {
	const origin = "http://localhost:5173"
	h := httpx.WithCORS(origin, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	allowed := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	allowed.Header.Set("Origin", origin)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, allowed)
	if got := rec.Header().Get("Access-Control-Allow-Origin"); got != origin {
		t.Errorf("allowed origin: got %q, want %q", got, origin)
	}
	if got := rec.Header().Get("Access-Control-Allow-Credentials"); got != "true" {
		t.Errorf("credentials header: got %q, want true", got)
	}

	other := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	other.Header.Set("Origin", "http://evil.example")
	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, other)
	if got := rec.Header().Get("Access-Control-Allow-Origin"); got != "" {
		t.Errorf("foreign origin must not receive CORS headers, got %q", got)
	}
}

// TestUnknownRouteUsesErrorEnvelope proves an unregistered path answers with the
// stable error shape (AC-18), not the mux's plain-text 404.
func TestUnknownRouteUsesErrorEnvelope(t *testing.T) {
	srv := newTestServer(t)

	resp, err := http.Get(srv.URL + "/does-not-exist")
	if err != nil {
		t.Fatalf("GET /does-not-exist: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("GET /does-not-exist: got %d, want 404", resp.StatusCode)
	}

	var body struct {
		Error struct {
			Code    string `json:"code"`
			Message string `json:"message"`
		} `json:"error"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatalf("decode error body: %v", err)
	}
	if body.Error.Code == "" || body.Error.Message == "" {
		t.Fatalf("error envelope incomplete: %+v", body.Error)
	}
}

// TestProtectedRoutesRejectAnonymous proves the auth middleware guards the whole
// workshop area: a caller without a bearer token never reaches a handler.
func TestProtectedRoutesRejectAnonymous(t *testing.T) {
	srv := newTestServer(t)

	routes := []struct {
		method string
		path   string
	}{
		{http.MethodPost, "/api/auth/logout"},
		{http.MethodPost, "/api/customers"},
		{http.MethodGet, "/api/customers/00000000-0000-0000-0000-000000000000"},
		{http.MethodPost, "/api/vehicles"},
		{http.MethodGet, "/api/vehicles"},
		{http.MethodGet, "/api/orders"},
		{http.MethodGet, "/api/orders/AU-ABC123"},
		{http.MethodPost, "/api/orders/AU-ABC123/status"},
		{http.MethodPost, "/api/orders/AU-ABC123/items"},
		{http.MethodGet, "/api/reports/dashboard"},
	}

	for _, rt := range routes {
		req, err := http.NewRequest(rt.method, srv.URL+rt.path, nil)
		if err != nil {
			t.Fatalf("build request: %v", err)
		}
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatalf("%s %s: %v", rt.method, rt.path, err)
		}
		resp.Body.Close()

		if resp.StatusCode != http.StatusUnauthorized {
			t.Errorf("%s %s without token: got %d, want 401", rt.method, rt.path, resp.StatusCode)
		}
	}
}
