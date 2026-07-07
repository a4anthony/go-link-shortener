package service

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/a4anthony/go-link-shortener/internal/domain"
)

type fakeStatsRepo struct {
	series    []domain.TimePoint
	total     int64
	referrers []domain.LabelCount
	countries []domain.LabelCount
	devices   []domain.LabelCount
	overview  domain.TenantOverview
}

func (f *fakeStatsRepo) SeriesForLink(context.Context, uuid.UUID, uuid.UUID, time.Time, time.Time, domain.Bucket) ([]domain.TimePoint, error) {
	return f.series, nil
}
func (f *fakeStatsRepo) TotalForLink(context.Context, uuid.UUID, uuid.UUID, time.Time, time.Time) (int64, error) {
	return f.total, nil
}
func (f *fakeStatsRepo) TopReferrers(context.Context, uuid.UUID, uuid.UUID, time.Time, time.Time, int) ([]domain.LabelCount, error) {
	return f.referrers, nil
}
func (f *fakeStatsRepo) TopCountries(context.Context, uuid.UUID, uuid.UUID, time.Time, time.Time, int) ([]domain.LabelCount, error) {
	return f.countries, nil
}
func (f *fakeStatsRepo) DeviceBreakdown(context.Context, uuid.UUID, uuid.UUID, time.Time, time.Time, int) ([]domain.LabelCount, error) {
	return f.devices, nil
}
func (f *fakeStatsRepo) TenantOverview(context.Context, uuid.UUID, int) (domain.TenantOverview, error) {
	return f.overview, nil
}

type fakeOwner struct {
	err error
}

func (f fakeOwner) GetByIDForTenant(_ context.Context, _, id uuid.UUID) (*domain.Link, error) {
	if f.err != nil {
		return nil, f.err
	}
	return &domain.Link{ID: id}, nil
}

func TestStatsService_LinkStats(t *testing.T) {
	repo := &fakeStatsRepo{
		series:    []domain.TimePoint{{Time: time.Now(), Count: 3}},
		total:     3,
		referrers: []domain.LabelCount{{Label: "(direct)", Count: 3}},
		countries: []domain.LabelCount{{Label: "US", Count: 2}},
		devices:   []domain.LabelCount{{Label: "mobile", Count: 3}},
	}
	svc := NewStatsService(repo, fakeOwner{})

	from := time.Now().Add(-24 * time.Hour)
	to := time.Now()
	stats, err := svc.LinkStats(context.Background(), uuid.New(), uuid.New(), from, to, domain.BucketHour)
	require.NoError(t, err)
	assert.Equal(t, int64(3), stats.TotalClicks)
	assert.Len(t, stats.Series, 1)
	assert.Equal(t, "(direct)", stats.TopReferrers[0].Label)
	assert.Equal(t, "US", stats.TopCountries[0].Label)
	assert.Equal(t, "mobile", stats.Devices[0].Label)
}

func TestStatsService_LinkStats_Validation(t *testing.T) {
	svc := NewStatsService(&fakeStatsRepo{}, fakeOwner{})
	now := time.Now()

	_, err := svc.LinkStats(context.Background(), uuid.New(), uuid.New(), now.Add(-time.Hour), now, domain.Bucket("week"))
	assert.ErrorIs(t, err, domain.ErrValidation, "bad bucket")

	_, err = svc.LinkStats(context.Background(), uuid.New(), uuid.New(), now, now, domain.BucketHour)
	assert.ErrorIs(t, err, domain.ErrValidation, "empty window")
}

func TestStatsService_LinkStats_NotOwned(t *testing.T) {
	svc := NewStatsService(&fakeStatsRepo{}, fakeOwner{err: domain.ErrNotFound})
	from := time.Now().Add(-time.Hour)
	to := time.Now()
	_, err := svc.LinkStats(context.Background(), uuid.New(), uuid.New(), from, to, domain.BucketHour)
	assert.ErrorIs(t, err, domain.ErrNotFound)
}

func TestStatsService_Overview(t *testing.T) {
	repo := &fakeStatsRepo{overview: domain.TenantOverview{TotalLinks: 4, TotalClicks: 99}}
	svc := NewStatsService(repo, fakeOwner{})
	ov, err := svc.Overview(context.Background(), uuid.New())
	require.NoError(t, err)
	assert.Equal(t, int64(4), ov.TotalLinks)
	assert.Equal(t, int64(99), ov.TotalClicks)
}
