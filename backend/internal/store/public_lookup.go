package store

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// PublicOrder is the privacy-safe view of an order for the customer lookup
// (GET /api/public/orders/{nr}). It carries no personal data: no customer name,
// e-mail or phone. Its shape matches the public contract exactly
// ({order_number,status,desired_date,description,plate,history}).
type PublicOrder struct {
	OrderNumber string              `json:"order_number"`
	Status      string              `json:"status"`
	DesiredDate time.Time           `json:"desired_date"`
	Description string              `json:"description"`
	Plate       string              `json:"plate"`
	History     []OrderHistoryEntry `json:"history"`
}

// GetPublicOrder assembles the public view of the order matching BOTH the order
// number and the plate. The pair is checked in a single query so a wrong plate
// can never leak the details (or existence) of someone else's order: a
// non-matching pair returns ErrOrderNotFound. The history comes from
// order_history, oldest first.
func GetPublicOrder(ctx context.Context, db *pgxpool.Pool, orderNumber, plate string) (*PublicOrder, error) {
	const query = `
		SELECT o.id::text, o.order_number, o.status, o.desired_date, o.description, v.plate
		FROM orders o
		JOIN vehicles v ON v.id = o.vehicle_id
		WHERE o.order_number = $1 AND upper(v.plate) = upper($2)`

	var (
		orderID string
		po      PublicOrder
	)
	err := db.QueryRow(ctx, query, orderNumber, plate).Scan(
		&orderID, &po.OrderNumber, &po.Status, &po.DesiredDate, &po.Description, &po.Plate,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrOrderNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("store: get public order: %w", err)
	}

	history, err := orderHistory(ctx, db, orderID)
	if err != nil {
		return nil, err
	}
	po.History = history
	return &po, nil
}

// GetPublicInvoice reads the invoice of the order matching BOTH the order number
// and the plate. The pair is resolved in one query, so a wrong plate returns
// ErrOrderNotFound instead of the invoice of another order; when the matching
// order simply has no invoice yet, it returns (nil, nil) and the caller answers
// 200 with a null invoice.
func GetPublicInvoice(ctx context.Context, db *pgxpool.Pool, orderNumber, plate string) (*OrderInvoice, error) {
	const query = `
		SELECT o.id::text
		FROM orders o
		JOIN vehicles v ON v.id = o.vehicle_id
		WHERE o.order_number = $1 AND upper(v.plate) = upper($2)`

	var orderID string
	err := db.QueryRow(ctx, query, orderNumber, plate).Scan(&orderID)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrOrderNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("store: resolve public invoice order: %w", err)
	}

	return orderInvoice(ctx, db, orderID)
}
