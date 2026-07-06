package handler

import (
	"context"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
)

// Checker reports the health of a single dependency. It should return quickly
// and respect the provided context deadline.
type Checker func(ctx context.Context) error

// HealthHandler serves liveness and readiness probes. Liveness is process-local
// (are we running?); readiness runs the registered dependency checkers (can we
// serve traffic?).
type HealthHandler struct {
	checkers map[string]Checker
	timeout  time.Duration
}

// NewHealthHandler builds a health handler. checkers maps a dependency name
// (e.g. "postgres", "redis") to its probe.
func NewHealthHandler(checkers map[string]Checker) *HealthHandler {
	return &HealthHandler{
		checkers: checkers,
		timeout:  2 * time.Second,
	}
}

// Live handles GET /healthz. It always returns 200 while the process is up.
func (h *HealthHandler) Live(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"status": "ok"})
}

// Ready handles GET /readyz. It returns 200 only if every dependency checker
// passes, otherwise 503 with a per-dependency breakdown.
func (h *HealthHandler) Ready(c *gin.Context) {
	ctx, cancel := context.WithTimeout(c.Request.Context(), h.timeout)
	defer cancel()

	results := make(map[string]string, len(h.checkers))
	healthy := true
	for name, check := range h.checkers {
		if err := check(ctx); err != nil {
			results[name] = "error: " + err.Error()
			healthy = false
		} else {
			results[name] = "ok"
		}
	}

	status := http.StatusOK
	overall := "ok"
	if !healthy {
		status = http.StatusServiceUnavailable
		overall = "unavailable"
	}
	c.JSON(status, gin.H{"status": overall, "checks": results})
}
