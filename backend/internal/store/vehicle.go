package store

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Vehicle is the contracted vehicle shape returned by the API.
type Vehicle struct {
	ID         string `json:"id"`
	CustomerID string `json:"customer_id"`
	Plate      string `json:"plate"`
	Brand      string `json:"brand"`
	Model      string `json:"model"`
	MileageKm  int    `json:"mileage_km"`
}

// CreateVehicle inserts a vehicle assigned to a customer. The plate is stored
// upper-cased and trimmed so the unique index and plate lookups agree. On a
// duplicate plate the insert fails and the existing row is left untouched.
func CreateVehicle(ctx context.Context, db *pgxpool.Pool, customerID, plate, brand, model string, mileageKm int) (Vehicle, error) {
	var v Vehicle
	err := db.QueryRow(ctx,
		`INSERT INTO vehicles (customer_id, plate, brand, model, mileage_km)
		 VALUES ($1, $2, $3, $4, $5)
		 RETURNING id::text, customer_id::text, plate, brand, model, mileage_km`,
		customerID, normalizePlate(plate), brand, model, mileageKm,
	).Scan(&v.ID, &v.CustomerID, &v.Plate, &v.Brand, &v.Model, &v.MileageKm)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) {
			switch pgErr.Code {
			case "23505": // unique_violation: vehicles.plate
				return Vehicle{}, ErrPlateTaken
			case "23503": // foreign_key_violation: vehicles.customer_id
				return Vehicle{}, ErrCustomerNotFound
			}
		}
		return Vehicle{}, fmt.Errorf("store: create vehicle: %w", err)
	}
	return v, nil
}

// ListVehicles returns the vehicles matching plate. An empty plate returns every
// vehicle, newest first. Both the plate filter and the limit are bound as query
// parameters; the SQL text is a constant.
func ListVehicles(ctx context.Context, db *pgxpool.Pool, plate string) ([]Vehicle, error) {
	query := `SELECT id::text, customer_id::text, plate, brand, model, mileage_km FROM vehicles`
	args := []any{}
	if p := normalizePlate(plate); p != "" {
		query += ` WHERE plate = $1`
		args = append(args, p)
	}
	query += ` ORDER BY created_at DESC LIMIT 100`

	rows, err := db.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("store: list vehicles: %w", err)
	}
	defer rows.Close()

	vehicles := []Vehicle{}
	for rows.Next() {
		var v Vehicle
		if err := rows.Scan(&v.ID, &v.CustomerID, &v.Plate, &v.Brand, &v.Model, &v.MileageKm); err != nil {
			return nil, fmt.Errorf("store: scan vehicle: %w", err)
		}
		vehicles = append(vehicles, v)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("store: list vehicles: %w", err)
	}
	return vehicles, nil
}

// normalizePlate trims and upper-cases a plate so writes and reads agree.
func normalizePlate(plate string) string {
	return strings.ToUpper(strings.TrimSpace(plate))
}
