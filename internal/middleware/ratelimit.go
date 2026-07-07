package middleware

import (
	"context"
	"log/slog"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/a4anthony/go-link-shortener/internal/httpx"
)

// RateLimiter decides whether a request keyed by key is permitted within limit
// requests per window. repository.RedisRateLimiter satisfies it.
type RateLimiter interface {
	Allow(ctx context.Context, key string, limit int, window time.Duration) (allowed bool, remaining int, retryAfter time.Duration, err error)
}

// RateLimit returns middleware that enforces a per-tenant sliding-window limit.
// It must run after authentication so a tenant is on the context. On a limiter
// backend error it fails open (allows the request) so Redis trouble never takes
// the API down.
func RateLimit(limiter RateLimiter, limit int, window time.Duration, log *slog.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		tenantID, ok := TenantID(c)
		if !ok {
			// No tenant (unauthenticated route slipped through); nothing to key on.
			c.Next()
			return
		}

		allowed, remaining, retryAfter, err := limiter.Allow(c.Request.Context(), tenantID.String(), limit, window)
		if err != nil {
			log.Warn("rate limiter backend error; failing open", "error", err, "tenant_id", tenantID)
			c.Next()
			return
		}

		c.Header("X-RateLimit-Limit", strconv.Itoa(limit))
		c.Header("X-RateLimit-Remaining", strconv.Itoa(remaining))

		if !allowed {
			retrySecs := int(retryAfter.Seconds())
			if retrySecs < 1 {
				retrySecs = 1
			}
			c.Header("Retry-After", strconv.Itoa(retrySecs))
			httpx.AbortError(c, 429, httpx.CodeRateLimited, "rate limit exceeded")
			return
		}
		c.Next()
	}
}
