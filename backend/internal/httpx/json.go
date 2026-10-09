// Package httpx holds the API's small HTTP building blocks: JSON read/write,
// the stable error envelope and the CORS/logging middleware.
package httpx

import (
	"encoding/json"
	"errors"
	"io"
	"log"
	"net/http"
)

// maxBodyBytes caps how much of a request body the API will read.
const maxBodyBytes = 1 << 20 // 1 MiB

// WriteJSON writes v as an application/json response with the given status.
func WriteJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	if v == nil {
		return
	}
	if err := json.NewEncoder(w).Encode(v); err != nil {
		// The status line is already on the wire, so the response cannot be
		// repaired; report it for operators without re-writing the header.
		log.Printf("httpx: encode response: %v", err)
	}
}

// DecodeJSON reads exactly one JSON value from the request body into dst. The
// body is size-limited and unknown fields are rejected so a client typo cannot
// silently disappear.
func DecodeJSON(r *http.Request, dst any) error {
	if r.Body == nil {
		return errors.New("leerer Anfragekörper")
	}
	dec := json.NewDecoder(io.LimitReader(r.Body, maxBodyBytes))
	dec.DisallowUnknownFields()
	if err := dec.Decode(dst); err != nil {
		return err
	}
	// Reject trailing data after the first JSON value.
	if dec.More() {
		return errors.New("unerwartete Daten nach dem JSON-Objekt")
	}
	return nil
}
