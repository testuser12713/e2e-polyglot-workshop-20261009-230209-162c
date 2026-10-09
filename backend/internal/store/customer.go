package store

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Customer is the contracted customer shape returned by the API.
type Customer struct {
	ID    string `json:"id"`
	Name  string `json:"name"`
	Email string `json:"email"`
	Phone string `json:"phone"`
}

// ErrCustomerNotFound is returned when no customer matches the requested id or
// the id is not a syntactically valid uuid.
var ErrCustomerNotFound = errors.New("store: customer not found")

// CreateCustomer inserts a customer and returns it with its generated id.
// Every value is bound as a query parameter, never concatenated into the SQL.
func CreateCustomer(ctx context.Context, db *pgxpool.Pool, name, email, phone string) (Customer, error) {
	var c Customer
	err := db.QueryRow(ctx,
		`INSERT INTO customers (name, email, phone)
		 VALUES ($1, $2, $3)
		 RETURNING id::text, name, email, phone`,
		name, email, phone,
	).Scan(&c.ID, &c.Name, &c.Email, &c.Phone)
	if err != nil {
		return Customer{}, fmt.Errorf("store: create customer: %w", err)
	}
	return c, nil
}

// GetCustomer loads one customer by id. A syntactically invalid id is treated
// like a missing customer so the handler answers 404 instead of 500.
func GetCustomer(ctx context.Context, db *pgxpool.Pool, id string) (Customer, error) {
	var c Customer
	err := db.QueryRow(ctx,
		`SELECT id::text, name, email, phone FROM customers WHERE id = $1`,
		id,
	).Scan(&c.ID, &c.Name, &c.Email, &c.Phone)
	switch {
	case errors.Is(err, pgx.ErrNoRows):
		return Customer{}, ErrCustomerNotFound
	case err != nil:
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "22P02" {
			// invalid_text_representation: the path id is not a uuid.
			return Customer{}, ErrCustomerNotFound
		}
		return Customer{}, fmt.Errorf("store: get customer: %w", err)
	}
	return c, nil
}
