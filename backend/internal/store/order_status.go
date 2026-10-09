package store

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
)

// OrderStatusForUpdate reads the order id and its current status under a row
// lock. Two concurrent transitions therefore cannot both read the same
// predecessor and apply from it; the second waits and then sees the new status.
func OrderStatusForUpdate(ctx context.Context, tx pgx.Tx, orderNumber string) (orderID, status string, err error) {
	err = tx.QueryRow(ctx,
		`SELECT id::text, status FROM orders WHERE order_number = $1 FOR UPDATE`,
		orderNumber,
	).Scan(&orderID, &status)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", "", ErrOrderNotFound
	}
	if err != nil {
		return "", "", err
	}
	return orderID, status, nil
}

// UpdateOrderStatus writes the new status and touches updated_at. The status
// value is passed as a parameter, never interpolated into the statement.
func UpdateOrderStatus(ctx context.Context, tx pgx.Tx, orderID, status string) error {
	_, err := tx.Exec(ctx,
		`UPDATE orders SET status = $2, updated_at = now() WHERE id = $1`,
		orderID, status)
	return err
}
