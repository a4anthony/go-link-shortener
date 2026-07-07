package middleware

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
)

type fakeLimiter struct {
	allowed    bool
	remaining  int
	retryAfter time.Duration
	err        error
	calls      int
}

func (f *fakeLimiter) Allow(context.Context, string, int, time.Duration) (bool, int, time.Duration, error) {
	f.calls++
	return f.allowed, f.remaining, f.retryAfter, f.err
}

func discardLogger() *slog.Logger { return slog.New(slog.NewJSONHandler(io.Discard, nil)) }

func setupRateRouter(l RateLimiter, withTenant bool) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	if withTenant {
		r.Use(func(c *gin.Context) { c.Set(ctxKeyTenantID, uuid.New()); c.Next() })
	}
	r.Use(RateLimit(l, 100, time.Minute, discardLogger()))
	r.GET("/x", func(c *gin.Context) { c.String(http.StatusOK, "ok") })
	return r
}

func do(r http.Handler) *httptest.ResponseRecorder {
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/x", nil))
	return w
}

func TestRateLimit_Allowed(t *testing.T) {
	l := &fakeLimiter{allowed: true, remaining: 99}
	w := do(setupRateRouter(l, true))
	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "100", w.Header().Get("X-RateLimit-Limit"))
	assert.Equal(t, "99", w.Header().Get("X-RateLimit-Remaining"))
}

func TestRateLimit_Denied(t *testing.T) {
	l := &fakeLimiter{allowed: false, retryAfter: 3 * time.Second}
	w := do(setupRateRouter(l, true))
	assert.Equal(t, http.StatusTooManyRequests, w.Code)
	assert.Equal(t, "3", w.Header().Get("Retry-After"))
	assert.Contains(t, w.Body.String(), "rate_limited")
}

func TestRateLimit_DeniedRoundsUpRetryAfter(t *testing.T) {
	l := &fakeLimiter{allowed: false, retryAfter: 200 * time.Millisecond}
	w := do(setupRateRouter(l, true))
	assert.Equal(t, http.StatusTooManyRequests, w.Code)
	assert.Equal(t, "1", w.Header().Get("Retry-After"), "sub-second retry rounds up to 1s")
}

func TestRateLimit_FailsOpenOnError(t *testing.T) {
	l := &fakeLimiter{err: errors.New("redis down")}
	w := do(setupRateRouter(l, true))
	assert.Equal(t, http.StatusOK, w.Code, "limiter errors must not take the API down")
}

func TestRateLimit_NoTenantSkips(t *testing.T) {
	l := &fakeLimiter{allowed: false}
	w := do(setupRateRouter(l, false))
	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, 0, l.calls, "without a tenant the limiter is not consulted")
}
