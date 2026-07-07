package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/a4anthony/go-link-shortener/internal/domain"
)

type fakeStatsService struct {
	stats     domain.LinkStats
	statsErr  error
	overview  domain.TenantOverview
	lastFrom  time.Time
	lastTo    time.Time
	lastBucke domain.Bucket
}

func (f *fakeStatsService) LinkStats(_ context.Context, _, _ uuid.UUID, from, to time.Time, bucket domain.Bucket) (domain.LinkStats, error) {
	f.lastFrom, f.lastTo, f.lastBucke = from, to, bucket
	return f.stats, f.statsErr
}
func (f *fakeStatsService) Overview(context.Context, uuid.UUID) (domain.TenantOverview, error) {
	return f.overview, nil
}

func setupStatsRouter(svc StatsService) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(func(c *gin.Context) { c.Set("tenant_id", uuid.New()); c.Next() })
	NewStatsHandler(svc).Register(r.Group("/api/v1"))
	return r
}

func TestStatsHandler_LinkStats(t *testing.T) {
	svc := &fakeStatsService{stats: domain.LinkStats{TotalClicks: 7, Bucket: domain.BucketDay}}
	r := setupStatsRouter(svc)

	url := "/api/v1/links/" + uuid.New().String() + "/stats?bucket=day&from=2026-01-01T00:00:00Z&to=2026-01-08T00:00:00Z"
	w := doJSON(r, http.MethodGet, url, nil)
	require.Equal(t, http.StatusOK, w.Code)

	var resp domain.LinkStats
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.Equal(t, int64(7), resp.TotalClicks)
	assert.Equal(t, domain.BucketDay, svc.lastBucke)
	assert.Equal(t, 2026, svc.lastFrom.Year())
}

func TestStatsHandler_LinkStats_DefaultsWindow(t *testing.T) {
	svc := &fakeStatsService{}
	r := setupStatsRouter(svc)
	w := doJSON(r, http.MethodGet, "/api/v1/links/"+uuid.New().String()+"/stats", nil)
	require.Equal(t, http.StatusOK, w.Code)
	// Defaults: hour bucket, ~7 day window.
	assert.Equal(t, domain.BucketHour, svc.lastBucke)
	assert.WithinDuration(t, svc.lastTo.Add(-7*24*time.Hour), svc.lastFrom, time.Minute)
}

func TestStatsHandler_LinkStats_BadBucket(t *testing.T) {
	r := setupStatsRouter(&fakeStatsService{})
	w := doJSON(r, http.MethodGet, "/api/v1/links/"+uuid.New().String()+"/stats?bucket=year", nil)
	assert.Equal(t, http.StatusBadRequest, w.Code)
	assert.Contains(t, w.Body.String(), "validation_error")
}

func TestStatsHandler_LinkStats_BadTime(t *testing.T) {
	r := setupStatsRouter(&fakeStatsService{})
	w := doJSON(r, http.MethodGet, "/api/v1/links/"+uuid.New().String()+"/stats?from=nonsense", nil)
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestStatsHandler_LinkStats_NotFound(t *testing.T) {
	r := setupStatsRouter(&fakeStatsService{statsErr: domain.ErrNotFound})
	w := doJSON(r, http.MethodGet, "/api/v1/links/"+uuid.New().String()+"/stats", nil)
	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestStatsHandler_Overview(t *testing.T) {
	svc := &fakeStatsService{overview: domain.TenantOverview{TotalLinks: 3, TotalClicks: 42}}
	r := setupStatsRouter(svc)
	w := doJSON(r, http.MethodGet, "/api/v1/stats/overview", nil)
	require.Equal(t, http.StatusOK, w.Code)

	var resp domain.TenantOverview
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.Equal(t, int64(3), resp.TotalLinks)
	assert.Equal(t, int64(42), resp.TotalClicks)
}
