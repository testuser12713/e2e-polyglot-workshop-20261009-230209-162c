// Package server registers every contracted route on a net/http ServeMux and
// wraps it with the CORS and logging middleware. It is the routing table the
// API serves and the skeleton test exercises.
package server

import (
	"net/http"
	"time"

	"workshop/internal/deps"
	"workshop/internal/handlers"
	"workshop/internal/httpx"
	"workshop/internal/middleware"
)

// Per-client limits (AC-23, AC-25).
const (
	loginLimit  = 10
	publicLimit = 20
	limitWindow = time.Minute
)

// New builds the fully routed and wrapped HTTP handler.
func New(d *deps.Deps) http.Handler {
	mux := http.NewServeMux()

	withAuth := middleware.RequireAuth
	loginRate := middleware.RateLimit(loginLimit, limitWindow)
	publicRate := middleware.RateLimit(publicLimit, limitWindow)

	// Public.
	mux.HandleFunc("GET /healthz", handlers.Health)
	mux.Handle("POST /api/auth/login", loginRate(http.HandlerFunc(handlers.Login)))
	mux.HandleFunc("POST /api/orders", handlers.CreatePublicOrder)
	mux.Handle("GET /api/public/orders/{nr}", publicRate(http.HandlerFunc(handlers.PublicOrderStatus)))
	mux.Handle("GET /api/public/orders/{nr}/invoice", publicRate(http.HandlerFunc(handlers.PublicOrderInvoice)))

	// Bearer-protected workshop area.
	mux.Handle("POST /api/auth/logout", withAuth(http.HandlerFunc(handlers.Logout)))
	mux.Handle("POST /api/customers", withAuth(http.HandlerFunc(handlers.CreateCustomer)))
	mux.Handle("GET /api/customers/{id}", withAuth(http.HandlerFunc(handlers.GetCustomer)))
	mux.Handle("POST /api/vehicles", withAuth(http.HandlerFunc(handlers.CreateVehicle)))
	mux.Handle("GET /api/vehicles", withAuth(http.HandlerFunc(handlers.ListVehicles)))
	mux.Handle("GET /api/orders", withAuth(http.HandlerFunc(handlers.ListOrders)))
	mux.Handle("GET /api/orders/{nr}", withAuth(http.HandlerFunc(handlers.GetOrder)))
	mux.Handle("POST /api/orders/{nr}/status", withAuth(http.HandlerFunc(handlers.UpdateOrderStatus)))
	mux.Handle("POST /api/orders/{nr}/items", withAuth(http.HandlerFunc(handlers.CreateOrderItem)))
	mux.Handle("GET /api/reports/dashboard", withAuth(http.HandlerFunc(handlers.DashboardReport)))

	// Anything else answers with the same stable error envelope instead of the
	// mux's bare-text 404 (AC-18).
	mux.HandleFunc("/", notFound)

	return httpx.WithLogging(httpx.WithCORS(d.Cfg.WebOrigin, mux))
}

func notFound(w http.ResponseWriter, r *http.Request) {
	httpx.WriteError(w, http.StatusNotFound, httpx.CodeNotFound, "Nicht gefunden.")
}
