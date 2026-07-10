package service

import (
	"context"
	"log/slog"
	"time"

	"github.com/google/uuid"
)

// ExpiredLinkPurger hard-deletes a tenant's dead links. The concrete
// repository.LinkRepository satisfies it structurally.
type ExpiredLinkPurger interface {
	PurgeExpired(ctx context.Context, tenantID uuid.UUID, cutoff time.Time) (int64, error)
}

// LinkJanitor periodically hard-deletes the demo tenant's expired and
// soft-deleted links (clicks go with them via ON DELETE CASCADE), so the public
// playground cannot accumulate junk or abuse links indefinitely.
type LinkJanitor struct {
	purger    ExpiredLinkPurger
	tenantID  uuid.UUID
	interval  time.Duration
	retention time.Duration
	logger    *slog.Logger
	now       func() time.Time
}

// NewLinkJanitor builds a janitor sweeping tenantID every interval, purging
// links that have been expired or soft-deleted for longer than retention.
func NewLinkJanitor(purger ExpiredLinkPurger, tenantID uuid.UUID, interval, retention time.Duration, log *slog.Logger) *LinkJanitor {
	return &LinkJanitor{
		purger:    purger,
		tenantID:  tenantID,
		interval:  interval,
		retention: retention,
		logger:    log,
		now:       time.Now,
	}
}

// Run sweeps immediately, then on every tick until ctx is cancelled. Sweep
// failures are logged and retried on the next tick.
func (j *LinkJanitor) Run(ctx context.Context) {
	j.sweep(ctx)
	ticker := time.NewTicker(j.interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			j.sweep(ctx)
		}
	}
}

func (j *LinkJanitor) sweep(ctx context.Context) {
	cutoff := j.now().Add(-j.retention)
	purged, err := j.purger.PurgeExpired(ctx, j.tenantID, cutoff)
	if err != nil {
		if ctx.Err() == nil {
			j.logger.Warn("demo link purge failed", "error", err)
		}
		return
	}
	if purged > 0 {
		j.logger.Info("purged dead demo links", "count", purged, "cutoff", cutoff)
	}
}
