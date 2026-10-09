// Package store owns the PostgreSQL connection pool and schema application.
package store

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
)

// NewPool opens a pool against databaseURL and verifies it is reachable.
func NewPool(ctx context.Context, databaseURL string) (*pgxpool.Pool, error) {
	cfg, err := pgxpool.ParseConfig(databaseURL)
	if err != nil {
		return nil, fmt.Errorf("store: invalid DATABASE_URL: %w", err)
	}
	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		return nil, fmt.Errorf("store: cannot create pool: %w", err)
	}
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("store: cannot reach DATABASE_URL: %w", err)
	}
	return pool, nil
}

// ApplyMigrations runs the schema SQL. The script is written idempotently
// (CREATE TABLE IF NOT EXISTS ...), so applying it on every startup is safe.
func ApplyMigrations(ctx context.Context, pool *pgxpool.Pool, script string) error {
	if _, err := pool.Exec(ctx, script); err != nil {
		return fmt.Errorf("store: applying migrations: %w", err)
	}
	return nil
}
