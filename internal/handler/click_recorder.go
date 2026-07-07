package handler

import (
	"time"

	"github.com/gin-gonic/gin"

	"github.com/a4anthony/go-link-shortener/internal/analytics"
	"github.com/a4anthony/go-link-shortener/internal/domain"
)

// ClickEnqueuer accepts raw click events for asynchronous processing. The
// analytics pipeline satisfies it. Enqueue must never block.
type ClickEnqueuer interface {
	Enqueue(e analytics.ClickEvent) bool
}

// PipelineClickRecorder captures a redirect as a raw ClickEvent and hands it to
// the analytics pipeline. It reads only cheap request fields; all enrichment
// happens off the hot path in the pipeline workers.
type PipelineClickRecorder struct {
	enqueuer ClickEnqueuer
	now      func() time.Time
}

// NewPipelineClickRecorder builds a PipelineClickRecorder.
func NewPipelineClickRecorder(enqueuer ClickEnqueuer) *PipelineClickRecorder {
	return &PipelineClickRecorder{enqueuer: enqueuer, now: time.Now}
}

// Record enqueues a click for the served redirect. A dropped event (full buffer)
// is intentionally ignored here — the pipeline counts drops for metrics.
func (r *PipelineClickRecorder) Record(c *gin.Context, link *domain.Link) {
	r.enqueuer.Enqueue(analytics.ClickEvent{
		LinkID:     link.ID,
		TenantID:   link.TenantID,
		Code:       link.Code,
		OccurredAt: r.now(),
		Referrer:   c.Request.Referer(),
		UserAgent:  c.Request.UserAgent(),
		IP:         c.ClientIP(),
	})
}
