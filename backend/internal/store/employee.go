// Package store holds the PostgreSQL access layer. Every query is parameterized
// (AC-26): no input value is ever concatenated into SQL text.
package store

import (
	"context"
	"crypto/pbkdf2"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Password hashing parameters. PBKDF2-HMAC-SHA256 with a per-password random
// salt, so the database only ever holds a salted hash, never the password
// itself (AC-13).
const (
	passwordHashAlgorithm  = "pbkdf2_sha256"
	passwordHashIterations = 210000
	passwordHashKeyLength  = 32
	passwordHashSaltLength = 16
)

// DefaultEmployeeName is shown for the seeded bootstrap employee, which has no
// configured display name.
const DefaultEmployeeName = "Werkstatt"

// Employee is one workshop employee as loaded for authentication. PasswordHash
// is the stored encoded hash; it is never logged or returned to a client.
type Employee struct {
	ID           string
	Email        string
	Name         string
	PasswordHash string
}

// EmployeeByEmail loads the employee with the given e-mail. It answers
// pgx.ErrNoRows when there is no such employee.
func EmployeeByEmail(ctx context.Context, pool *pgxpool.Pool, email string) (*Employee, error) {
	var e Employee
	err := pool.QueryRow(ctx,
		`SELECT id::text, email, name, password_hash
		   FROM employees
		  WHERE lower(email) = lower($1)`,
		strings.TrimSpace(email),
	).Scan(&e.ID, &e.Email, &e.Name, &e.PasswordHash)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, pgx.ErrNoRows
		}
		return nil, fmt.Errorf("store: read employee by email: %w", err)
	}
	return &e, nil
}

// EnsureBootstrapEmployee creates the configured workshop employee with a hashed
// password when it does not exist yet. An empty e-mail or password means no
// bootstrap is configured; the call is then a no-op. An existing employee is
// left untouched.
func EnsureBootstrapEmployee(ctx context.Context, pool *pgxpool.Pool, email, password string) error {
	email = strings.TrimSpace(email)
	if email == "" || password == "" {
		return nil
	}
	hash, err := HashPassword(password)
	if err != nil {
		return fmt.Errorf("store: hash bootstrap password: %w", err)
	}
	if _, err := pool.Exec(ctx,
		`INSERT INTO employees (email, name, password_hash)
		 VALUES ($1, $2, $3)
		 ON CONFLICT (email) DO NOTHING`,
		email, DefaultEmployeeName, hash,
	); err != nil {
		return fmt.Errorf("store: create bootstrap employee: %w", err)
	}
	return nil
}

// HashPassword derives a salted hash for password in the encoded form
// "pbkdf2_sha256$<iterations>$<salt>$<key>".
func HashPassword(password string) (string, error) {
	salt := make([]byte, passwordHashSaltLength)
	if _, err := rand.Read(salt); err != nil {
		return "", err
	}
	key, err := pbkdf2.Key(sha256.New, password, salt, passwordHashIterations, passwordHashKeyLength)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("%s$%d$%s$%s",
		passwordHashAlgorithm,
		passwordHashIterations,
		base64.RawStdEncoding.EncodeToString(salt),
		base64.RawStdEncoding.EncodeToString(key),
	), nil
}

// VerifyPassword reports whether password matches the encoded hash. The
// comparison is constant-time.
func VerifyPassword(encoded, password string) bool {
	parts := strings.Split(encoded, "$")
	if len(parts) != 4 || parts[0] != passwordHashAlgorithm {
		return false
	}
	iterations, err := strconv.Atoi(parts[1])
	if err != nil || iterations <= 0 {
		return false
	}
	salt, err := base64.RawStdEncoding.DecodeString(parts[2])
	if err != nil {
		return false
	}
	want, err := base64.RawStdEncoding.DecodeString(parts[3])
	if err != nil || len(want) == 0 {
		return false
	}
	got, err := pbkdf2.Key(sha256.New, password, salt, iterations, len(want))
	if err != nil {
		return false
	}
	return subtle.ConstantTimeCompare(got, want) == 1
}
