package handlers

import (
	"errors"
	"net/http"

	"workshop/internal/deps"
	"workshop/internal/httpx"
	"workshop/internal/store"
)

// ListOrders handles GET /api/orders. It returns the orders matching an
// optional exact status filter and an optional plate substring, otherwise all.
func ListOrders(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query()
	orders, err := store.ListOrders(r.Context(), deps.DB(), query.Get("status"), query.Get("plate"))
	if err != nil {
		httpx.WriteError(w, http.StatusInternalServerError, httpx.CodeInternal,
			"Die Auftragsliste konnte nicht geladen werden.")
		return
	}
	httpx.WriteJSON(w, http.StatusOK, orders)
}

// GetOrder handles GET /api/orders/{nr}. It returns the fully assembled order
// with customer, vehicle, positions, history and the invoice (or null); an
// unknown order number answers 404.
func GetOrder(w http.ResponseWriter, r *http.Request) {
	order, err := store.GetOrder(r.Context(), deps.DB(), r.PathValue("nr"))
	switch {
	case errors.Is(err, store.ErrOrderNotFound):
		httpx.WriteError(w, http.StatusNotFound, httpx.CodeNotFound, "Der Auftrag wurde nicht gefunden.")
		return
	case err != nil:
		httpx.WriteError(w, http.StatusInternalServerError, httpx.CodeInternal,
			"Der Auftrag konnte nicht geladen werden.")
		return
	}
	httpx.WriteJSON(w, http.StatusOK, order)
}
