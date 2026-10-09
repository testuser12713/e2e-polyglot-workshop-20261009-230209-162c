package store

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// ErrOrderNotFound is returned by GetOrder when no order carries the requested
// order number.
var ErrOrderNotFound = errors.New("store: order not found")

// OrderListItem is one row of the workshop order list
// (GET /api/orders). It carries exactly the fields the list contracts.
type OrderListItem struct {
	OrderNumber  string    `json:"order_number"`
	Status       string    `json:"status"`
	DesiredDate  time.Time `json:"desired_date"`
	Plate        string    `json:"plate"`
	CustomerName string    `json:"customer_name"`
}

// OrderCustomer is the customer block of an assembled order detail.
type OrderCustomer struct {
	ID    string `json:"id"`
	Name  string `json:"name"`
	Email string `json:"email"`
	Phone string `json:"phone"`
}

// OrderVehicle is the vehicle block of an assembled order detail.
type OrderVehicle struct {
	ID         string `json:"id"`
	CustomerID string `json:"customer_id"`
	Plate      string `json:"plate"`
	Brand      string `json:"brand"`
	Model      string `json:"model"`
	MileageKM  int    `json:"mileage_km"`
}

// OrderLine is one captured position of an order: a labor entry carries hours,
// a part carries quantity and unit price; the unused columns stay null.
type OrderLine struct {
	ID             string   `json:"id"`
	Kind           string   `json:"kind"`
	Description    string   `json:"description"`
	Hours          *float64 `json:"hours"`
	Quantity       *int     `json:"quantity"`
	UnitPriceCents *int     `json:"unit_price_cents"`
}

// OrderHistoryEntry is one status change of an order.
type OrderHistoryEntry struct {
	Status    string    `json:"status"`
	ChangedAt time.Time `json:"changed_at"`
}

// OrderInvoiceItem is one line of an issued invoice.
type OrderInvoiceItem struct {
	Description    string  `json:"description"`
	Quantity       float64 `json:"quantity"`
	UnitPriceCents int     `json:"unit_price_cents"`
}

// OrderInvoice is the invoice of an order, or nil while the worker has not
// issued one yet.
type OrderInvoice struct {
	InvoiceNumber string             `json:"invoice_number"`
	IssuedAt      time.Time          `json:"issued_at"`
	NetCents      int                `json:"net_cents"`
	VatCents      int                `json:"vat_cents"`
	GrossCents    int                `json:"gross_cents"`
	Items         []OrderInvoiceItem `json:"items"`
}

// OrderDetail is the fully assembled order returned by GET /api/orders/{nr}.
type OrderDetail struct {
	OrderNumber string              `json:"order_number"`
	Status      string              `json:"status"`
	DesiredDate time.Time           `json:"desired_date"`
	Description string              `json:"description"`
	Customer    OrderCustomer       `json:"customer"`
	Vehicle     OrderVehicle        `json:"vehicle"`
	Items       []OrderLine         `json:"items"`
	History     []OrderHistoryEntry `json:"history"`
	Invoice     *OrderInvoice       `json:"invoice"`
}

// ListOrders reads the workshop order list, optionally narrowed by an exact
// status and by a plate substring. Every input value is bound as a query
// parameter; an empty filter matches everything.
func ListOrders(ctx context.Context, db *pgxpool.Pool, status, plate string) ([]OrderListItem, error) {
	const query = `
		SELECT o.order_number, o.status, o.desired_date, v.plate, c.name
		FROM orders o
		JOIN vehicles v ON v.id = o.vehicle_id
		JOIN customers c ON c.id = o.customer_id
		WHERE ($1 = '' OR o.status = $1)
		  AND ($2 = '' OR v.plate ILIKE '%' || $2 || '%' ESCAPE '\')
		ORDER BY o.created_at DESC, o.order_number`

	rows, err := db.Query(ctx, query, status, escapeLike(plate))
	if err != nil {
		return nil, fmt.Errorf("store: list orders: %w", err)
	}
	defer rows.Close()

	items := make([]OrderListItem, 0)
	for rows.Next() {
		var it OrderListItem
		if err := rows.Scan(&it.OrderNumber, &it.Status, &it.DesiredDate, &it.Plate, &it.CustomerName); err != nil {
			return nil, fmt.Errorf("store: scan order list row: %w", err)
		}
		items = append(items, it)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("store: list orders: %w", err)
	}
	return items, nil
}

// GetOrder assembles one order with its customer, vehicle, positions, status
// history and (possibly absent) invoice. It returns ErrOrderNotFound for an
// unknown order number.
func GetOrder(ctx context.Context, db *pgxpool.Pool, orderNumber string) (*OrderDetail, error) {
	const orderQuery = `
		SELECT o.id::text, o.order_number, o.status, o.desired_date, o.description,
		       c.id::text, c.name, c.email, c.phone,
		       v.id::text, v.customer_id::text, v.plate, v.brand, v.model, v.mileage_km
		FROM orders o
		JOIN customers c ON c.id = o.customer_id
		JOIN vehicles v ON v.id = o.vehicle_id
		WHERE o.order_number = $1`

	var (
		orderID string
		d       OrderDetail
	)
	err := db.QueryRow(ctx, orderQuery, orderNumber).Scan(
		&orderID, &d.OrderNumber, &d.Status, &d.DesiredDate, &d.Description,
		&d.Customer.ID, &d.Customer.Name, &d.Customer.Email, &d.Customer.Phone,
		&d.Vehicle.ID, &d.Vehicle.CustomerID, &d.Vehicle.Plate, &d.Vehicle.Brand,
		&d.Vehicle.Model, &d.Vehicle.MileageKM,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrOrderNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("store: get order: %w", err)
	}

	if d.Items, err = orderLines(ctx, db, orderID); err != nil {
		return nil, err
	}
	if d.History, err = orderHistory(ctx, db, orderID); err != nil {
		return nil, err
	}
	if d.Invoice, err = orderInvoice(ctx, db, orderID); err != nil {
		return nil, err
	}
	return &d, nil
}

// orderLines reads the positions of an order, oldest first.
func orderLines(ctx context.Context, db *pgxpool.Pool, orderID string) ([]OrderLine, error) {
	const query = `
		SELECT id::text, kind, description, hours, quantity, unit_price_cents
		FROM order_items
		WHERE order_id = $1
		ORDER BY created_at, id`

	rows, err := db.Query(ctx, query, orderID)
	if err != nil {
		return nil, fmt.Errorf("store: list order items: %w", err)
	}
	defer rows.Close()

	lines := make([]OrderLine, 0)
	for rows.Next() {
		var l OrderLine
		if err := rows.Scan(&l.ID, &l.Kind, &l.Description, &l.Hours, &l.Quantity, &l.UnitPriceCents); err != nil {
			return nil, fmt.Errorf("store: scan order item: %w", err)
		}
		lines = append(lines, l)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("store: list order items: %w", err)
	}
	return lines, nil
}

// orderHistory reads the status changes of an order, oldest first.
func orderHistory(ctx context.Context, db *pgxpool.Pool, orderID string) ([]OrderHistoryEntry, error) {
	const query = `
		SELECT status, changed_at
		FROM order_history
		WHERE order_id = $1
		ORDER BY changed_at, id`

	rows, err := db.Query(ctx, query, orderID)
	if err != nil {
		return nil, fmt.Errorf("store: list order history: %w", err)
	}
	defer rows.Close()

	history := make([]OrderHistoryEntry, 0)
	for rows.Next() {
		var h OrderHistoryEntry
		if err := rows.Scan(&h.Status, &h.ChangedAt); err != nil {
			return nil, fmt.Errorf("store: scan order history: %w", err)
		}
		history = append(history, h)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("store: list order history: %w", err)
	}
	return history, nil
}

// orderInvoice reads the invoice of an order and its lines, or nil when the
// worker has not issued one yet.
func orderInvoice(ctx context.Context, db *pgxpool.Pool, orderID string) (*OrderInvoice, error) {
	const invoiceQuery = `
		SELECT id::text, invoice_number, issued_at, net_cents, vat_cents, gross_cents
		FROM invoices
		WHERE order_id = $1`

	var (
		invoiceID string
		inv       OrderInvoice
	)
	err := db.QueryRow(ctx, invoiceQuery, orderID).Scan(
		&invoiceID, &inv.InvoiceNumber, &inv.IssuedAt, &inv.NetCents, &inv.VatCents, &inv.GrossCents,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("store: get order invoice: %w", err)
	}

	const itemsQuery = `
		SELECT description, quantity, unit_price_cents
		FROM invoice_items
		WHERE invoice_id = $1
		ORDER BY id`

	rows, err := db.Query(ctx, itemsQuery, invoiceID)
	if err != nil {
		return nil, fmt.Errorf("store: list invoice items: %w", err)
	}
	defer rows.Close()

	inv.Items = make([]OrderInvoiceItem, 0)
	for rows.Next() {
		var it OrderInvoiceItem
		if err := rows.Scan(&it.Description, &it.Quantity, &it.UnitPriceCents); err != nil {
			return nil, fmt.Errorf("store: scan invoice item: %w", err)
		}
		inv.Items = append(inv.Items, it)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("store: list invoice items: %w", err)
	}
	return &inv, nil
}

// escapeLike neutralises the LIKE metacharacters in a user-supplied plate
// fragment so they are matched literally (the pattern's own wildcards stay).
func escapeLike(s string) string {
	return strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`).Replace(s)
}
