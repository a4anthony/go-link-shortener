package domain

import "errors"

// Sentinel errors returned by the service/repository layers. Handlers map these
// to HTTP status codes and stable error codes in the JSON envelope.
var (
	// ErrNotFound is returned when a resource does not exist (or is not visible
	// to the requesting tenant — the two are indistinguishable by design).
	ErrNotFound = errors.New("resource not found")

	// ErrConflict is returned on a uniqueness violation (e.g. duplicate alias).
	ErrConflict = errors.New("resource conflict")

	// ErrValidation is returned when input fails business-rule validation.
	ErrValidation = errors.New("validation failed")

	// ErrUnauthorized is returned when authentication is missing or invalid.
	ErrUnauthorized = errors.New("unauthorized")

	// ErrGone is returned when a link is expired or has exhausted its click
	// limit; the redirect path maps this to 410 Gone.
	ErrGone = errors.New("resource gone")

	// ErrCodeExhausted is returned when the shortcode generator cannot find a
	// free code within its retry budget.
	ErrCodeExhausted = errors.New("could not allocate unique shortcode")
)
