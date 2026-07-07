package middleware

import (
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRequestID_GeneratesAndExposes(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(RequestID())
	var seen string
	r.GET("/x", func(c *gin.Context) {
		seen = GetRequestID(c)
		c.Status(http.StatusOK)
	})

	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/x", nil))

	assert.NotEmpty(t, seen, "request id set on context")
	assert.Equal(t, seen, w.Header().Get("X-Request-ID"), "request id echoed in response")
}

func TestRequestID_HonoursInbound(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(RequestID())
	r.GET("/x", func(c *gin.Context) { c.Status(http.StatusOK) })

	req := httptest.NewRequest(http.MethodGet, "/x", nil)
	req.Header.Set("X-Request-ID", "trace-123")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, "trace-123", w.Header().Get("X-Request-ID"))
}

type capturingObserver struct {
	mu     sync.Mutex
	method string
	route  string
	status string
	calls  int
}

func (o *capturingObserver) ObserveHTTP(method, route, status string, _ time.Duration) {
	o.mu.Lock()
	o.method, o.route, o.status, o.calls = method, route, status, o.calls+1
	o.mu.Unlock()
}

func TestMetrics_ObservesRouteTemplate(t *testing.T) {
	gin.SetMode(gin.TestMode)
	obs := &capturingObserver{}
	r := gin.New()
	r.Use(Metrics(obs))
	r.GET("/links/:id", func(c *gin.Context) { c.Status(http.StatusOK) })

	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/links/abc", nil))

	require.Equal(t, 1, obs.calls)
	assert.Equal(t, http.MethodGet, obs.method)
	assert.Equal(t, "/links/:id", obs.route, "uses route template, not raw path")
	assert.Equal(t, "200", obs.status)
}

func TestMetrics_UnmatchedRoute(t *testing.T) {
	gin.SetMode(gin.TestMode)
	obs := &capturingObserver{}
	r := gin.New()
	r.Use(Metrics(obs))
	// No routes registered; any path is unmatched.
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/nope", nil))

	assert.Equal(t, "unmatched", obs.route)
}
