package handlers_test

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"workshop/internal/config"
	"workshop/internal/deps"
	"workshop/internal/server"
	"workshop/internal/store"
)

// customerBody mirrors the contracted customer shape.
type customerBody struct {
	ID    string `json:"id"`
	Name  string `json:"name"`
	Email string `json:"email"`
	Phone string `json:"phone"`
}

// apiErrorBody mirrors the stable error envelope.
type apiErrorBody struct {
	Error struct {
		Code    string `json:"code"`
		Message string `json:"message"`
	} `json:"error"`
}

// newAuthTestServer starts the real routing table against the real PostgreSQL,
// applies the schema and provisions one authenticated session. It skips when
// DATABASE_URL is unavailable in this shell; the office runs it with the DB.
func newAuthTestServer(t *testing.T) (*httptest.Server, string) {
	t.Helper()

	databaseURL := os.Getenv("DATABASE_URL")
	if databaseURL == "" {
		t.Skip("DATABASE_URL must be set to exercise the customers/vehicles routes")
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

	suffix := fmt.Sprintf("%d", time.Now().UnixNano())
	token := "test-session-" + suffix
	sum := sha256.Sum256([]byte(token))

	var employeeID string
	if err := pool.QueryRow(ctx,
		`INSERT INTO employees (email, password_hash, name) VALUES ($1, $2, $3) RETURNING id::text`,
		"tester-"+suffix+"@example.com", "not-a-real-hash", "Test Employee",
	).Scan(&employeeID); err != nil {
		t.Fatalf("seed employee: %v", err)
	}
	if _, err := pool.Exec(ctx,
		`INSERT INTO sessions (token_hash, employee_id, expires_at) VALUES ($1, $2, now() + interval '1 hour')`,
		hex.EncodeToString(sum[:]), employeeID,
	); err != nil {
		t.Fatalf("seed session: %v", err)
	}

	d := &deps.Deps{
		DB:  pool,
		Cfg: &config.Config{WebOrigin: config.DefaultWebOrigin, APIPort: "0"},
	}
	deps.Set(d)

	srv := httptest.NewServer(server.New(d))
	t.Cleanup(srv.Close)
	return srv, token
}

// doJSON sends a JSON request with the test bearer token and decodes the JSON
// response into out when out is non-nil. It fails the test on an unexpected
// status so a broken route is caught at once.
func doJSON(t *testing.T, method, url, token string, body, out any, wantStatus int) {
	t.Helper()

	var reader io.Reader
	if body != nil {
		raw, err := json.Marshal(body)
		if err != nil {
			t.Fatalf("encode request body: %v", err)
		}
		reader = bytes.NewReader(raw)
	}
	req, err := http.NewRequest(method, url, reader)
	if err != nil {
		t.Fatalf("build request: %v", err)
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("%s %s: %v", method, url, err)
	}
	defer resp.Body.Close()

	raw, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != wantStatus {
		t.Fatalf("%s %s: got %d, want %d; body=%s", method, url, resp.StatusCode, wantStatus, raw)
	}
	if out != nil {
		if err := json.Unmarshal(raw, out); err != nil {
			t.Fatalf("decode response from %s %s: %v; body=%s", method, url, err, raw)
		}
	}
}

// TestCustomerCreateAndReadRoundTrip covers AC-01: a customer is created with
// name, email and phone and read back with all three values.
func TestCustomerCreateAndReadRoundTrip(t *testing.T) {
	srv, token := newAuthTestServer(t)

	var created customerBody
	doJSON(t, http.MethodPost, srv.URL+"/api/customers", token,
		map[string]string{
			"name":  "Anna Beispiel",
			"email": "anna@example.com",
			"phone": "+49 30 123456",
		}, &created, http.StatusCreated)

	if created.ID == "" {
		t.Fatalf("created customer has no id: %+v", created)
	}
	if created.Name != "Anna Beispiel" || created.Email != "anna@example.com" || created.Phone != "+49 30 123456" {
		t.Fatalf("created customer fields wrong: %+v", created)
	}

	var fetched customerBody
	doJSON(t, http.MethodGet, srv.URL+"/api/customers/"+created.ID, token, nil, &fetched, http.StatusOK)
	if fetched != created {
		t.Fatalf("round trip mismatch: got %+v, want %+v", fetched, created)
	}
}

// TestGetUnknownCustomerReturns404 proves an unknown id answers the stable
// not_found envelope instead of crashing.
func TestGetUnknownCustomerReturns404(t *testing.T) {
	srv, token := newAuthTestServer(t)

	var body apiErrorBody
	doJSON(t, http.MethodGet, srv.URL+"/api/customers/00000000-0000-0000-0000-000000000000",
		token, nil, &body, http.StatusNotFound)
	if body.Error.Code != "not_found" || body.Error.Message == "" {
		t.Fatalf("unexpected error envelope: %+v", body.Error)
	}
}

// TestCustomerCreateRejectsMissingFields proves validation on an incomplete
// body answers 400 bad_request.
func TestCustomerCreateRejectsMissingFields(t *testing.T) {
	srv, token := newAuthTestServer(t)

	var body apiErrorBody
	doJSON(t, http.MethodPost, srv.URL+"/api/customers", token,
		map[string]string{"name": "Nur Name"}, &body, http.StatusBadRequest)
	if body.Error.Code != "bad_request" {
		t.Fatalf("unexpected error code: %q", body.Error.Code)
	}
}

// TestCustomerInputInjectionDoesNotChangeQueryStructure covers AC-26: an SQL
// injection payload is stored and returned literally, and the customers table
// is still intact afterwards.
func TestCustomerInputInjectionDoesNotChangeQueryStructure(t *testing.T) {
	srv, token := newAuthTestServer(t)
	const payload = "'; DROP TABLE customers; --"

	var created customerBody
	doJSON(t, http.MethodPost, srv.URL+"/api/customers", token,
		map[string]string{
			"name":  payload,
			"email": "injection@example.com",
			"phone": "000",
		}, &created, http.StatusCreated)
	if created.Name != payload {
		t.Fatalf("payload was altered: got %q, want %q", created.Name, payload)
	}

	// The table must still exist and the row be readable with the payload intact.
	var fetched customerBody
	doJSON(t, http.MethodGet, srv.URL+"/api/customers/"+created.ID, token, nil, &fetched, http.StatusOK)
	if fetched.Name != payload {
		t.Fatalf("read back payload altered: got %q, want %q", fetched.Name, payload)
	}
}
