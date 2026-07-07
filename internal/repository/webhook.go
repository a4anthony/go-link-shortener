package repository

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/a4anthony/go-link-shortener/internal/domain"
)

// WebhookRepository persists webhook subscriptions. All reads/writes are
// tenant-scoped except the delivery-bookkeeping updates keyed by webhook id.
type WebhookRepository struct {
	db Querier
}

// NewWebhookRepository builds a WebhookRepository.
func NewWebhookRepository(db Querier) *WebhookRepository {
	return &WebhookRepository{db: db}
}

const webhookColumns = `id, tenant_id, url, secret, events, active, failure_count, disabled_at, created_at`

func scanWebhook(row pgx.Row) (*domain.Webhook, error) {
	var w domain.Webhook
	var events []string
	if err := row.Scan(&w.ID, &w.TenantID, &w.URL, &w.Secret, &events, &w.Active, &w.FailureCount, &w.DisabledAt, &w.CreatedAt); err != nil {
		return nil, err
	}
	w.Events = toEventTypes(events)
	return &w, nil
}

// Create inserts a webhook subscription.
func (r *WebhookRepository) Create(ctx context.Context, w *domain.Webhook) error {
	const q = `
		INSERT INTO webhooks (tenant_id, url, secret, events)
		VALUES ($1, $2, $3, $4)
		RETURNING ` + webhookColumns
	row := r.db.QueryRow(ctx, q, w.TenantID, w.URL, w.Secret, fromEventTypes(w.Events))
	created, err := scanWebhook(row)
	if err != nil {
		return fmt.Errorf("insert webhook: %w", err)
	}
	*w = *created
	return nil
}

// ListByTenant returns all of a tenant's webhooks, newest first.
func (r *WebhookRepository) ListByTenant(ctx context.Context, tenantID uuid.UUID) ([]domain.Webhook, error) {
	const q = `SELECT ` + webhookColumns + ` FROM webhooks WHERE tenant_id = $1 ORDER BY created_at DESC`
	rows, err := r.db.Query(ctx, q, tenantID)
	if err != nil {
		return nil, fmt.Errorf("list webhooks: %w", err)
	}
	defer rows.Close()

	var hooks []domain.Webhook
	for rows.Next() {
		w, err := scanWebhook(rows)
		if err != nil {
			return nil, fmt.Errorf("scan webhook: %w", err)
		}
		hooks = append(hooks, *w)
	}
	return hooks, rows.Err()
}

// GetByIDForTenant loads a webhook by id within a tenant.
func (r *WebhookRepository) GetByIDForTenant(ctx context.Context, tenantID, id uuid.UUID) (*domain.Webhook, error) {
	const q = `SELECT ` + webhookColumns + ` FROM webhooks WHERE id = $1 AND tenant_id = $2`
	w, err := scanWebhook(r.db.QueryRow(ctx, q, id, tenantID))
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, domain.ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("select webhook: %w", err)
	}
	return w, nil
}

// DeleteForTenant removes a webhook, scoped to its tenant.
func (r *WebhookRepository) DeleteForTenant(ctx context.Context, tenantID, id uuid.UUID) error {
	const q = `DELETE FROM webhooks WHERE id = $1 AND tenant_id = $2`
	tag, err := r.db.Exec(ctx, q, id, tenantID)
	if err != nil {
		return fmt.Errorf("delete webhook: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return domain.ErrNotFound
	}
	return nil
}

// ListActiveForEvent returns the active webhooks in a tenant subscribed to the
// given event type. This is the dispatch-path lookup.
func (r *WebhookRepository) ListActiveForEvent(ctx context.Context, tenantID uuid.UUID, event domain.EventType) ([]domain.Webhook, error) {
	const q = `SELECT ` + webhookColumns + `
		FROM webhooks
		WHERE tenant_id = $1 AND active AND $2 = ANY(events)`
	rows, err := r.db.Query(ctx, q, tenantID, string(event))
	if err != nil {
		return nil, fmt.Errorf("list active webhooks: %w", err)
	}
	defer rows.Close()

	var hooks []domain.Webhook
	for rows.Next() {
		w, err := scanWebhook(rows)
		if err != nil {
			return nil, fmt.Errorf("scan webhook: %w", err)
		}
		hooks = append(hooks, *w)
	}
	return hooks, rows.Err()
}

// RecordSuccess clears the consecutive-failure counter after a delivery succeeds.
func (r *WebhookRepository) RecordSuccess(ctx context.Context, id uuid.UUID) error {
	const q = `UPDATE webhooks SET failure_count = 0 WHERE id = $1`
	if _, err := r.db.Exec(ctx, q, id); err != nil {
		return fmt.Errorf("record webhook success: %w", err)
	}
	return nil
}

// RecordFailure increments the consecutive-failure counter and, once it reaches
// disableThreshold, dead-letters the webhook (active=false, disabled_at set). It
// reports whether the webhook is now disabled.
func (r *WebhookRepository) RecordFailure(ctx context.Context, id uuid.UUID, disableThreshold int) (bool, error) {
	const q = `
		UPDATE webhooks
		SET failure_count = failure_count + 1,
		    active = (failure_count + 1 < $2),
		    disabled_at = CASE WHEN failure_count + 1 >= $2 THEN now() ELSE disabled_at END
		WHERE id = $1
		RETURNING active`
	var active bool
	err := r.db.QueryRow(ctx, q, id, disableThreshold).Scan(&active)
	if errors.Is(err, pgx.ErrNoRows) {
		return false, domain.ErrNotFound
	}
	if err != nil {
		return false, fmt.Errorf("record webhook failure: %w", err)
	}
	return !active, nil
}

func toEventTypes(ss []string) []domain.EventType {
	out := make([]domain.EventType, len(ss))
	for i, s := range ss {
		out[i] = domain.EventType(s)
	}
	return out
}

func fromEventTypes(es []domain.EventType) []string {
	out := make([]string, len(es))
	for i, e := range es {
		out[i] = string(e)
	}
	return out
}
