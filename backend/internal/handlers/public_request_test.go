package handlers_test

import (
	"bytes"
	"crypto/rand"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"testing"

	"workshop/internal/deps"
)

// validOrderPayload returns a complete, valid POST /api/orders body with the
// given plate.
func validOrderPayload(plate string) map[string]any {
	return map[string]any{
		"name":         "Erika Mustermann",
		"email":        "erika@example.com",
		"phone":        "+49 30 1234567",
		"plate":        plate,
		"brand":        "VW",
		"model":        "Golf",
		"mileage_km":   123456,
		"desired_date": "2099-05-17",
		"description":  "Bremsen quietschen beim Anhalten.",
	}
}

// randomPlate builds a plate that is very unlikely to collide with a row left
// behind by an earlier local run.
func randomPlate(t *testing.T) string {
	t.Helper()
	const letters = "ABCDEFGHIJKLMNOPQRSTUVWXYZ"
	const digits = "0123456789"
	buf := make([]byte, 6)
	if _, err := rand.Read(buf); err != nil {
		t.Fatalf("rand: %v", err)
	}
	return fmt.Sprintf("%c%c-%c%c %c%c%c%c",
		letters[int(buf[0])%26], letters[int(buf[1])%26],
		letters[int(buf[2])%26], letters[int(buf[3])%26],
		digits[int(buf[4])%10], digits[int(buf[5])%10],
		digits[int(buf[4]+7)%10], digits[int(buf[5]+3)%10],
	)
}

// postOrder sends a JSON body to POST /api/orders and returns the status and a
// decoded error code (empty when the body has no error envelope).
func postOrder(t *testing.T, url string, body any) (int, string, map[string]any) {
	t.Helper()
	raw, err := json.Marshal(body)
	if err != nil {
		t.Fatalf("marshal body: %v", err)
	}
	resp, err := http.Post(url+"/api/orders", "application/json", bytes.NewReader(raw))
	if err != nil {
		t.Fatalf("POST /api/orders: %v", err)
	}
	defer resp.Body.Close()

	var decoded map[string]any
	_ = json.NewDecoder(resp.Body).Decode(&decoded)

	code := ""
	if env, ok := decoded["error"].(map[string]any); ok {
		code, _ = env["code"].(string)
	}
	return resp.StatusCode, code, decoded
}

// TestCreatePublicOrderHappyPath proves a complete request returns 201 with a
// contract-shaped order number, status requested, and persists the order.
func TestCreatePublicOrderHappyPath(t *testing.T) {
	srv := newTestServer(t)
	plate := randomPlate(t)

	status, code, body := postOrder(t, srv.URL, validOrderPayload(plate))
	if status != http.StatusCreated {
		t.Fatalf("got status %d (%s), want 201", status, code)
	}

	orderNumber, _ := body["order_number"].(string)
	if !strings.HasPrefix(orderNumber, "AU-") || len(orderNumber) != 9 {
		t.Fatalf("order_number %q does not match AU- + 6 chars", orderNumber)
	}
	for _, r := range orderNumber[3:] {
		if !((r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9')) {
			t.Fatalf("order_number %q contains an invalid character %q", orderNumber, r)
		}
	}
	if got, _ := body["status"].(string); got != "requested" {
		t.Fatalf("status: got %q, want requested", got)
	}

	// The order must really exist in status requested, so the (separately owned)
	// status lookup can find it via order number and plate.
	var storedStatus string
	err := deps.DB().QueryRow(t.Context(),
		`SELECT o.status
		   FROM orders o
		   JOIN vehicles v ON v.id = o.vehicle_id
		  WHERE o.order_number = $1 AND v.plate = $2`,
		orderNumber, plate,
	).Scan(&storedStatus)
	if err != nil {
		t.Fatalf("query created order: %v", err)
	}
	if storedStatus != "requested" {
		t.Fatalf("stored status: got %q, want requested", storedStatus)
	}

	var historyCount int
	if err := deps.DB().QueryRow(t.Context(),
		`SELECT count(*) FROM order_history h
		   JOIN orders o ON o.id = h.order_id
		  WHERE o.order_number = $1 AND h.status = 'requested'`,
		orderNumber,
	).Scan(&historyCount); err != nil {
		t.Fatalf("query order history: %v", err)
	}
	if historyCount != 1 {
		t.Fatalf("order history for requested: got %d rows, want 1", historyCount)
	}
}

// TestCreatePublicOrderMissingField proves an incomplete payload is rejected
// with 400 and the stable bad_request code.
func TestCreatePublicOrderMissingField(t *testing.T) {
	srv := newTestServer(t)

	payload := validOrderPayload(randomPlate(t))
	delete(payload, "name")

	status, code, _ := postOrder(t, srv.URL, payload)
	if status != http.StatusBadRequest {
		t.Fatalf("got status %d, want 400", status)
	}
	if code != "bad_request" {
		t.Fatalf("error code: got %q, want bad_request", code)
	}
}

// TestCreatePublicOrderDuplicatePlate proves a plate that already exists is
// rejected with 409 plate_taken.
func TestCreatePublicOrderDuplicatePlate(t *testing.T) {
	srv := newTestServer(t)
	plate := randomPlate(t)

	status, code, _ := postOrder(t, srv.URL, validOrderPayload(plate))
	if status != http.StatusCreated {
		t.Fatalf("first request: got status %d (%s), want 201", status, code)
	}

	// A second request with the same plate (and even a different customer) must
	// be refused.
	second := validOrderPayload(strings.ToLower(plate))
	second["name"] = "Max Beispiel"

	status, code, _ = postOrder(t, srv.URL, second)
	if status != http.StatusConflict {
		t.Fatalf("duplicate plate: got status %d, want 409", status)
	}
	if code != "plate_taken" {
		t.Fatalf("duplicate plate error code: got %q, want plate_taken", code)
	}
}
