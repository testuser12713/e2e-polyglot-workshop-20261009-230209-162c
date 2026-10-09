package handlers_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"workshop/internal/handlers"
)

// publicOrderDTO pins the exact public status shape independently of the store
// type, so a renamed JSON field fails the test.
type publicOrderDTO struct {
	OrderNumber string       `json:"order_number"`
	Status      string       `json:"status"`
	DesiredDate time.Time    `json:"desired_date"`
	Description string       `json:"description"`
	Plate       string       `json:"plate"`
	History     []historyDTO `json:"history"`
}

// publicInvoiceResponseDTO pins the {"invoice": <invoice>|null} envelope.
type publicInvoiceResponseDTO struct {
	Invoice *invoiceDTO `json:"invoice"`
}

// apiErrorEnvelope is the standard non-2xx body.
type apiErrorEnvelope struct {
	Error struct {
		Code    string `json:"code"`
		Message string `json:"message"`
	} `json:"error"`
}

func lookupStatus(t *testing.T, orderNumber, plate string) *httptest.ResponseRecorder {
	t.Helper()
	target := "/api/public/orders/" + url.PathEscape(orderNumber) + "?plate=" + url.QueryEscape(plate)
	req := httptest.NewRequest(http.MethodGet, target, nil)
	req.SetPathValue("nr", orderNumber)
	rec := httptest.NewRecorder()
	handlers.PublicOrderStatus(rec, req)
	return rec
}

func lookupInvoice(t *testing.T, orderNumber, plate string) *httptest.ResponseRecorder {
	t.Helper()
	target := "/api/public/orders/" + url.PathEscape(orderNumber) + "/invoice?plate=" + url.QueryEscape(plate)
	req := httptest.NewRequest(http.MethodGet, target, nil)
	req.SetPathValue("nr", orderNumber)
	rec := httptest.NewRecorder()
	handlers.PublicOrderInvoice(rec, req)
	return rec
}

func decodeError(t *testing.T, rec *httptest.ResponseRecorder) apiErrorEnvelope {
	t.Helper()
	var env apiErrorEnvelope
	if err := json.Unmarshal(rec.Body.Bytes(), &env); err != nil {
		t.Fatalf("decode error body: %v (body %s)", err, rec.Body.String())
	}
	return env
}

// TestPublicOrderStatusLookup covers AC-11: the matching order number and plate
// return status and history, while a wrong plate or an unknown order number
// answers 404 with the standard error body.
func TestPublicOrderStatusLookup(t *testing.T) {
	pool := readTestDB(t)
	fx := seedOrders(t, pool)

	t.Run("matching order number and plate", func(t *testing.T) {
		rec := lookupStatus(t, fx.orderA, fx.plateA)
		if rec.Code != http.StatusOK {
			t.Fatalf("got %d, want 200 (body %s)", rec.Code, rec.Body.String())
		}
		var got publicOrderDTO
		if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
			t.Fatalf("decode public order: %v (body %s)", err, rec.Body.String())
		}
		if got.OrderNumber != fx.orderA {
			t.Errorf("order_number: got %q, want %q", got.OrderNumber, fx.orderA)
		}
		if got.Status != "in_progress" {
			t.Errorf("status: got %q, want in_progress", got.Status)
		}
		if got.Plate != fx.plateA {
			t.Errorf("plate: got %q, want %q", got.Plate, fx.plateA)
		}
		if got.Description != "Bremsen quietschen" {
			t.Errorf("description: got %q", got.Description)
		}
		if got.DesiredDate.Year() != 2026 || got.DesiredDate.Month() != time.November || got.DesiredDate.Day() != 2 {
			t.Errorf("desired_date: got %v", got.DesiredDate)
		}
		if len(got.History) != 3 {
			t.Fatalf("history: got %d, want 3 (%+v)", len(got.History), got.History)
		}
		wantStatuses := []string{"requested", "confirmed", "in_progress"}
		for i, want := range wantStatuses {
			if got.History[i].Status != want {
				t.Errorf("history[%d]: got %q, want %q", i, got.History[i].Status, want)
			}
			if got.History[i].ChangedAt.IsZero() {
				t.Errorf("history[%d] changed_at is zero", i)
			}
		}

		// The public view must carry no personal data.
		body := rec.Body.String()
		if strings.Contains(body, "kunde-"+fx.plateATag) || strings.Contains(body, "0123-"+fx.plateATag) {
			t.Errorf("public order leaked personal data: %s", body)
		}
	})

	t.Run("case-insensitive plate matches", func(t *testing.T) {
		rec := lookupStatus(t, fx.orderA, strings.ToLower(fx.plateA))
		if rec.Code != http.StatusOK {
			t.Fatalf("lower-case plate: got %d, want 200 (body %s)", rec.Code, rec.Body.String())
		}
	})

	t.Run("wrong plate answers 404", func(t *testing.T) {
		rec := lookupStatus(t, fx.orderA, fx.plateB)
		if rec.Code != http.StatusNotFound {
			t.Fatalf("wrong plate: got %d, want 404 (body %s)", rec.Code, rec.Body.String())
		}
		env := decodeError(t, rec)
		if env.Error.Code == "" || env.Error.Message == "" {
			t.Errorf("error envelope: %+v", env.Error)
		}
	})

	t.Run("unknown order number answers 404", func(t *testing.T) {
		rec := lookupStatus(t, "AU-NOPE99", fx.plateA)
		if rec.Code != http.StatusNotFound {
			t.Fatalf("unknown order: got %d, want 404 (body %s)", rec.Code, rec.Body.String())
		}
		if env := decodeError(t, rec); env.Error.Code == "" || env.Error.Message == "" {
			t.Errorf("error envelope: %+v", env.Error)
		}
	})
}

// TestPublicOrderInvoiceLookup covers the invoice half: an issued invoice is
// returned with its amounts and items, an order without one is a 200 with null,
// and a wrong or unknown pair answers 404.
func TestPublicOrderInvoiceLookup(t *testing.T) {
	pool := readTestDB(t)
	fx := seedOrders(t, pool)

	t.Run("order with invoice", func(t *testing.T) {
		rec := lookupInvoice(t, fx.orderA, fx.plateA)
		if rec.Code != http.StatusOK {
			t.Fatalf("got %d, want 200 (body %s)", rec.Code, rec.Body.String())
		}
		var got publicInvoiceResponseDTO
		if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
			t.Fatalf("decode invoice response: %v (body %s)", err, rec.Body.String())
		}
		if got.Invoice == nil {
			t.Fatal("invoice: got null, want the issued invoice")
		}
		if got.Invoice.InvoiceNumber == "" || got.Invoice.IssuedAt.IsZero() {
			t.Errorf("invoice identity: %+v", *got.Invoice)
		}
		if got.Invoice.NetCents != 20000 || got.Invoice.VatCents != 3800 || got.Invoice.GrossCents != 23800 {
			t.Errorf("invoice amounts: %+v", *got.Invoice)
		}
		if len(got.Invoice.Items) != 2 {
			t.Errorf("invoice items: got %d, want 2 (%+v)", len(got.Invoice.Items), got.Invoice.Items)
		}
	})

	t.Run("order without invoice is a 200 with null", func(t *testing.T) {
		rec := lookupInvoice(t, fx.orderC, fx.plateB)
		if rec.Code != http.StatusOK {
			t.Fatalf("got %d, want 200 (body %s)", rec.Code, rec.Body.String())
		}
		if !strings.Contains(rec.Body.String(), `"invoice":null`) {
			t.Errorf("body: got %s, want an explicit null invoice", rec.Body.String())
		}
		var got publicInvoiceResponseDTO
		if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
			t.Fatalf("decode invoice response: %v (body %s)", err, rec.Body.String())
		}
		if got.Invoice != nil {
			t.Errorf("invoice: got %+v, want null", *got.Invoice)
		}
	})

	t.Run("wrong plate answers 404", func(t *testing.T) {
		rec := lookupInvoice(t, fx.orderA, fx.plateB)
		if rec.Code != http.StatusNotFound {
			t.Fatalf("wrong plate: got %d, want 404 (body %s)", rec.Code, rec.Body.String())
		}
		if env := decodeError(t, rec); env.Error.Code == "" || env.Error.Message == "" {
			t.Errorf("error envelope: %+v", env.Error)
		}
	})

	t.Run("unknown order number answers 404", func(t *testing.T) {
		rec := lookupInvoice(t, "AU-NOPE99", fx.plateA)
		if rec.Code != http.StatusNotFound {
			t.Fatalf("unknown order: got %d, want 404 (body %s)", rec.Code, rec.Body.String())
		}
		if env := decodeError(t, rec); env.Error.Code == "" || env.Error.Message == "" {
			t.Errorf("error envelope: %+v", env.Error)
		}
	})
}
