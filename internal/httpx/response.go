// Package httpx holds HTTP helpers shared by the handler and middleware layers:
// the canonical JSON error envelope and helpers to write it. Keeping it in its
// own leaf package lets middleware emit errors without importing handler.
package httpx

import (
	"errors"

	"github.com/gin-gonic/gin"

	"github.com/a4anthony/go-link-shortener/internal/domain"
)

// Error codes used in the JSON envelope. Clients can branch on these stable
// strings rather than parsing messages.
const (
	CodeUnauthorized = "unauthorized"
	CodeForbidden    = "forbidden"
	CodeNotFound     = "not_found"
	CodeValidation   = "validation_error"
	CodeConflict     = "conflict"
	CodeRateLimited  = "rate_limited"
	CodeGone         = "gone"
	CodeInternal     = "internal_error"
)

// ErrorEnvelope is the top-level shape of every error response:
// {"error": {"code": "...", "message": "..."}}.
type ErrorEnvelope struct {
	Error ErrorBody `json:"error"`
}

// ErrorBody carries the machine-readable code and human-readable message.
type ErrorBody struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

// AbortError writes the error envelope with the given status/code/message and
// aborts the middleware chain so no further handlers run.
func AbortError(c *gin.Context, status int, code, message string) {
	c.AbortWithStatusJSON(status, ErrorEnvelope{Error: ErrorBody{Code: code, Message: message}})
}

// WriteError writes the error envelope without aborting (for terminal handlers).
func WriteError(c *gin.Context, status int, code, message string) {
	c.JSON(status, ErrorEnvelope{Error: ErrorBody{Code: code, Message: message}})
}

// StatusForError maps a domain sentinel error to an HTTP status and error code.
// Unknown errors map to 500/internal_error so we never leak internals.
func StatusForError(err error) (status int, code string) {
	switch {
	case errors.Is(err, domain.ErrNotFound):
		return 404, CodeNotFound
	case errors.Is(err, domain.ErrConflict):
		return 409, CodeConflict
	case errors.Is(err, domain.ErrValidation):
		return 400, CodeValidation
	case errors.Is(err, domain.ErrUnauthorized):
		return 401, CodeUnauthorized
	case errors.Is(err, domain.ErrGone):
		return 410, CodeGone
	default:
		return 500, CodeInternal
	}
}
