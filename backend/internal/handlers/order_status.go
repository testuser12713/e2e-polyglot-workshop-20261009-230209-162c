package handlers

import (
	"errors"
	"net/http"

	"workshop/internal/deps"
	"workshop/internal/httpx"
	"workshop/internal/store"
)

// orderFlow is the one accepted order of statuses (AC-04 / shared interface):
// requested -> confirmed -> in_progress -> done -> picked_up. A transition is
// legal only to the immediate successor; everything else is rejected with 409
// invalid_transition and leaves the stored status untouched.
var orderFlow = []string{"requested", "confirmed", "in_progress", "done", "picked_up"}

// orderStatusRequest is the body of POST /api/orders/{nr}/status.
type orderStatusRequest struct {
	Status string `json:"status"`
}

// orderStatusResponse is the documented 200 shape {order_number,status,history}.
type orderStatusResponse struct {
	OrderNumber string               `json:"order_number"`
	Status      string               `json:"status"`
	History     []store.StatusChange `json:"history"`
}

// isValidOrderStatus reports whether s is one of the five known statuses.
func isValidOrderStatus(s string) bool {
	for _, known := range orderFlow {
		if known == s {
			return true
		}
	}
	return false
}

// nextOrderStatus returns the single status that may follow current, and whether
// a transition exists at all (picked_up is terminal).
func nextOrderStatus(current string) (string, bool) {
	for i, known := range orderFlow {
		if known == current {
			if i+1 < len(orderFlow) {
				return orderFlow[i+1], true
			}
			return "", false
		}
	}
	return "", false
}

// UpdateOrderStatus handles POST /api/orders/{nr}/status.
//
// It accepts only the next status in orderFlow, records the accepted transition
// in the order history with its timestamp, and, when the target status is done,
// pushes exactly one invoice message carrying the order number onto the Valkey
// list. An out-of-order request answers 409 invalid_transition without changing
// the stored status.
func UpdateOrderStatus(w http.ResponseWriter, r *http.Request) {
	orderNumber := r.PathValue("nr")

	var req orderStatusRequest
	if err := httpx.DecodeJSON(r, &req); err != nil {
		httpx.WriteError(w, http.StatusBadRequest, httpx.CodeBadRequest,
			"Der Statuswechsel ist unvollständig.")
		return
	}
	if !isValidOrderStatus(req.Status) {
		httpx.WriteError(w, http.StatusBadRequest, httpx.CodeBadRequest,
			"Unbekannter Status.")
		return
	}

	pool := deps.DB()
	if pool == nil {
		httpx.WriteError(w, http.StatusInternalServerError, httpx.CodeInternal,
			"Interner Fehler bei der Datenbankverbindung.")
		return
	}

	ctx := r.Context()
	tx, err := pool.Begin(ctx)
	if err != nil {
		httpx.WriteError(w, http.StatusInternalServerError, httpx.CodeInternal,
			"Interner Fehler beim Statuswechsel.")
		return
	}
	defer func() { _ = tx.Rollback(ctx) }()

	orderID, current, err := store.OrderStatusForUpdate(ctx, tx, orderNumber)
	switch {
	case errors.Is(err, store.ErrOrderNotFound):
		httpx.WriteError(w, http.StatusNotFound, httpx.CodeNotFound, "Auftrag nicht gefunden.")
		return
	case err != nil:
		httpx.WriteError(w, http.StatusInternalServerError, httpx.CodeInternal,
			"Interner Fehler beim Statuswechsel.")
		return
	}

	next, ok := nextOrderStatus(current)
	if !ok || req.Status != next {
		httpx.WriteError(w, http.StatusConflict, httpx.CodeInvalidTransition,
			"Dieser Statuswechsel ist nicht erlaubt.")
		return
	}

	if err := store.UpdateOrderStatus(ctx, tx, orderID, req.Status); err != nil {
		httpx.WriteError(w, http.StatusInternalServerError, httpx.CodeInternal,
			"Interner Fehler beim Statuswechsel.")
		return
	}
	if _, err := store.InsertStatusHistory(ctx, tx, orderID, req.Status); err != nil {
		httpx.WriteError(w, http.StatusInternalServerError, httpx.CodeInternal,
			"Interner Fehler beim Statuswechsel.")
		return
	}

	history, err := store.ListStatusHistory(ctx, tx, orderID)
	if err != nil {
		httpx.WriteError(w, http.StatusInternalServerError, httpx.CodeInternal,
			"Interner Fehler beim Statuswechsel.")
		return
	}

	if err := tx.Commit(ctx); err != nil {
		httpx.WriteError(w, http.StatusInternalServerError, httpx.CodeInternal,
			"Interner Fehler beim Statuswechsel.")
		return
	}

	// AC-07: "fertig" triggers the invoice. The transition is committed first, so
	// a message never exists for a status that was not stored; a retry of the
	// same call is already an illegal transition and therefore cannot enqueue a
	// second message.
	if req.Status == "done" {
		q := deps.Queue()
		if q == nil {
			httpx.WriteError(w, http.StatusInternalServerError, httpx.CodeInternal,
				"Rechnung konnte nicht in die Warteschlange gestellt werden.")
			return
		}
		if err := q.EnqueueInvoice(ctx, orderNumber); err != nil {
			httpx.WriteError(w, http.StatusInternalServerError, httpx.CodeInternal,
				"Rechnung konnte nicht in die Warteschlange gestellt werden.")
			return
		}
	}

	httpx.WriteJSON(w, http.StatusOK, orderStatusResponse{
		OrderNumber: orderNumber,
		Status:      req.Status,
		History:     history,
	})
}
