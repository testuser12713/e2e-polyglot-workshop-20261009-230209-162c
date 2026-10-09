package handlers

import "net/http"

// CreateVehicle handles POST /api/vehicles.
func CreateVehicle(w http.ResponseWriter, r *http.Request) {
	notImplemented(w, "Das Anlegen von Fahrzeugen")
}

// ListVehicles handles GET /api/vehicles.
func ListVehicles(w http.ResponseWriter, r *http.Request) {
	notImplemented(w, "Das Suchen von Fahrzeugen")
}
