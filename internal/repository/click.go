package repository

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/a4anthony/go-link-shortener/internal/domain"
)

// ClickRepository persists click events and answers analytics queries. It holds
// the pool directly because batch inserts use CopyFrom inside a transaction.
type ClickRepository struct {
	pool *pgxpool.Pool
}

// NewClickRepository builds a ClickRepository.
func NewClickRepository(pool *pgxpool.Pool) *ClickRepository {
	return &ClickRepository{pool: pool}
}

var clickCopyColumns = []string{
	"link_id", "tenant_id", "occurred_at", "referrer",
	"user_agent", "browser", "os", "device", "country", "ip_hash",
}

// RecordBatch inserts a batch of clicks via COPY, increments each link's
// click_count, and returns the codes of links that became click-exhausted so the
// caller can evict them from the redirect cache. The whole batch is one
// transaction: clicks and counters stay consistent.
func (r *ClickRepository) RecordBatch(ctx context.Context, clicks []domain.Click) ([]string, error) {
	if len(clicks) == 0 {
		return nil, nil
	}

	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("begin click batch: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	rows := make([][]any, len(clicks))
	counts := make(map[uuid.UUID]int64, len(clicks))
	for i, c := range clicks {
		rows[i] = []any{
			c.LinkID, c.TenantID, c.OccurredAt, c.Referrer,
			c.UserAgent, c.Browser, c.OS, c.Device, c.Country, c.IPHash,
		}
		counts[c.LinkID]++
	}

	if _, err := tx.CopyFrom(ctx, pgx.Identifier{"clicks"}, clickCopyColumns, pgx.CopyFromRows(rows)); err != nil {
		return nil, fmt.Errorf("copy clicks: %w", err)
	}

	var exhausted []string
	const incQ = `
		UPDATE links
		SET click_count = click_count + $2, updated_at = now()
		WHERE id = $1
		RETURNING code, max_clicks, click_count`
	for linkID, n := range counts {
		var code string
		var maxClicks *int64
		var clickCount int64
		err := tx.QueryRow(ctx, incQ, linkID, n).Scan(&code, &maxClicks, &clickCount)
		if errors.Is(err, pgx.ErrNoRows) {
			continue // link hard-deleted; skip
		}
		if err != nil {
			return nil, fmt.Errorf("increment click count: %w", err)
		}
		if maxClicks != nil && clickCount >= *maxClicks {
			exhausted = append(exhausted, code)
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("commit click batch: %w", err)
	}
	return exhausted, nil
}

// SeriesForLink returns bucketed click counts for a link within [from, to),
// scoped to the tenant. bucket must be "hour" or "day".
func (r *ClickRepository) SeriesForLink(ctx context.Context, tenantID, linkID uuid.UUID, from, to time.Time, bucket domain.Bucket) ([]domain.TimePoint, error) {
	const q = `
		SELECT date_trunc($4::text, occurred_at) AS t, count(*)
		FROM clicks
		WHERE tenant_id = $1 AND link_id = $2 AND occurred_at >= $3 AND occurred_at < $5
		GROUP BY t
		ORDER BY t`
	rows, err := r.pool.Query(ctx, q, tenantID, linkID, from, string(bucket), to)
	if err != nil {
		return nil, fmt.Errorf("query click series: %w", err)
	}
	defer rows.Close()

	var series []domain.TimePoint
	for rows.Next() {
		var p domain.TimePoint
		if err := rows.Scan(&p.Time, &p.Count); err != nil {
			return nil, fmt.Errorf("scan time point: %w", err)
		}
		series = append(series, p)
	}
	return series, rows.Err()
}

// TotalForLink returns the click count for a link within [from, to).
func (r *ClickRepository) TotalForLink(ctx context.Context, tenantID, linkID uuid.UUID, from, to time.Time) (int64, error) {
	const q = `
		SELECT count(*)
		FROM clicks
		WHERE tenant_id = $1 AND link_id = $2 AND occurred_at >= $3 AND occurred_at < $4`
	var total int64
	if err := r.pool.QueryRow(ctx, q, tenantID, linkID, from, to).Scan(&total); err != nil {
		return 0, fmt.Errorf("query link total: %w", err)
	}
	return total, nil
}

// labelQuery runs a "SELECT <labelExpr> AS label, count(*) ... GROUP BY label
// ORDER BY count DESC LIMIT" query for a link over a window.
func (r *ClickRepository) labelQuery(ctx context.Context, labelExpr string, tenantID, linkID uuid.UUID, from, to time.Time, limit int) ([]domain.LabelCount, error) {
	q := fmt.Sprintf(`
		SELECT %s AS label, count(*) AS n
		FROM clicks
		WHERE tenant_id = $1 AND link_id = $2 AND occurred_at >= $3 AND occurred_at < $4
		GROUP BY label
		ORDER BY n DESC, label
		LIMIT $5`, labelExpr)
	rows, err := r.pool.Query(ctx, q, tenantID, linkID, from, to, limit)
	if err != nil {
		return nil, fmt.Errorf("query label counts: %w", err)
	}
	defer rows.Close()

	var out []domain.LabelCount
	for rows.Next() {
		var lc domain.LabelCount
		if err := rows.Scan(&lc.Label, &lc.Count); err != nil {
			return nil, fmt.Errorf("scan label count: %w", err)
		}
		out = append(out, lc)
	}
	return out, rows.Err()
}

// TopReferrers returns the most common referrers for a link (empty referrer
// shown as "(direct)").
func (r *ClickRepository) TopReferrers(ctx context.Context, tenantID, linkID uuid.UUID, from, to time.Time, limit int) ([]domain.LabelCount, error) {
	return r.labelQuery(ctx, `COALESCE(NULLIF(referrer, ''), '(direct)')`, tenantID, linkID, from, to, limit)
}

// TopCountries returns the most common countries for a link (empty shown as
// "(unknown)").
func (r *ClickRepository) TopCountries(ctx context.Context, tenantID, linkID uuid.UUID, from, to time.Time, limit int) ([]domain.LabelCount, error) {
	return r.labelQuery(ctx, `COALESCE(NULLIF(country, ''), '(unknown)')`, tenantID, linkID, from, to, limit)
}

// DeviceBreakdown returns click counts by device class for a link.
func (r *ClickRepository) DeviceBreakdown(ctx context.Context, tenantID, linkID uuid.UUID, from, to time.Time, limit int) ([]domain.LabelCount, error) {
	return r.labelQuery(ctx, `COALESCE(NULLIF(device, ''), 'unknown')`, tenantID, linkID, from, to, limit)
}

// TenantOverview returns tenant-wide totals and the busiest links.
func (r *ClickRepository) TenantOverview(ctx context.Context, tenantID uuid.UUID, topN int) (domain.TenantOverview, error) {
	var ov domain.TenantOverview

	const totalsQ = `
		SELECT
			(SELECT count(*) FROM links  WHERE tenant_id = $1 AND deleted_at IS NULL),
			(SELECT count(*) FROM clicks WHERE tenant_id = $1)`
	if err := r.pool.QueryRow(ctx, totalsQ, tenantID).Scan(&ov.TotalLinks, &ov.TotalClicks); err != nil {
		return ov, fmt.Errorf("query tenant totals: %w", err)
	}

	const topQ = `
		SELECT l.code AS label, count(*) AS n
		FROM clicks c
		JOIN links l ON l.id = c.link_id
		WHERE c.tenant_id = $1
		GROUP BY l.code
		ORDER BY n DESC, label
		LIMIT $2`
	rows, err := r.pool.Query(ctx, topQ, tenantID, topN)
	if err != nil {
		return ov, fmt.Errorf("query top links: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var lc domain.LabelCount
		if err := rows.Scan(&lc.Label, &lc.Count); err != nil {
			return ov, fmt.Errorf("scan top link: %w", err)
		}
		ov.TopLinks = append(ov.TopLinks, lc)
	}
	return ov, rows.Err()
}
