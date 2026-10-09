package handlers

import (
	"errors"
	"net/http"

	"workshop/internal/deps"
	"workshop/internal/httpx"
	"workshop/internal/store"
)

// publicInvoiceResponse is the body of GET /api/public/orders/{nr}/invoice: the
// invoice when the worker has issued one, otherwise an explicit null. It is
// never an empty 200.
type publicInvoiceResponse struct {
	Invoice *store.OrderInvoice `json:"invoice"`
}

// PublicOrderStatus handles GET /api/public/orders/{nr}?plate=X. It returns the
// privacy-safe status view of the order matching the order number and plate; a
// wrong plate or an unknown order number answers 404 with the standard error
// envelope.
func PublicOrderStatus(w http.ResponseWriter, r *http.Request) {
	order, err := store.GetPublicOrder(r.Context(), deps.DB(), r.PathValue("nr"), r.URL.Query().Get("plate"))
	switch {
	case errors.Is(err, store.ErrOrderNotFound):
		httpx.WriteError(w, http.StatusNotFound, httpx.CodeNotFound,
			"Zu dieser Kombination aus Auftragsnummer und Kennzeichen wurde kein Auftrag gefunden.")
		return
	case err != nil:
		httpx.WriteError(w, http.StatusInternalServerError, httpx.CodeInternal,
			"Der Auftrag konnte nicht geladen werden.")
		return
	}
	httpx.WriteJSON(w, http.StatusOK, order)
}

// PublicOrderInvoice handles GET /api/public/orders/{nr}/invoice?plate=X. It
// returns {"invoice": <invoice>|null}: an order without an invoice is a 200 with
// null, a wrong combination answers 404 with the standard error envelope.
func PublicOrderInvoice(w http.ResponseWriter, r *http.Request) {
	invoice, err := store.GetPublicInvoice(r.Context(), deps.DB(), r.PathValue("nr"), r.URL.Query().Get("plate"))
	switch {
	case errors.Is(err, store.ErrOrderNotFound):
		httpx.WriteError(w, http.StatusNotFound, httpx.CodeNotFound,
			"Zu dieser Kombination aus Auftragsnummer und Kennzeichen wurde kein Auftrag gefunden.")
		return
	case err != nil:
		httpx.WriteError(w, http.StatusInternalServerError, httpx.CodeInternal,
			"Die Rechnung konnte nicht geladen werden.")
		return
	}
	httpx.WriteJSON(w, http.StatusOK, publicInvoiceResponse{Invoice: invoice})
}
