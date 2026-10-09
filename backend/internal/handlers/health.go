// Package handlers holds one handler file per capability. Routes whose owning
// ticket has not landed yet answer 501 with the standard error envelope (never
// 500), so the rest of the product can talk to a live contract.
package handlers

import (
	"net/http"

	"workshop/internal/deps"
	"workshop/internal/httpx"
)

// notImplemented is the honest answer of a route whose feature is still owned by
// another, not-yet-merged ticket.
func notImplemented(w http.ResponseWriter, feature string) {
	httpx.WriteError(w, http.StatusNotImplemented, httpx.CodeNotImplemented,
		feature+" ist noch nicht implementiert.")
}

// Health is the real health check: it exercises the same database the real
// routes use, so a bound port with a dead database is not reported as healthy.
func Health(w http.ResponseWriter, r *http.Request) {
	pool := deps.DB()
	if pool == nil || pool.Ping(r.Context()) != nil {
		httpx.WriteError(w, http.StatusServiceUnavailable, httpx.CodeServiceUnavailable,
			"Datenbank nicht erreichbar.")
		return
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}
