package handlers

import "net/http"

// DashboardReport handles GET /api/reports/dashboard.
func DashboardReport(w http.ResponseWriter, r *http.Request) {
	notImplemented(w, "Das Dashboard")
}
