// Package store owns the PostgreSQL data access for the API. Every query is
// parameterized: no request value is ever concatenated into SQL text (AC-26).
package store

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// OrderItem is a persisted position of an order. Hours belongs to a labor
// position, Quantity and UnitPriceCents to a part position; the fields that do
// not apply to a kind are stored as SQL NULL and read back as nil.
type OrderItem struct {
	ID             string
	Kind           string
	Description    string
	Hours          *float64
	Quantity       *int
	UnitPriceCents *int
}

// NewOrderItem is the input of CreateOrderItem.
type NewOrderItem struct {
	Kind           string
	Description    string
	Hours          *float64
	Quantity       *int
	UnitPriceCents *int
}

// CreateOrderItem inserts a position for the order with the given order number
// and returns it with its generated id. INSERT ... SELECT lets a missing order
// insert nothing and be reported as ErrOrderNotFound rather than a foreign-key
// error. Every value is bound as a query parameter.
func CreateOrderItem(ctx context.Context, db *pgxpool.Pool, orderNumber string, in NewOrderItem) (OrderItem, error) {
	const query = `
		INSERT INTO order_items (order_id, kind, description, hours, quantity, unit_price_cents)
		SELECT o.id, $1, $2, $3, $4, $5
		FROM orders o
		WHERE o.order_number = $6
		RETURNING id::text, kind, description, hours::float8, quantity, unit_price_cents`

	var item OrderItem
	err := db.QueryRow(ctx, query,
		in.Kind, in.Description, in.Hours, in.Quantity, in.UnitPriceCents, orderNumber,
	).Scan(&item.ID, &item.Kind, &item.Description, &item.Hours, &item.Quantity, &item.UnitPriceCents)
	if errors.Is(err, pgx.ErrNoRows) {
		return OrderItem{}, ErrOrderNotFound
	}
	if err != nil {
		return OrderItem{}, fmt.Errorf("store: create order item: %w", err)
	}
	return item, nil
}
