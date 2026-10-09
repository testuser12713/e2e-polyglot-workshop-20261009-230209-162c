package handlers

import (
	"errors"
	"net/http"
	"strings"

	"workshop/internal/deps"
	"workshop/internal/httpx"
	"workshop/internal/store"
)

// createCustomerRequest is the POST /api/customers body.
type createCustomerRequest struct {
	Name  string `json:"name"`
	Email string `json:"email"`
	Phone string `json:"phone"`
}

// CreateCustomer handles POST /api/customers. It creates a customer with name,
// email and phone and answers 201 with the stored customer.
func CreateCustomer(w http.ResponseWriter, r *http.Request) {
	var req createCustomerRequest
	if err := httpx.DecodeJSON(r, &req); err != nil {
		httpx.WriteError(w, http.StatusBadRequest, httpx.CodeBadRequest, "Ungültige Anfrage.")
		return
	}

	name := strings.TrimSpace(req.Name)
	email := strings.TrimSpace(req.Email)
	phone := strings.TrimSpace(req.Phone)
	if name == "" || email == "" || phone == "" {
		httpx.WriteError(w, http.StatusBadRequest, httpx.CodeBadRequest,
			"Name, E-Mail und Telefon sind erforderlich.")
		return
	}

	c, err := store.CreateCustomer(r.Context(), deps.DB(), name, email, phone)
	if err != nil {
		httpx.WriteError(w, http.StatusInternalServerError, httpx.CodeInternal,
			"Der Kunde konnte nicht angelegt werden.")
		return
	}
	httpx.WriteJSON(w, http.StatusCreated, c)
}

// GetCustomer handles GET /api/customers/{id}. It answers 200 with the customer
// or 404 when no such customer exists.
func GetCustomer(w http.ResponseWriter, r *http.Request) {
	c, err := store.GetCustomer(r.Context(), deps.DB(), r.PathValue("id"))
	switch {
	case errors.Is(err, store.ErrCustomerNotFound):
		httpx.WriteError(w, http.StatusNotFound, httpx.CodeNotFound, "Kunde nicht gefunden.")
	case err != nil:
		httpx.WriteError(w, http.StatusInternalServerError, httpx.CodeInternal,
			"Der Kunde konnte nicht geladen werden.")
	default:
		httpx.WriteJSON(w, http.StatusOK, c)
	}
}
