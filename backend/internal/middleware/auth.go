// Package middleware holds the API's request middleware: session protection and
// per-client rate limiting.
package middleware

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"

	"workshop/internal/deps"
	"workshop/internal/httpx"
)

type contextKey string

const employeeIDKey contextKey = "employee_id"

// EmployeeID returns the authenticated employee id stored in ctx, or "".
func EmployeeID(ctx context.Context) string {
	id, _ := ctx.Value(employeeIDKey).(string)
	return id
}

// RequireAuth rejects a request without a valid, unexpired session. The token is
// only ever compared as its sha256 hash; the raw token is never stored or
// logged.
func RequireAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		const prefix = "Bearer "
		header := r.Header.Get("Authorization")
		if !strings.HasPrefix(header, prefix) {
			writeUnauthorized(w)
			return
		}
		token := strings.TrimSpace(header[len(prefix):])
		if token == "" {
			writeUnauthorized(w)
			return
		}

		sum := sha256.Sum256([]byte(token))
		tokenHash := hex.EncodeToString(sum[:])

		var (
			employeeID string
			expiresAt  time.Time
		)
		err := deps.DB().QueryRow(r.Context(),
			`SELECT employee_id::text, expires_at FROM sessions WHERE token_hash = $1`,
			tokenHash,
		).Scan(&employeeID, &expiresAt)
		switch {
		case errors.Is(err, pgx.ErrNoRows):
			writeUnauthorized(w)
			return
		case err != nil:
			httpx.WriteError(w, http.StatusInternalServerError, httpx.CodeInternal, "Interner Fehler bei der Anmeldeprüfung.")
			return
		}
		if time.Now().After(expiresAt) {
			writeUnauthorized(w)
			return
		}

		ctx := context.WithValue(r.Context(), employeeIDKey, employeeID)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func writeUnauthorized(w http.ResponseWriter) {
	httpx.WriteError(w, http.StatusUnauthorized, httpx.CodeUnauthorized, "Bitte melden Sie sich an.")
}
