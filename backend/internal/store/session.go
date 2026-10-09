package store

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// HashToken returns the hex-encoded sha256 of a session token. Only this hash is
// ever stored or compared; the raw token exists only in the client's hand.
func HashToken(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}

// CreateSession stores one session for employeeID. Only tokenHash is persisted
// (the raw token never reaches the database).
func CreateSession(ctx context.Context, pool *pgxpool.Pool, employeeID, tokenHash string, expiresAt time.Time) error {
	if _, err := pool.Exec(ctx,
		`INSERT INTO sessions (token_hash, employee_id, expires_at)
		 VALUES ($1, $2::uuid, $3)`,
		tokenHash, employeeID, expiresAt,
	); err != nil {
		return fmt.Errorf("store: create session: %w", err)
	}
	return nil
}

// DeleteSession removes the session identified by tokenHash. Removing an already
// absent session is not an error.
func DeleteSession(ctx context.Context, pool *pgxpool.Pool, tokenHash string) error {
	if _, err := pool.Exec(ctx,
		`DELETE FROM sessions WHERE token_hash = $1`,
		tokenHash,
	); err != nil {
		return fmt.Errorf("store: delete session: %w", err)
	}
	return nil
}
