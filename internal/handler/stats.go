package handler

import (
	"context"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/a4anthony/go-link-shortener/internal/domain"
	"github.com/a4anthony/go-link-shortener/internal/middleware"
)

// defaultStatsWindow is used when the caller omits from/to.
const defaultStatsWindow = 7 * 24 * time.Hour

// StatsService is the analytics behaviour the handler needs.
type StatsService interface {
	LinkStats(ctx context.Context, tenantID, linkID uuid.UUID, from, to time.Time, bucket domain.Bucket) (domain.LinkStats, error)
	Overview(ctx context.Context, tenantID uuid.UUID) (domain.TenantOverview, error)
}

// StatsHandler serves the analytics endpoints.
type StatsHandler struct {
	svc StatsService
	now func() time.Time
}

// NewStatsHandler builds a StatsHandler.
func NewStatsHandler(svc StatsService) *StatsHandler {
	return &StatsHandler{svc: svc, now: time.Now}
}

// Register mounts the analytics routes on the authenticated group.
func (h *StatsHandler) Register(g *gin.RouterGroup) {
	g.GET("/links/:id/stats", h.LinkStats)
	g.GET("/stats/overview", h.Overview)
}

// LinkStats handles GET /api/v1/links/:id/stats?from=&to=&bucket=hour|day.
func (h *StatsHandler) LinkStats(c *gin.Context) {
	id, ok := parseID(c)
	if !ok {
		return
	}

	to, ok := parseTimeQuery(c, "to", h.now())
	if !ok {
		respondValidation(c, "to must be an RFC3339 timestamp")
		return
	}
	from, ok := parseTimeQuery(c, "from", to.Add(-defaultStatsWindow))
	if !ok {
		respondValidation(c, "from must be an RFC3339 timestamp")
		return
	}

	bucket := domain.Bucket(c.DefaultQuery("bucket", string(domain.BucketHour)))
	if !bucket.Valid() {
		respondValidation(c, "bucket must be 'hour' or 'day'")
		return
	}
	if !to.After(from) {
		respondValidation(c, "'to' must be after 'from'")
		return
	}

	stats, err := h.svc.LinkStats(c.Request.Context(), middleware.MustTenantID(c), id, from, to, bucket)
	if err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, stats)
}

// Overview handles GET /api/v1/stats/overview.
func (h *StatsHandler) Overview(c *gin.Context) {
	overview, err := h.svc.Overview(c.Request.Context(), middleware.MustTenantID(c))
	if err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, overview)
}

// parseTimeQuery parses an RFC3339 query param, returning def if absent. The bool
// is false only when the param is present but unparseable.
func parseTimeQuery(c *gin.Context, key string, def time.Time) (time.Time, bool) {
	raw := c.Query(key)
	if raw == "" {
		return def, true
	}
	t, err := time.Parse(time.RFC3339, raw)
	if err != nil {
		return time.Time{}, false
	}
	return t, true
}
