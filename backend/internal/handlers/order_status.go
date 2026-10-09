package handlers

import "net/http"

// UpdateOrderStatus handles POST /api/orders/{nr}/status.
func UpdateOrderStatus(w http.ResponseWriter, r *http.Request) {
	notImplemented(w, "Der Statuswechsel")
}
