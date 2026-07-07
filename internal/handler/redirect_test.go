package handler

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/a4anthony/go-link-shortener/internal/domain"
)

type fakeResolver struct {
	link *domain.Link
	err  error
}

func (f *fakeResolver) Resolve(_ context.Context, _ string) (*domain.Link, error) {
	return f.link, f.err
}

type recordingClicks struct {
	mu    sync.Mutex
	count int
}

func (r *recordingClicks) Record(_ *gin.Context, _ *domain.Link) {
	r.mu.Lock()
	r.count++
	r.mu.Unlock()
}

func (r *recordingClicks) calls() int {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.count
}

func setupRedirectRouter(svc RedirectResolver, clicks ClickRecorder) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	// Register static routes alongside the wildcard to assert they coexist.
	r.GET("/healthz", func(c *gin.Context) { c.String(http.StatusOK, "ok") })
	r.GET("/metrics", func(c *gin.Context) { c.String(http.StatusOK, "metrics") })
	NewRedirectHandler(svc, clicks, nil).Register(r)
	return r
}

func TestRedirectHandler_Found(t *testing.T) {
	link := &domain.Link{Code: "abc1234", TargetURL: "https://example.com", RedirectType: domain.RedirectFound}
	clicks := &recordingClicks{}
	r := setupRedirectRouter(&fakeResolver{link: link}, clicks)

	req := httptest.NewRequest(http.MethodGet, "/abc1234", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusFound, w.Code)
	assert.Equal(t, "https://example.com", w.Header().Get("Location"))
	assert.Equal(t, 1, clicks.calls(), "a served redirect should record a click")
}

func TestRedirectHandler_Permanent(t *testing.T) {
	link := &domain.Link{Code: "perm", TargetURL: "https://example.org", RedirectType: domain.RedirectPermanent}
	r := setupRedirectRouter(&fakeResolver{link: link}, nil)

	req := httptest.NewRequest(http.MethodGet, "/perm", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusMovedPermanently, w.Code)
	assert.Equal(t, "https://example.org", w.Header().Get("Location"))
}

func TestRedirectHandler_NotFound(t *testing.T) {
	r := setupRedirectRouter(&fakeResolver{err: domain.ErrNotFound}, nil)
	req := httptest.NewRequest(http.MethodGet, "/missing", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code)
	assert.Contains(t, w.Body.String(), "not_found")
}

func TestRedirectHandler_Gone(t *testing.T) {
	r := setupRedirectRouter(&fakeResolver{err: domain.ErrGone}, nil)
	req := httptest.NewRequest(http.MethodGet, "/expired", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusGone, w.Code)
	assert.Contains(t, w.Body.String(), "gone")
}

func TestRedirectHandler_StaticRoutesStillWork(t *testing.T) {
	r := setupRedirectRouter(&fakeResolver{err: domain.ErrNotFound}, nil)
	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	require.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "ok", w.Body.String())
}
