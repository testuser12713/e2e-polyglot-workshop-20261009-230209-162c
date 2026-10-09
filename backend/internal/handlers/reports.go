package handlers

import (
	"net/http"
	"time"

	"workshop/internal/deps"
	"workshop/internal/httpx"
	"workshop/internal/store"
)

// dashboardResponse is the exact body of GET /api/reports/dashboard.
type dashboardResponse struct {
	OpenOrders        int   `json:"open_orders"`
	FinishedToday     int   `json:"finished_today"`
	MonthRevenueCents int64 `json:"month_revenue_cents"`
}

// DashboardReport handles GET /api/reports/dashboard. It answers the number of
// open orders, the orders finished today and the current month's revenue as
// whole cents, all read from the real PostgreSQL (AC-17).
func DashboardReport(w http.ResponseWriter, r *http.Request) {
	pool := deps.DB()
	if pool == nil {
		httpx.WriteError(w, http.StatusServiceUnavailable, httpx.CodeServiceUnavailable,
			"Datenbank nicht erreichbar.")
		return
	}

	rep, err := store.LoadDashboardReport(r.Context(), pool, time.Now())
	if err != nil {
		httpx.WriteError(w, http.StatusInternalServerError, httpx.CodeInternal,
			"Das Dashboard konnte nicht geladen werden.")
		return
	}

	httpx.WriteJSON(w, http.StatusOK, dashboardResponse{
		OpenOrders:        rep.OpenOrders,
		FinishedToday:     rep.FinishedToday,
		MonthRevenueCents: rep.MonthRevenueCents,
	})
}
