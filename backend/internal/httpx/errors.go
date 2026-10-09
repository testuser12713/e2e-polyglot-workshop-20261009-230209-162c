package httpx

import "net/http"

// Stable error codes. Every non-2xx response carries exactly one of these so the
// web app can react to a code without parsing a message.
const (
	CodeBadRequest         = "bad_request"
	CodeUnauthorized       = "unauthorized"
	CodeInvalidCredentials = "invalid_credentials"
	CodeNotFound           = "not_found"
	CodeConflict           = "conflict"
	CodePlateTaken         = "plate_taken"
	CodeInvalidTransition  = "invalid_transition"
	CodeTooManyRequests    = "too_many_requests"
	CodeNotImplemented     = "not_implemented"
	CodeInternal           = "internal_error"
	CodeServiceUnavailable = "service_unavailable"
)

// APIError is the readable half of the error envelope.
type APIError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

// errorEnvelope is the single shape of every non-2xx response:
// {"error":{"code":"<stable>","message":"<German text>"}}.
type errorEnvelope struct {
	Error APIError `json:"error"`
}

// WriteError writes the stable error envelope. The message is a readable German
// sentence for the user and never contains personal data or a secret.
func WriteError(w http.ResponseWriter, status int, code, message string) {
	WriteJSON(w, status, errorEnvelope{Error: APIError{Code: code, Message: message}})
}
