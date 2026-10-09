package handlers

import "net/http"

// CreateCustomer handles POST /api/customers.
func CreateCustomer(w http.ResponseWriter, r *http.Request) {
	notImplemented(w, "Das Anlegen von Kunden")
}

// GetCustomer handles GET /api/customers/{id}.
func GetCustomer(w http.ResponseWriter, r *http.Request) {
	notImplemented(w, "Das Abrufen von Kunden")
}
