package handlers

import (
	"errors"
	"net/http"
	"strings"

	"workshop/internal/deps"
	"workshop/internal/httpx"
	"workshop/internal/store"
)

// maxLaborHours is the largest value the numeric(6,2) column accepts. Validating
// it here keeps an out-of-range request a readable 400 instead of a database
// error surfacing as 500.
const maxLaborHours = 9999.99

// createOrderItemRequest is the body of POST /api/orders/{nr}/items. Optional
// fields are pointers so an absent value is distinguishable from zero.
type createOrderItemRequest struct {
	Kind           string   `json:"kind"`
	Description    string   `json:"description"`
	Hours          *float64 `json:"hours"`
	Quantity       *int     `json:"quantity"`
	UnitPriceCents *int     `json:"unit_price_cents"`
}

// orderItemResponse is the shared item shape: a labor position carries hours, a
// part position carries quantity and unit_price_cents, the others are null.
type orderItemResponse struct {
	ID             string   `json:"id"`
	Kind           string   `json:"kind"`
	Description    string   `json:"description"`
	Hours          *float64 `json:"hours"`
	Quantity       *int     `json:"quantity"`
	UnitPriceCents *int     `json:"unit_price_cents"`
}

// CreateOrderItem handles POST /api/orders/{nr}/items. It validates that a labor
// position carries hours and a part position carries quantity and a unit price in
// whole cents, stores the position and answers 201 with the created item.
func CreateOrderItem(w http.ResponseWriter, r *http.Request) {
	orderNumber := r.PathValue("nr")

	var req createOrderItemRequest
	if err := httpx.DecodeJSON(r, &req); err != nil {
		httpx.WriteError(w, http.StatusBadRequest, httpx.CodeBadRequest,
			"Ungültige Positionsdaten.")
		return
	}

	item, ok := validateOrderItem(w, req)
	if !ok {
		return
	}

	db := deps.DB()
	if db == nil {
		httpx.WriteError(w, http.StatusInternalServerError, httpx.CodeInternal,
			"Interner Fehler beim Speichern der Position.")
		return
	}

	created, err := store.CreateOrderItem(r.Context(), db, orderNumber, item)
	switch {
	case errors.Is(err, store.ErrOrderNotFound):
		httpx.WriteError(w, http.StatusNotFound, httpx.CodeNotFound,
			"Auftrag nicht gefunden.")
		return
	case err != nil:
		httpx.WriteError(w, http.StatusInternalServerError, httpx.CodeInternal,
			"Interner Fehler beim Speichern der Position.")
		return
	}

	httpx.WriteJSON(w, http.StatusCreated, orderItemResponse{
		ID:             created.ID,
		Kind:           created.Kind,
		Description:    created.Description,
		Hours:          created.Hours,
		Quantity:       created.Quantity,
		UnitPriceCents: created.UnitPriceCents,
	})
}

// validateOrderItem checks the request against the item contract and returns the
// storage input, or writes a 400 and returns false.
func validateOrderItem(w http.ResponseWriter, req createOrderItemRequest) (store.NewOrderItem, bool) {
	description := strings.TrimSpace(req.Description)
	if description == "" {
		httpx.WriteError(w, http.StatusBadRequest, httpx.CodeBadRequest,
			"Die Bezeichnung darf nicht leer sein.")
		return store.NewOrderItem{}, false
	}

	switch req.Kind {
	case "labor":
		if req.Hours == nil || *req.Hours <= 0 || *req.Hours > maxLaborHours {
			httpx.WriteError(w, http.StatusBadRequest, httpx.CodeBadRequest,
				"Für eine Arbeitszeit wird eine Stundenzahl größer 0 benötigt.")
			return store.NewOrderItem{}, false
		}
		return store.NewOrderItem{
			Kind:        "labor",
			Description: description,
			Hours:       req.Hours,
		}, true

	case "part":
		if req.Quantity == nil || *req.Quantity <= 0 {
			httpx.WriteError(w, http.StatusBadRequest, httpx.CodeBadRequest,
				"Für ein Teil wird eine Menge größer 0 benötigt.")
			return store.NewOrderItem{}, false
		}
		if req.UnitPriceCents == nil || *req.UnitPriceCents < 0 {
			httpx.WriteError(w, http.StatusBadRequest, httpx.CodeBadRequest,
				"Für ein Teil wird ein Einzelpreis in ganzen Cent benötigt.")
			return store.NewOrderItem{}, false
		}
		return store.NewOrderItem{
			Kind:           "part",
			Description:    description,
			Quantity:       req.Quantity,
			UnitPriceCents: req.UnitPriceCents,
		}, true

	default:
		httpx.WriteError(w, http.StatusBadRequest, httpx.CodeBadRequest,
			"Unbekannte Positionsart.")
		return store.NewOrderItem{}, false
	}
}
