//go:build integration

package testutil

import (
	"io"
	"log/slog"
)

// Logger returns a no-op structured logger for use in integration tests.
func Logger() *slog.Logger {
	return slog.New(slog.NewJSONHandler(io.Discard, nil))
}
