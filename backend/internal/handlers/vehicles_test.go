package handlers_test

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"
)

// vehicleBody mirrors the contracted vehicle shape.
type vehicleBody struct {
	ID         string `json:"id"`
	CustomerID string `json:"customer_id"`
	Plate      string `json:"plate"`
	Brand      string `json:"brand"`
	Model      string `json:"model"`
	MileageKm  int    `json:"mileage_km"`
}

// createCustomerForTest creates one customer for a vehicle test.
func createCustomerForTest(t *testing.T, srv *httptest.Server, token string) customerBody {
	t.Helper()
	var c customerBody
	doJSON(t, http.MethodPost, srv.URL+"/api/customers", token,
		map[string]string{
			"name":  "Test Kunde",
			"email": fmt.Sprintf("kunde-%d@example.com", time.Now().UnixNano()),
			"phone": "030 000000",
		}, &c, http.StatusCreated)
	return c
}

// uniquePlate returns a plate that no other test run will collide with.
func uniquePlate() string {
	return fmt.Sprintf("B-AB %d", time.Now().UnixNano()%100000000)
}

// vehiclesByPlate reads the vehicle list for an exact plate.
func vehiclesByPlate(t *testing.T, srv *httptest.Server, token, plate string) []vehicleBody {
	t.Helper()
	var list []vehicleBody
	doJSON(t, http.MethodGet, srv.URL+"/api/vehicles?plate="+url.QueryEscape(plate),
		token, nil, &list, http.StatusOK)
	return list
}

// TestVehicleCreateAndReadRoundTrip covers AC-02: a vehicle is assigned to a
// customer with plate, brand, model and mileage, then read back by plate.
func TestVehicleCreateAndReadRoundTrip(t *testing.T) {
	srv, token := newAuthTestServer(t)
	customer := createCustomerForTest(t, srv, token)
	plate := uniquePlate()

	var created vehicleBody
	doJSON(t, http.MethodPost, srv.URL+"/api/vehicles", token,
		map[string]any{
			"customer_id": customer.ID,
			"plate":       plate,
			"brand":       "VW",
			"model":       "Golf",
			"mileage_km":  12345,
		}, &created, http.StatusCreated)

	if created.ID == "" {
		t.Fatalf("created vehicle has no id: %+v", created)
	}
	if created.CustomerID != customer.ID || created.Brand != "VW" || created.Model != "Golf" || created.MileageKm != 12345 {
		t.Fatalf("created vehicle fields wrong: %+v", created)
	}

	list := vehiclesByPlate(t, srv, token, created.Plate)
	if len(list) != 1 || list[0] != created {
		t.Fatalf("round trip mismatch: got %+v, want [%+v]", list, created)
	}
}

// TestDuplicatePlateRejectedAndExistingUnchanged covers AC-02: a second vehicle
// with the same plate is rejected with 409 plate_taken and the first vehicle is
// left untouched.
func TestDuplicatePlateRejectedAndExistingUnchanged(t *testing.T) {
	srv, token := newAuthTestServer(t)
	customer := createCustomerForTest(t, srv, token)
	plate := uniquePlate()

	var first vehicleBody
	doJSON(t, http.MethodPost, srv.URL+"/api/vehicles", token,
		map[string]any{
			"customer_id": customer.ID,
			"plate":       plate,
			"brand":       "VW",
			"model":       "Golf",
			"mileage_km":  100,
		}, &first, http.StatusCreated)

	var body apiErrorBody
	doJSON(t, http.MethodPost, srv.URL+"/api/vehicles", token,
		map[string]any{
			"customer_id": customer.ID,
			"plate":       first.Plate,
			"brand":       "Audi",
			"model":       "A3",
			"mileage_km":  200,
		}, &body, http.StatusConflict)
	if body.Error.Code != "plate_taken" {
		t.Fatalf("unexpected error code: %q", body.Error.Code)
	}

	list := vehiclesByPlate(t, srv, token, first.Plate)
	if len(list) != 1 {
		t.Fatalf("duplicate plate created a second vehicle: %+v", list)
	}
	if list[0] != first {
		t.Fatalf("existing vehicle changed after rejected duplicate: got %+v, want %+v", list[0], first)
	}
}

// TestVehiclePlateInjectionDoesNotChangeQueryStructure covers AC-26: a plate
// carrying an SQL injection payload is stored and matched literally, and the
// customers table survives.
func TestVehiclePlateInjectionDoesNotChangeQueryStructure(t *testing.T) {
	srv, token := newAuthTestServer(t)
	customer := createCustomerForTest(t, srv, token)
	// Plates are stored upper-cased, so the payload is upper-case too; its shape
	// is unchanged and it must survive as a literal. The suffix keeps the unique
	// plate index from colliding with an earlier run against the same database.
	payload := fmt.Sprintf("'; DROP TABLE CUSTOMERS; -- %d", time.Now().UnixNano())

	var created vehicleBody
	doJSON(t, http.MethodPost, srv.URL+"/api/vehicles", token,
		map[string]any{
			"customer_id": customer.ID,
			"plate":       payload,
			"brand":       "Test",
			"model":       "Injection",
			"mileage_km":  0,
		}, &created, http.StatusCreated)
	if created.Plate != payload {
		t.Fatalf("payload was altered: got %q, want %q", created.Plate, payload)
	}

	// The customers table must still exist: the customer is still readable.
	var fetched customerBody
	doJSON(t, http.MethodGet, srv.URL+"/api/customers/"+customer.ID, token, nil, &fetched, http.StatusOK)
	if fetched != customer {
		t.Fatalf("customer changed after injection attempt: got %+v, want %+v", fetched, customer)
	}

	// The payload plate is a literal match, not a query fragment.
	list := vehiclesByPlate(t, srv, token, created.Plate)
	if len(list) != 1 || list[0].ID != created.ID {
		t.Fatalf("payload plate lookup wrong: got %+v, want [%+v]", list, created)
	}
}

// TestVehicleForUnknownCustomerRejected proves a vehicle for a non-existent
// customer is refused with a 4xx instead of a 5xx crash.
func TestVehicleForUnknownCustomerRejected(t *testing.T) {
	srv, token := newAuthTestServer(t)

	var body apiErrorBody
	doJSON(t, http.MethodPost, srv.URL+"/api/vehicles", token,
		map[string]any{
			"customer_id": "00000000-0000-0000-0000-000000000000",
			"plate":       uniquePlate(),
			"brand":       "VW",
			"model":       "Golf",
			"mileage_km":  0,
		}, &body, http.StatusBadRequest)
	if body.Error.Code != "bad_request" {
		t.Fatalf("unexpected error code: %q", body.Error.Code)
	}
}

// TestListVehiclesWithoutPlateReturnsArray proves the route answers a JSON array
// (never null) so the client can render it directly.
func TestListVehiclesWithoutPlateReturnsArray(t *testing.T) {
	srv, token := newAuthTestServer(t)

	var list []vehicleBody
	doJSON(t, http.MethodGet, srv.URL+"/api/vehicles", token, nil, &list, http.StatusOK)
	if list == nil {
		t.Fatal("expected a JSON array, got null")
	}
}
