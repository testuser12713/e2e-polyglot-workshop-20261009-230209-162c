package handlers

import "net/http"

// ListOrders handles GET /api/orders.
func ListOrders(w http.ResponseWriter, r *http.Request) {
	notImplemented(w, "Die Auftragsliste")
}

// GetOrder handles GET /api/orders/{nr}.
func GetOrder(w http.ResponseWriter, r *http.Request) {
	notImplemented(w, "Die Auftragsdetails")
}
