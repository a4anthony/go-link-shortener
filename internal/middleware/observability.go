package middleware

import (
	"log/slog"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// HTTPObserver records per-request latency and outcome. metrics.Metrics
// satisfies it.
type HTTPObserver interface {
	ObserveHTTP(method, route, status string, dur time.Duration)
}

// RequestID assigns each request a stable id (honouring an inbound
// X-Request-ID), exposes it on the context and response header, so logs and
// clients can correlate a request end to end.
func RequestID() gin.HandlerFunc {
	return func(c *gin.Context) {
		id := c.GetHeader("X-Request-ID")
		if id == "" {
			id = uuid.NewString()
		}
		c.Set(ctxKeyRequestID, id)
		c.Header("X-Request-ID", id)
		c.Next()
	}
}

// RequestLogger logs one structured line per request after it completes, at a
// level chosen by status class.
func RequestLogger(log *slog.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		c.Next()

		status := c.Writer.Status()
		attrs := []any{
			"method", c.Request.Method,
			"path", c.Request.URL.Path,
			"status", status,
			"latency_ms", time.Since(start).Milliseconds(),
			"bytes", c.Writer.Size(),
			"client_ip", c.ClientIP(),
			"request_id", GetRequestID(c),
		}
		if tid, ok := TenantID(c); ok {
			attrs = append(attrs, "tenant_id", tid.String())
		}

		switch {
		case status >= 500:
			log.Error("request", attrs...)
		case status >= 400:
			log.Warn("request", attrs...)
		default:
			log.Info("request", attrs...)
		}
	}
}

// Metrics records request latency and outcome to the observer, using the matched
// route template (not the raw path) to keep label cardinality bounded.
func Metrics(obs HTTPObserver) gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		c.Next()

		route := c.FullPath()
		if route == "" {
			route = "unmatched"
		}
		obs.ObserveHTTP(c.Request.Method, route, strconv.Itoa(c.Writer.Status()), time.Since(start))
	}
}
