package handler

import (
	"errors"

	"github.com/gin-gonic/gin"

	"github.com/a4anthony/go-link-shortener/internal/domain"
	"github.com/a4anthony/go-link-shortener/internal/httpx"
)

// respondError maps a domain/service error to the canonical JSON envelope. For
// validation errors it surfaces the underlying message; for everything else it
// uses a generic message so internals never leak.
func respondError(c *gin.Context, err error) {
	status, code := httpx.StatusForError(err)
	message := genericMessage(status)
	if errors.Is(err, domain.ErrValidation) {
		message = err.Error()
	}
	httpx.WriteError(c, status, code, message)
}

// respondValidation writes a 400 validation error with the given message.
func respondValidation(c *gin.Context, message string) {
	httpx.WriteError(c, 400, httpx.CodeValidation, message)
}

func genericMessage(status int) string {
	switch status {
	case 400:
		return "invalid request"
	case 401:
		return "unauthorized"
	case 403:
		return "forbidden"
	case 404:
		return "resource not found"
	case 409:
		return "resource already exists"
	case 410:
		return "resource is no longer available"
	default:
		return "internal server error"
	}
}
