// This file owns the write path of the public appointment request: it creates
// the customer, vehicle and order (plus the initial history entry) in a single
// transaction. Every statement binds its input as a pgx parameter, so no user
// value ever changes the shape of the SQL (AC-26).
package store

import (
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

// ErrPlateTaken reports that a vehicle with the requested licence plate already
// exists. The caller answers 409 plate_taken.
var ErrPlateTaken = errors.New("store: plate already exists")

// errOrderNumberConflict is internal: it triggers one retry with a freshly
// rolled order number.
var errOrderNumberConflict = errors.New("store: order number collision")

// orderNumberAttempts caps how often the creation retries after a (statistically
// very unlikely) order-number collision.
const orderNumberAttempts = 5

// CreateOrderInput is a validated appointment request.
type CreateOrderInput struct {
	CustomerName  string
	CustomerEmail string
	CustomerPhone string
	Plate         string
	Brand         string
	Model         string
	MileageKm     int
	DesiredDate   time.Time
	Description   string
}

// CreateOrderResult identifies the freshly created order.
type CreateOrderResult struct {
	OrderNumber string
	Status      string
}

// orderNumberAlphabet is the contract's character set for the six-char suffix.
const orderNumberAlphabet = "ABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"

// CreateRequestedOrder creates customer, vehicle, order and the initial history
// entry in one transaction and answers with the new order identity. A duplicate
// plate rolls the whole transaction back and reports ErrPlateTaken.
func CreateRequestedOrder(ctx context.Context, pool *pgxpool.Pool, in CreateOrderInput) (CreateOrderResult, error) {
	for attempt := 0; attempt < orderNumberAttempts; attempt++ {
		number, err := newOrderNumber()
		if err != nil {
			return CreateOrderResult{}, err
		}
		result, err := insertRequestedOrder(ctx, pool, number, in)
		switch {
		case err == nil:
			return result, nil
		case errors.Is(err, errOrderNumberConflict):
			continue
		default:
			return CreateOrderResult{}, err
		}
	}
	return CreateOrderResult{}, errors.New("store: could not allocate a unique order number")
}

// insertRequestedOrder performs the single-transaction write for one order
// number.
func insertRequestedOrder(ctx context.Context, pool *pgxpool.Pool, orderNumber string, in CreateOrderInput) (CreateOrderResult, error) {
	tx, err := pool.Begin(ctx)
	if err != nil {
		return CreateOrderResult{}, fmt.Errorf("store: begin: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	var customerID string
	if err := tx.QueryRow(ctx,
		`INSERT INTO customers (name, email, phone) VALUES ($1, $2, $3) RETURNING id::text`,
		in.CustomerName, in.CustomerEmail, in.CustomerPhone,
	).Scan(&customerID); err != nil {
		return CreateOrderResult{}, fmt.Errorf("store: insert customer: %w", err)
	}

	var vehicleID string
	err = tx.QueryRow(ctx,
		`INSERT INTO vehicles (customer_id, plate, brand, model, mileage_km)
		 VALUES ($1, $2, $3, $4, $5) RETURNING id::text`,
		customerID, in.Plate, in.Brand, in.Model, in.MileageKm,
	).Scan(&vehicleID)
	if err != nil {
		if isPlateConflict(err) {
			return CreateOrderResult{}, ErrPlateTaken
		}
		return CreateOrderResult{}, fmt.Errorf("store: insert vehicle: %w", err)
	}

	var orderID string
	err = tx.QueryRow(ctx,
		`INSERT INTO orders (order_number, customer_id, vehicle_id, status, desired_date, description)
		 VALUES ($1, $2, $3, 'requested', $4, $5) RETURNING id::text`,
		orderNumber, customerID, vehicleID, in.DesiredDate, in.Description,
	).Scan(&orderID)
	if err != nil {
		if isUniqueViolation(err) {
			return CreateOrderResult{}, errOrderNumberConflict
		}
		return CreateOrderResult{}, fmt.Errorf("store: insert order: %w", err)
	}

	if _, err := tx.Exec(ctx,
		`INSERT INTO order_history (order_id, status) VALUES ($1, 'requested')`,
		orderID,
	); err != nil {
		return CreateOrderResult{}, fmt.Errorf("store: insert order history: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return CreateOrderResult{}, fmt.Errorf("store: commit: %w", err)
	}
	return CreateOrderResult{OrderNumber: orderNumber, Status: "requested"}, nil
}

// newOrderNumber rolls "AU-" plus six characters from the contract alphabet.
func newOrderNumber() (string, error) {
	buf := make([]byte, 6)
	if _, err := rand.Read(buf); err != nil {
		return "", fmt.Errorf("store: generate order number: %w", err)
	}
	suffix := make([]byte, 6)
	for i, b := range buf {
		suffix[i] = orderNumberAlphabet[int(b)%len(orderNumberAlphabet)]
	}
	return "AU-" + string(suffix), nil
}

// isPlateConflict reports whether err is the unique violation of
// vehicles.plate (constraint vehicles_plate_key).
func isPlateConflict(err error) bool {
	var pgErr *pgconn.PgError
	if !errors.As(err, &pgErr) {
		return false
	}
	return pgErr.Code == "23505" && pgErr.ConstraintName == "vehicles_plate_key"
}

// isUniqueViolation reports any unique-constraint violation.
func isUniqueViolation(err error) bool {
	var pgErr *pgconn.PgError
	if !errors.As(err, &pgErr) {
		return false
	}
	return pgErr.Code == "23505"
}
