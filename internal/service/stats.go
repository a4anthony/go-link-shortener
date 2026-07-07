package service

import (
	"context"
	"time"

	"github.com/google/uuid"

	"github.com/a4anthony/go-link-shortener/internal/domain"
)

// topN bounds how many rows each breakdown (referrers, countries, devices,
// busiest links) returns.
const topN = 10

// ClickStatsRepository is the analytics persistence the StatsService depends on.
type ClickStatsRepository interface {
	SeriesForLink(ctx context.Context, tenantID, linkID uuid.UUID, from, to time.Time, bucket domain.Bucket) ([]domain.TimePoint, error)
	TotalForLink(ctx context.Context, tenantID, linkID uuid.UUID, from, to time.Time) (int64, error)
	TopReferrers(ctx context.Context, tenantID, linkID uuid.UUID, from, to time.Time, limit int) ([]domain.LabelCount, error)
	TopCountries(ctx context.Context, tenantID, linkID uuid.UUID, from, to time.Time, limit int) ([]domain.LabelCount, error)
	DeviceBreakdown(ctx context.Context, tenantID, linkID uuid.UUID, from, to time.Time, limit int) ([]domain.LabelCount, error)
	TenantOverview(ctx context.Context, tenantID uuid.UUID, topN int) (domain.TenantOverview, error)
}

// LinkOwnershipChecker verifies a link belongs to a tenant before its stats are
// exposed.
type LinkOwnershipChecker interface {
	GetByIDForTenant(ctx context.Context, tenantID, id uuid.UUID) (*domain.Link, error)
}

// StatsService answers analytics queries, enforcing tenant ownership.
type StatsService struct {
	clicks ClickStatsRepository
	links  LinkOwnershipChecker
}

// NewStatsService builds a StatsService.
func NewStatsService(clicks ClickStatsRepository, links LinkOwnershipChecker) *StatsService {
	return &StatsService{clicks: clicks, links: links}
}

// LinkStats returns the analytics rollup for a link over [from, to). It returns
// domain.ErrNotFound if the link is not owned by the tenant, and
// domain.ErrValidation for a bad window or bucket.
func (s *StatsService) LinkStats(ctx context.Context, tenantID, linkID uuid.UUID, from, to time.Time, bucket domain.Bucket) (domain.LinkStats, error) {
	if !bucket.Valid() {
		return domain.LinkStats{}, domain.ErrValidation
	}
	if !to.After(from) {
		return domain.LinkStats{}, domain.ErrValidation
	}

	// Ownership check: a link outside the tenant is indistinguishable from missing.
	if _, err := s.links.GetByIDForTenant(ctx, tenantID, linkID); err != nil {
		return domain.LinkStats{}, err
	}

	series, err := s.clicks.SeriesForLink(ctx, tenantID, linkID, from, to, bucket)
	if err != nil {
		return domain.LinkStats{}, err
	}
	total, err := s.clicks.TotalForLink(ctx, tenantID, linkID, from, to)
	if err != nil {
		return domain.LinkStats{}, err
	}
	referrers, err := s.clicks.TopReferrers(ctx, tenantID, linkID, from, to, topN)
	if err != nil {
		return domain.LinkStats{}, err
	}
	countries, err := s.clicks.TopCountries(ctx, tenantID, linkID, from, to, topN)
	if err != nil {
		return domain.LinkStats{}, err
	}
	devices, err := s.clicks.DeviceBreakdown(ctx, tenantID, linkID, from, to, topN)
	if err != nil {
		return domain.LinkStats{}, err
	}

	return domain.LinkStats{
		LinkID:       linkID,
		From:         from,
		To:           to,
		Bucket:       bucket,
		TotalClicks:  total,
		Series:       series,
		TopReferrers: referrers,
		TopCountries: countries,
		Devices:      devices,
	}, nil
}

// Overview returns the tenant-wide analytics summary.
func (s *StatsService) Overview(ctx context.Context, tenantID uuid.UUID) (domain.TenantOverview, error) {
	return s.clicks.TenantOverview(ctx, tenantID, topN)
}
