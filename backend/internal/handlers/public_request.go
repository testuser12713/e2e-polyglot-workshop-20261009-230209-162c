package handlers

import (
	"errors"
	"log"
	"net/http"
	"regexp"
	"strings"
	"time"

	"workshop/internal/deps"
	"workshop/internal/httpx"
	"workshop/internal/store"
)

// publicOrderRequest is the POST /api/orders body. mileage_km is optional and
// defaults to 0; every other field is required.
type publicOrderRequest struct {
	Name        string `json:"name"`
	Email       string `json:"email"`
	Phone       string `json:"phone"`
	Plate       string `json:"plate"`
	Brand       string `json:"brand"`
	Model       string `json:"model"`
	MileageKm   int    `json:"mileage_km"`
	DesiredDate string `json:"desired_date"`
	Description string `json:"description"`
}

// publicOrderResponse is the 201 body: the new order number and its status.
type publicOrderResponse struct {
	OrderNumber string `json:"order_number"`
	Status      string `json:"status"`
}

// dateLayout is the ISO calendar date the client sends ("2006-01-02").
const dateLayout = "2006-01-02"

// emailPattern mirrors the client-side check in the appointment form.
var emailPattern = regexp.MustCompile(`^[^\s@]+@[^\s@]+\.[^\s@]+$`)

// CreatePublicOrder handles POST /api/orders for the public appointment request:
// it validates the payload, then creates customer, vehicle and order in one
// transaction with a fresh order number in status requested.
func CreatePublicOrder(w http.ResponseWriter, r *http.Request) {
	var req publicOrderRequest
	if err := httpx.DecodeJSON(r, &req); err != nil {
		httpx.WriteError(w, http.StatusBadRequest, httpx.CodeBadRequest,
			"Die Anfrage ist unvollständig oder fehlerhaft.")
		return
	}

	in, ok := validatePublicOrder(req)
	if !ok {
		httpx.WriteError(w, http.StatusBadRequest, httpx.CodeBadRequest,
			"Bitte füllen Sie alle Pflichtfelder korrekt aus.")
		return
	}

	result, err := store.CreateRequestedOrder(r.Context(), deps.DB(), in)
	switch {
	case err == nil:
		httpx.WriteJSON(w, http.StatusCreated, publicOrderResponse{
			OrderNumber: result.OrderNumber,
			Status:      result.Status,
		})
	case errors.Is(err, store.ErrPlateTaken):
		httpx.WriteError(w, http.StatusConflict, httpx.CodePlateTaken,
			"Dieses Kennzeichen ist bereits erfasst.")
	default:
		// The error may embed the submitted values, so only its type is logged
		// (AC-30).
		log.Printf("handlers: POST /api/orders failed: %T", err)
		httpx.WriteError(w, http.StatusInternalServerError, httpx.CodeInternal,
			"Die Anfrage konnte nicht gespeichert werden.")
	}
}

// validatePublicOrder trims and checks the payload, returning the store input.
// It never trusts the client: empty required fields, a malformed e-mail or
// desired_date and a negative mileage are all rejected.
func validatePublicOrder(req publicOrderRequest) (store.CreateOrderInput, bool) {
	in := store.CreateOrderInput{
		CustomerName:  strings.TrimSpace(req.Name),
		CustomerEmail: strings.TrimSpace(req.Email),
		CustomerPhone: strings.TrimSpace(req.Phone),
		Plate:         strings.ToUpper(strings.TrimSpace(req.Plate)),
		Brand:         strings.TrimSpace(req.Brand),
		Model:         strings.TrimSpace(req.Model),
		MileageKm:     req.MileageKm,
		Description:   strings.TrimSpace(req.Description),
	}

	if in.CustomerName == "" || in.CustomerPhone == "" || in.Plate == "" ||
		in.Brand == "" || in.Model == "" || in.Description == "" {
		return store.CreateOrderInput{}, false
	}
	if !emailPattern.MatchString(in.CustomerEmail) {
		return store.CreateOrderInput{}, false
	}
	if in.MileageKm < 0 {
		return store.CreateOrderInput{}, false
	}

	desiredDate, err := time.Parse(dateLayout, strings.TrimSpace(req.DesiredDate))
	if err != nil {
		return store.CreateOrderInput{}, false
	}
	in.DesiredDate = desiredDate
	return in, true
}
