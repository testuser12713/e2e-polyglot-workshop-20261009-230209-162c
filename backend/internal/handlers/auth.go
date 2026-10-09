package handlers

import "net/http"

// Login handles POST /api/auth/login. It is owned by the workshop-login ticket.
func Login(w http.ResponseWriter, r *http.Request) {
	notImplemented(w, "Die Anmeldung")
}

// Logout handles POST /api/auth/logout. It is owned by the workshop-login ticket.
func Logout(w http.ResponseWriter, r *http.Request) {
	notImplemented(w, "Die Abmeldung")
}
