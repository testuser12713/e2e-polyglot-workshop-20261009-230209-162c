package store

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5"
)

// StatusChange is one recorded transition of an order: the status that was
// reached and when. It is the JSON shape the public and workshop APIs expose as
// history=[{status,changed_at}].
type StatusChange struct {
	Status    string    `json:"status"`
	ChangedAt time.Time `json:"changed_at"`
}

// StatusHistoryQuerier is the read surface of pgx used by the status history.
// Both *pgxpool.Pool and pgx.Tx satisfy it, so callers can read the history
// inside an open transaction or straight from the pool.
type StatusHistoryQuerier interface {
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
}

// InsertStatusHistory records one accepted transition for the order and returns
// the row as it was written, including the database-assigned timestamp.
func InsertStatusHistory(ctx context.Context, tx pgx.Tx, orderID, status string) (StatusChange, error) {
	var change StatusChange
	err := tx.QueryRow(ctx,
		`INSERT INTO order_history (order_id, status) VALUES ($1, $2)
		 RETURNING status, changed_at`,
		orderID, status,
	).Scan(&change.Status, &change.ChangedAt)
	return change, err
}

// ListStatusHistory returns every recorded transition of the order, oldest
// first. The tie-breaker on id keeps the order deterministic for rows written in
// the same clock tick.
func ListStatusHistory(ctx context.Context, q StatusHistoryQuerier, orderID string) ([]StatusChange, error) {
	rows, err := q.Query(ctx,
		`SELECT status, changed_at FROM order_history WHERE order_id = $1
		 ORDER BY changed_at, id`,
		orderID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	changes := make([]StatusChange, 0)
	for rows.Next() {
		var change StatusChange
		if err := rows.Scan(&change.Status, &change.ChangedAt); err != nil {
			return nil, err
		}
		changes = append(changes, change)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return changes, nil
}
