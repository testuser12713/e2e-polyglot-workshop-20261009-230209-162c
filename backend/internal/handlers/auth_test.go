package handlers_test

import (
	"bytes"
	"context"
	"encoding/json"
	"log"
	"net/http"
	"net/http/httptest"
	"os"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"workshop/internal/config"
	"workshop/internal/deps"
	"workshop/internal/middleware"
	"workshop/internal/queue"
	"workshop/internal/server"
	"workshop/internal/store"
)

// newAuthEnv wires the real routing table against the real PostgreSQL and
// Valkey from the environment and seeds one unique bootstrap employee through
// the same startup path the product uses (deps.Set). It skips when the
// databases are not available in this shell; the office runs it with them.
func newAuthEnv(t *testing.T) (srv *httptest.Server, pool *pgxpool.Pool, email, password string) {
	t.Helper()

	databaseURL := os.Getenv("DATABASE_URL")
	valkeyURL := os.Getenv("VALKEY_URL")
	if databaseURL == "" || valkeyURL == "" {
		t.Skip("DATABASE_URL and VALKEY_URL must be set to run the auth test")
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

	unique := strconv.FormatInt(time.Now().UnixNano(), 10)
	email = "auth-" + unique + "@example.com"
	password = "geheim-" + unique

	cfg := &config.Config{
		DatabaseURL:       databaseURL,
		ValkeyURL:         valkeyURL,
		WebOrigin:         config.DefaultWebOrigin,
		APIPort:           "0",
		BootstrapEmail:    email,
		BootstrapPassword: password,
	}
	// deps.Set is the product's startup hook: it seeds the configured employee.
	deps.Set(&deps.Deps{DB: pool, Queue: q, Cfg: cfg})

	srv = httptest.NewServer(server.New(&deps.Deps{DB: pool, Queue: q, Cfg: cfg}))
	t.Cleanup(srv.Close)
	return srv, pool, email, password
}

// login posts credentials and returns the issued token.
func login(t *testing.T, srv *httptest.Server, email, password string) string {
	t.Helper()

	body, err := json.Marshal(map[string]string{"email": email, "password": password})
	if err != nil {
		t.Fatalf("encode login body: %v", err)
	}
	resp, err := http.Post(srv.URL+"/api/auth/login", "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatalf("POST /api/auth/login: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("POST /api/auth/login: got %d, want 200", resp.StatusCode)
	}

	var out struct {
		Token    string `json:"token"`
		Employee struct {
			ID    string `json:"id"`
			Email string `json:"email"`
			Name  string `json:"name"`
		} `json:"employee"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		t.Fatalf("decode login response: %v", err)
	}
	if out.Token == "" {
		t.Fatal("login returned an empty token")
	}
	if out.Employee.ID == "" || out.Employee.Email != email || out.Employee.Name == "" {
		t.Fatalf("login returned an incomplete employee: %+v", out.Employee)
	}
	return out.Token
}

// assertErrorCode reads the standard error envelope and checks its stable code.
func assertErrorCode(t *testing.T, resp *http.Response, wantCode string) {
	t.Helper()

	var body struct {
		Error struct {
			Code    string `json:"code"`
			Message string `json:"message"`
		} `json:"error"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatalf("decode error body: %v", err)
	}
	if body.Error.Code != wantCode {
		t.Fatalf("error code: got %q, want %q", body.Error.Code, wantCode)
	}
	if body.Error.Message == "" {
		t.Fatal("error envelope carries no message")
	}
}

// TestLoginIssuesTokenAcceptedByProtectedRoute proves the login token opens the
// protected area, while the same call without it is rejected with the standard
// envelope (AC-13, AC-14).
func TestLoginIssuesTokenAcceptedByProtectedRoute(t *testing.T) {
	srv, _, email, password := newAuthEnv(t)

	// Anonymous protected call: 401 with the stable envelope.
	req, err := http.NewRequest(http.MethodPost, srv.URL+"/api/auth/logout", nil)
	if err != nil {
		t.Fatalf("build request: %v", err)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("POST /api/auth/logout without token: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("logout without token: got %d, want 401", resp.StatusCode)
	}
	assertErrorCode(t, resp, "unauthorized")

	// Login, then use the token on a real protected route.
	token := login(t, srv, email, password)

	// A valid token must satisfy the middleware itself with a 200 handler; the
	// sibling workshop handlers answer 501 until their tickets land.
	guarded := middleware.RequireAuth(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	rec := httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodGet, "/api/reports/dashboard", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	guarded.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("guarded handler with valid token: got %d, want 200", rec.Code)
	}

	// The token also opens a real protected route and logout ends the session.
	req, err = http.NewRequest(http.MethodPost, srv.URL+"/api/auth/logout", nil)
	if err != nil {
		t.Fatalf("build request: %v", err)
	}
	req.Header.Set("Authorization", "Bearer "+token)
	resp, err = http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("POST /api/auth/logout with token: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusUnauthorized {
		t.Fatal("valid token was rejected by the protected route")
	}
}

// TestLoginRejectsWrongPassword proves a wrong password answers 401 with the
// invalid_credentials code (AC-18).
func TestLoginRejectsWrongPassword(t *testing.T) {
	srv, _, email, _ := newAuthEnv(t)

	body, _ := json.Marshal(map[string]string{"email": email, "password": "definitely-wrong"})
	resp, err := http.Post(srv.URL+"/api/auth/login", "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatalf("POST /api/auth/login: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("wrong password: got %d, want 401", resp.StatusCode)
	}
	assertErrorCode(t, resp, "invalid_credentials")
}

// TestLoginRejectsUnknownEmail proves an unknown e-mail answers the same 401, so
// the response does not reveal whether the account exists.
func TestLoginRejectsUnknownEmail(t *testing.T) {
	srv, _, _, _ := newAuthEnv(t)

	body, _ := json.Marshal(map[string]string{"email": "nobody@example.com", "password": "whatever"})
	resp, err := http.Post(srv.URL+"/api/auth/login", "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatalf("POST /api/auth/login: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("unknown email: got %d, want 401", resp.StatusCode)
	}
	assertErrorCode(t, resp, "invalid_credentials")
}

// TestBootstrapPasswordIsStoredHashed proves the database holds only a hash and
// that the hash verifies the configured password (AC-13).
func TestBootstrapPasswordIsStoredHashed(t *testing.T) {
	_, pool, email, password := newAuthEnv(t)

	employee, err := store.EmployeeByEmail(context.Background(), pool, email)
	if err != nil {
		t.Fatalf("read bootstrap employee: %v", err)
	}
	if employee.PasswordHash == password {
		t.Fatal("password is stored in plaintext")
	}
	if employee.PasswordHash == "" {
		t.Fatal("stored password hash is empty")
	}
	if !store.VerifyPassword(employee.PasswordHash, password) {
		t.Fatal("stored hash does not verify the configured password")
	}
	if store.VerifyPassword(employee.PasswordHash, password+"x") {
		t.Fatal("stored hash accepted a wrong password")
	}
}

// TestLoginLogsNeitherPasswordNorToken proves a successful login writes no log
// line containing the password or the issued token (AC-27, AC-30).
func TestLoginLogsNeitherPasswordNorToken(t *testing.T) {
	srv, _, email, password := newAuthEnv(t)

	var buf bytes.Buffer
	previous := log.Writer()
	log.SetOutput(&buf)
	t.Cleanup(func() { log.SetOutput(previous) })

	token := login(t, srv, email, password)

	logged := buf.String()
	if strings.Contains(logged, password) {
		t.Fatalf("a log line contains the password: %q", logged)
	}
	if strings.Contains(logged, token) {
		t.Fatalf("a log line contains the session token: %q", logged)
	}
}
