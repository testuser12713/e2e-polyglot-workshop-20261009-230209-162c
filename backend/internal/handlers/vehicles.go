package handlers

import (
	"errors"
	"net/http"
	"strings"

	"workshop/internal/deps"
	"workshop/internal/httpx"
	"workshop/internal/store"
)

// createVehicleRequest is the POST /api/vehicles body.
type createVehicleRequest struct {
	CustomerID string `json:"customer_id"`
	Plate      string `json:"plate"`
	Brand      string `json:"brand"`
	Model      string `json:"model"`
	MileageKm  int    `json:"mileage_km"`
}

// CreateVehicle handles POST /api/vehicles. It assigns a vehicle to a customer
// and answers 201; a duplicate plate is rejected with 409 plate_taken while the
// existing vehicle stays unchanged.
func CreateVehicle(w http.ResponseWriter, r *http.Request) {
	var req createVehicleRequest
	if err := httpx.DecodeJSON(r, &req); err != nil {
		httpx.WriteError(w, http.StatusBadRequest, httpx.CodeBadRequest, "Ungültige Anfrage.")
		return
	}

	customerID := strings.TrimSpace(req.CustomerID)
	plate := strings.TrimSpace(req.Plate)
	brand := strings.TrimSpace(req.Brand)
	model := strings.TrimSpace(req.Model)
	if customerID == "" || plate == "" || brand == "" || model == "" {
		httpx.WriteError(w, http.StatusBadRequest, httpx.CodeBadRequest,
			"Kunde, Kennzeichen, Marke und Modell sind erforderlich.")
		return
	}
	if req.MileageKm < 0 {
		httpx.WriteError(w, http.StatusBadRequest, httpx.CodeBadRequest,
			"Der Kilometerstand darf nicht negativ sein.")
		return
	}

	v, err := store.CreateVehicle(r.Context(), deps.DB(), customerID, plate, brand, model, req.MileageKm)
	switch {
	case errors.Is(err, store.ErrPlateTaken):
		httpx.WriteError(w, http.StatusConflict, httpx.CodePlateTaken,
			"Dieses Kennzeichen ist bereits einem Fahrzeug zugeordnet.")
	case errors.Is(err, store.ErrCustomerNotFound):
		httpx.WriteError(w, http.StatusBadRequest, httpx.CodeBadRequest,
			"Der angegebene Kunde existiert nicht.")
	case err != nil:
		httpx.WriteError(w, http.StatusInternalServerError, httpx.CodeInternal,
			"Das Fahrzeug konnte nicht angelegt werden.")
	default:
		httpx.WriteJSON(w, http.StatusCreated, v)
	}
}

// ListVehicles handles GET /api/vehicles?plate=X. The plate filter is optional;
// without it the newest vehicles are returned.
func ListVehicles(w http.ResponseWriter, r *http.Request) {
	vehicles, err := store.ListVehicles(r.Context(), deps.DB(), r.URL.Query().Get("plate"))
	if err != nil {
		httpx.WriteError(w, http.StatusInternalServerError, httpx.CodeInternal,
			"Die Fahrzeuge konnten nicht geladen werden.")
		return
	}
	httpx.WriteJSON(w, http.StatusOK, vehicles)
}
