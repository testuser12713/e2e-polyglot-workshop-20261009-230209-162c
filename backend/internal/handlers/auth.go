package handlers

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"

	"workshop/internal/deps"
	"workshop/internal/httpx"
	"workshop/internal/store"
)

// sessionTTL is how long an issued session token stays valid.
const sessionTTL = 12 * time.Hour

type loginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

type employeeView struct {
	ID    string `json:"id"`
	Email string `json:"email"`
	Name  string `json:"name"`
}

type loginResponse struct {
	Token    string       `json:"token"`
	Employee employeeView `json:"employee"`
}

// Login handles POST /api/auth/login. It is owned by the workshop-login ticket.
// Bad credentials always answer the same 401 so a caller cannot tell an unknown
// e-mail from a wrong password; neither the password nor the issued token is
// ever logged.
func Login(w http.ResponseWriter, r *http.Request) {
	pool := deps.DB()
	if pool == nil {
		httpx.WriteError(w, http.StatusServiceUnavailable, httpx.CodeServiceUnavailable,
			"Der Anmeldedienst ist derzeit nicht verfügbar.")
		return
	}

	var req loginRequest
	if err := httpx.DecodeJSON(r, &req); err != nil {
		writeInvalidCredentials(w)
		return
	}
	req.Email = strings.TrimSpace(req.Email)
	if req.Email == "" || req.Password == "" {
		writeInvalidCredentials(w)
		return
	}

	employee, err := store.EmployeeByEmail(r.Context(), pool, req.Email)
	switch {
	case errors.Is(err, pgx.ErrNoRows):
		writeInvalidCredentials(w)
		return
	case err != nil:
		httpx.WriteError(w, http.StatusInternalServerError, httpx.CodeInternal,
			"Interner Fehler bei der Anmeldung.")
		return
	}
	if !store.VerifyPassword(employee.PasswordHash, req.Password) {
		writeInvalidCredentials(w)
		return
	}

	token, err := newSessionToken()
	if err != nil {
		httpx.WriteError(w, http.StatusInternalServerError, httpx.CodeInternal,
			"Interner Fehler bei der Anmeldung.")
		return
	}
	if err := store.CreateSession(r.Context(), pool, employee.ID, store.HashToken(token), time.Now().Add(sessionTTL)); err != nil {
		httpx.WriteError(w, http.StatusInternalServerError, httpx.CodeInternal,
			"Interner Fehler bei der Anmeldung.")
		return
	}

	httpx.WriteJSON(w, http.StatusOK, loginResponse{
		Token: token,
		Employee: employeeView{
			ID:    employee.ID,
			Email: employee.Email,
			Name:  employee.Name,
		},
	})
}

// Logout handles POST /api/auth/logout. It is owned by the workshop-login
// ticket. The route is bearer-protected, so the token is valid; deleting it ends
// the session server-side.
func Logout(w http.ResponseWriter, r *http.Request) {
	pool := deps.DB()
	if pool != nil {
		if token := bearerToken(r); token != "" {
			if err := store.DeleteSession(r.Context(), pool, store.HashToken(token)); err != nil {
				httpx.WriteError(w, http.StatusInternalServerError, httpx.CodeInternal,
					"Die Abmeldung ist fehlgeschlagen.")
				return
			}
		}
	}
	w.WriteHeader(http.StatusNoContent)
}

// newSessionToken returns 32 random bytes as a hex string. crypto/rand is the
// only source, so tokens are not guessable.
func newSessionToken() (string, error) {
	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return hex.EncodeToString(buf), nil
}

// bearerToken extracts the raw token from the Authorization header, or "".
func bearerToken(r *http.Request) string {
	const prefix = "Bearer "
	header := r.Header.Get("Authorization")
	if !strings.HasPrefix(header, prefix) {
		return ""
	}
	return strings.TrimSpace(header[len(prefix):])
}

func writeInvalidCredentials(w http.ResponseWriter) {
	httpx.WriteError(w, http.StatusUnauthorized, httpx.CodeInvalidCredentials,
		"E-Mail oder Passwort ist falsch.")
}
