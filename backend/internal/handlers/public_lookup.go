package handlers

import "net/http"

// PublicOrderStatus handles GET /api/public/orders/{nr}.
func PublicOrderStatus(w http.ResponseWriter, r *http.Request) {
	notImplemented(w, "Die Statusauskunft")
}

// PublicOrderInvoice handles GET /api/public/orders/{nr}/invoice.
func PublicOrderInvoice(w http.ResponseWriter, r *http.Request) {
	notImplemented(w, "Der Rechnungsabruf")
}
