package store

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// DashboardReport holds the three figures the workshop dashboard shows (AC-17).
type DashboardReport struct {
	OpenOrders        int
	FinishedToday     int
	MonthRevenueCents int64
}

// dashboardQuery computes all three figures in one round trip. Every date
// boundary is derived from the single bound parameter $1, the reference instant
// "now", so the query depends on no server-local literal and stays testable.
//
//   - open_orders: every order that has not reached picked_up yet.
//   - finished_today: every distinct order whose history carries a done entry
//     dated on the reference day (an order that was finished and later picked up
//     still counts).
//   - month_revenue_cents: the gross sum of the invoices issued in the calendar
//     month of the reference instant; no invoice means 0, never NULL.
const dashboardQuery = `
SELECT
    (SELECT count(*) FROM orders WHERE status <> 'picked_up'),
    (SELECT count(DISTINCT h.order_id)
       FROM order_history h
      WHERE h.status = 'done'
        AND h.changed_at >= date_trunc('day', $1::timestamptz)
        AND h.changed_at <  date_trunc('day', $1::timestamptz) + interval '1 day'),
    (SELECT COALESCE(sum(i.gross_cents), 0)
       FROM invoices i
      WHERE i.issued_at >= date_trunc('month', $1::timestamptz)
        AND i.issued_at <  date_trunc('month', $1::timestamptz) + interval '1 month')
`

// LoadDashboardReport runs the dashboard aggregate against the real PostgreSQL
// pool. now is bound as a query parameter, never interpolated into the SQL.
func LoadDashboardReport(ctx context.Context, pool *pgxpool.Pool, now time.Time) (DashboardReport, error) {
	var rep DashboardReport
	err := pool.QueryRow(ctx, dashboardQuery, now).Scan(
		&rep.OpenOrders,
		&rep.FinishedToday,
		&rep.MonthRevenueCents,
	)
	if err != nil {
		return DashboardReport{}, fmt.Errorf("store: load dashboard report: %w", err)
	}
	return rep, nil
}
