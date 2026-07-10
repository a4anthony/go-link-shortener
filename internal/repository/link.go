package repository

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/a4anthony/go-link-shortener/internal/domain"
)

// LinkRepository persists links. Every read/write except GetByCode (the public
// redirect lookup) is tenant-scoped: the tenant_id predicate is mandatory.
type LinkRepository struct {
	db Querier
}

// NewLinkRepository builds a LinkRepository over the given pool.
func NewLinkRepository(db Querier) *LinkRepository {
	return &LinkRepository{db: db}
}

const linkColumns = `id, tenant_id, code, target_url, redirect_type,
	expires_at, max_clicks, click_count, created_at, updated_at, deleted_at`

func scanLink(row pgx.Row) (*domain.Link, error) {
	var l domain.Link
	err := row.Scan(
		&l.ID, &l.TenantID, &l.Code, &l.TargetURL, &l.RedirectType,
		&l.ExpiresAt, &l.MaxClicks, &l.ClickCount, &l.CreatedAt, &l.UpdatedAt, &l.DeletedAt,
	)
	if err != nil {
		return nil, err
	}
	return &l, nil
}

// Create inserts a link. A duplicate code (live) surfaces as domain.ErrConflict.
func (r *LinkRepository) Create(ctx context.Context, l *domain.Link) error {
	const q = `
		INSERT INTO links (tenant_id, code, target_url, redirect_type, expires_at, max_clicks)
		VALUES ($1, $2, $3, $4, $5, $6)
		RETURNING ` + linkColumns
	row := r.db.QueryRow(ctx, q, l.TenantID, l.Code, l.TargetURL, l.RedirectType, l.ExpiresAt, l.MaxClicks)
	created, err := scanLink(row)
	if err != nil {
		if isUniqueViolation(err) {
			return domain.ErrConflict
		}
		return fmt.Errorf("insert link: %w", err)
	}
	*l = *created
	return nil
}

// GetByIDForTenant loads a live link by id within a tenant.
func (r *LinkRepository) GetByIDForTenant(ctx context.Context, tenantID, id uuid.UUID) (*domain.Link, error) {
	const q = `SELECT ` + linkColumns + `
		FROM links
		WHERE id = $1 AND tenant_id = $2 AND deleted_at IS NULL`
	l, err := scanLink(r.db.QueryRow(ctx, q, id, tenantID))
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, domain.ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("select link: %w", err)
	}
	return l, nil
}

// GetByCode loads a live link by its public code. This is the only non
// tenant-scoped read: the redirect path has no tenant context.
func (r *LinkRepository) GetByCode(ctx context.Context, code string) (*domain.Link, error) {
	const q = `SELECT ` + linkColumns + `
		FROM links
		WHERE code = $1 AND deleted_at IS NULL`
	l, err := scanLink(r.db.QueryRow(ctx, q, code))
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, domain.ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("select link by code: %w", err)
	}
	return l, nil
}

// ListByTenant returns a page of a tenant's live links, newest first.
func (r *LinkRepository) ListByTenant(ctx context.Context, tenantID uuid.UUID, limit, offset int) ([]domain.Link, error) {
	const q = `SELECT ` + linkColumns + `
		FROM links
		WHERE tenant_id = $1 AND deleted_at IS NULL
		ORDER BY created_at DESC
		LIMIT $2 OFFSET $3`
	rows, err := r.db.Query(ctx, q, tenantID, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("list links: %w", err)
	}
	defer rows.Close()

	var links []domain.Link
	for rows.Next() {
		l, err := scanLink(rows)
		if err != nil {
			return nil, fmt.Errorf("scan link: %w", err)
		}
		links = append(links, *l)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate links: %w", err)
	}
	return links, nil
}

// Update writes mutable fields of a link, scoped to its tenant. It returns
// domain.ErrNotFound if the link does not exist for the tenant, and
// domain.ErrConflict if a new code collides.
func (r *LinkRepository) Update(ctx context.Context, l *domain.Link) error {
	const q = `
		UPDATE links
		SET code = $3, target_url = $4, redirect_type = $5,
		    expires_at = $6, max_clicks = $7, updated_at = now()
		WHERE id = $1 AND tenant_id = $2 AND deleted_at IS NULL
		RETURNING ` + linkColumns
	row := r.db.QueryRow(ctx, q, l.ID, l.TenantID, l.Code, l.TargetURL, l.RedirectType, l.ExpiresAt, l.MaxClicks)
	updated, err := scanLink(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.ErrNotFound
	}
	if err != nil {
		if isUniqueViolation(err) {
			return domain.ErrConflict
		}
		return fmt.Errorf("update link: %w", err)
	}
	*l = *updated
	return nil
}

// SoftDelete marks a link deleted (freeing its code), scoped to its tenant.
func (r *LinkRepository) SoftDelete(ctx context.Context, tenantID, id uuid.UUID) error {
	const q = `
		UPDATE links SET deleted_at = now(), updated_at = now()
		WHERE id = $1 AND tenant_id = $2 AND deleted_at IS NULL`
	tag, err := r.db.Exec(ctx, q, id, tenantID)
	if err != nil {
		return fmt.Errorf("soft delete link: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return domain.ErrNotFound
	}
	return nil
}

// PurgeExpired hard-deletes a tenant's links that expired or were soft-deleted
// before cutoff, returning the number removed. Their clicks are removed by the
// ON DELETE CASCADE on clicks.link_id. Used by the demo-tenant janitor; regular
// tenants keep soft-deleted rows.
func (r *LinkRepository) PurgeExpired(ctx context.Context, tenantID uuid.UUID, cutoff time.Time) (int64, error) {
	const q = `
		DELETE FROM links
		WHERE tenant_id = $1
		  AND (expires_at < $2 OR deleted_at < $2)`
	tag, err := r.db.Exec(ctx, q, tenantID, cutoff)
	if err != nil {
		return 0, fmt.Errorf("purge expired links: %w", err)
	}
	return tag.RowsAffected(), nil
}

// ExistsByCode reports whether a live link already uses the given code. Used by
// the shortcode collision-retry loop.
func (r *LinkRepository) ExistsByCode(ctx context.Context, code string) (bool, error) {
	const q = `SELECT EXISTS (SELECT 1 FROM links WHERE code = $1 AND deleted_at IS NULL)`
	var exists bool
	if err := r.db.QueryRow(ctx, q, code).Scan(&exists); err != nil {
		return false, fmt.Errorf("check code exists: %w", err)
	}
	return exists, nil
}
