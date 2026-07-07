//go:build integration

package repository_test

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/a4anthony/go-link-shortener/internal/analytics"
	"github.com/a4anthony/go-link-shortener/internal/config"
	"github.com/a4anthony/go-link-shortener/internal/domain"
	"github.com/a4anthony/go-link-shortener/internal/repository"
	"github.com/a4anthony/go-link-shortener/internal/testutil"
)

func makeClick(linkID, tenantID uuid.UUID, at time.Time, ref, device, country string) domain.Click {
	return domain.Click{
		LinkID: linkID, TenantID: tenantID, OccurredAt: at,
		Referrer: ref, Device: device, Country: country, Browser: "Chrome", OS: "Linux",
	}
}

func TestClickRepository_RecordBatchAndStats(t *testing.T) {
	pool, _ := testutil.Postgres(t)
	ctx := context.Background()
	tenants := repository.NewTenantRepository(pool)
	links := repository.NewLinkRepository(pool)
	clicks := repository.NewClickRepository(pool)

	tenant, err := tenants.Create(ctx, "acme")
	require.NoError(t, err)
	link := &domain.Link{TenantID: tenant.ID, Code: "stat01", TargetURL: "https://x.com", RedirectType: 302}
	require.NoError(t, links.Create(ctx, link))

	base := time.Date(2026, 3, 1, 10, 0, 0, 0, time.UTC)
	batch := []domain.Click{
		makeClick(link.ID, tenant.ID, base, "https://google.com", "desktop", "US"),
		makeClick(link.ID, tenant.ID, base.Add(5*time.Minute), "https://google.com", "mobile", "US"),
		makeClick(link.ID, tenant.ID, base.Add(time.Hour), "", "desktop", "GB"),
	}
	exhausted, err := clicks.RecordBatch(ctx, batch)
	require.NoError(t, err)
	assert.Empty(t, exhausted)

	from := base.Add(-time.Hour)
	to := base.Add(24 * time.Hour)

	// click_count on the link was incremented.
	got, err := links.GetByIDForTenant(ctx, tenant.ID, link.ID)
	require.NoError(t, err)
	assert.Equal(t, int64(3), got.ClickCount)

	total, err := clicks.TotalForLink(ctx, tenant.ID, link.ID, from, to)
	require.NoError(t, err)
	assert.Equal(t, int64(3), total)

	// Two hourly buckets (10:00 has 2, 11:00 has 1).
	series, err := clicks.SeriesForLink(ctx, tenant.ID, link.ID, from, to, domain.BucketHour)
	require.NoError(t, err)
	require.Len(t, series, 2)
	assert.Equal(t, int64(2), series[0].Count)
	assert.Equal(t, int64(1), series[1].Count)

	// One daily bucket with all 3.
	daily, err := clicks.SeriesForLink(ctx, tenant.ID, link.ID, from, to, domain.BucketDay)
	require.NoError(t, err)
	require.Len(t, daily, 1)
	assert.Equal(t, int64(3), daily[0].Count)

	referrers, err := clicks.TopReferrers(ctx, tenant.ID, link.ID, from, to, 10)
	require.NoError(t, err)
	require.NotEmpty(t, referrers)
	assert.Equal(t, "https://google.com", referrers[0].Label)
	assert.Equal(t, int64(2), referrers[0].Count)
	// Empty referrer surfaces as "(direct)".
	assert.Contains(t, labelSet(referrers), "(direct)")

	countries, err := clicks.TopCountries(ctx, tenant.ID, link.ID, from, to, 10)
	require.NoError(t, err)
	assert.Equal(t, "US", countries[0].Label)

	devices, err := clicks.DeviceBreakdown(ctx, tenant.ID, link.ID, from, to, 10)
	require.NoError(t, err)
	assert.Equal(t, "desktop", devices[0].Label)
	assert.Equal(t, int64(2), devices[0].Count)

	overview, err := clicks.TenantOverview(ctx, tenant.ID, 10)
	require.NoError(t, err)
	assert.Equal(t, int64(1), overview.TotalLinks)
	assert.Equal(t, int64(3), overview.TotalClicks)
	require.Len(t, overview.TopLinks, 1)
	assert.Equal(t, "stat01", overview.TopLinks[0].Label)
}

func TestClickRepository_ReportsExhaustion(t *testing.T) {
	pool, _ := testutil.Postgres(t)
	ctx := context.Background()
	tenants := repository.NewTenantRepository(pool)
	links := repository.NewLinkRepository(pool)
	clicks := repository.NewClickRepository(pool)

	tenant, err := tenants.Create(ctx, "acme")
	require.NoError(t, err)
	max := int64(2)
	link := &domain.Link{TenantID: tenant.ID, Code: "cap001", TargetURL: "https://x.com", RedirectType: 302, MaxClicks: &max}
	require.NoError(t, links.Create(ctx, link))

	now := time.Now()
	exhausted, err := clicks.RecordBatch(ctx, []domain.Click{
		makeClick(link.ID, tenant.ID, now, "", "desktop", ""),
		makeClick(link.ID, tenant.ID, now, "", "desktop", ""),
	})
	require.NoError(t, err)
	assert.Equal(t, []string{"cap001"}, exhausted, "reaching max_clicks reports the code as exhausted")
}

// TestAnalyticsFlow_EndToEnd drives the full asynchronous path: events enqueued
// on the pipeline are enriched, batched, and persisted, then surface through the
// stats queries.
func TestAnalyticsFlow_EndToEnd(t *testing.T) {
	pool, _ := testutil.Postgres(t)
	ctx := context.Background()
	tenants := repository.NewTenantRepository(pool)
	links := repository.NewLinkRepository(pool)
	clicks := repository.NewClickRepository(pool)

	tenant, err := tenants.Create(ctx, "acme")
	require.NoError(t, err)
	link := &domain.Link{TenantID: tenant.ID, Code: "flow01", TargetURL: "https://x.com", RedirectType: 302}
	require.NoError(t, links.Create(ctx, link))

	pipeline := analytics.NewPipeline(config.AnalyticsConfig{
		BufferSize: 100, Workers: 2, BatchSize: 10, FlushInterval: 50 * time.Millisecond,
	}, clicks, nil, analytics.NoopResolver{}, "salt", nil, testutil.Logger())
	pipeline.Start()

	for i := 0; i < 15; i++ {
		ok := pipeline.Enqueue(analytics.ClickEvent{
			LinkID: link.ID, TenantID: tenant.ID, Code: "flow01",
			OccurredAt: time.Now(),
			UserAgent:  "Mozilla/5.0 (Windows NT 10.0) Chrome/120.0 Safari/537.36",
			IP:         "203.0.113.7",
		})
		require.True(t, ok)
	}

	drainCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	require.NoError(t, pipeline.Shutdown(drainCtx))

	got, err := links.GetByIDForTenant(ctx, tenant.ID, link.ID)
	require.NoError(t, err)
	assert.Equal(t, int64(15), got.ClickCount)

	overview, err := clicks.TenantOverview(ctx, tenant.ID, 10)
	require.NoError(t, err)
	assert.Equal(t, int64(15), overview.TotalClicks)
}

func labelSet(lcs []domain.LabelCount) map[string]struct{} {
	out := make(map[string]struct{}, len(lcs))
	for _, lc := range lcs {
		out[lc.Label] = struct{}{}
	}
	return out
}
