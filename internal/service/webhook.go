package service

import (
	"context"
	"fmt"
	"net/url"

	"github.com/google/uuid"

	"github.com/a4anthony/go-link-shortener/internal/domain"
)

// WebhookRepository is the persistence WebhookService depends on.
type WebhookRepository interface {
	Create(ctx context.Context, w *domain.Webhook) error
	ListByTenant(ctx context.Context, tenantID uuid.UUID) ([]domain.Webhook, error)
	GetByIDForTenant(ctx context.Context, tenantID, id uuid.UUID) (*domain.Webhook, error)
	DeleteForTenant(ctx context.Context, tenantID, id uuid.UUID) error
}

// WebhookService implements webhook registration and management.
type WebhookService struct {
	repo WebhookRepository
}

// NewWebhookService builds a WebhookService.
func NewWebhookService(repo WebhookRepository) *WebhookService {
	return &WebhookService{repo: repo}
}

// CreateWebhookInput is the validated input for registering a webhook.
type CreateWebhookInput struct {
	URL    string
	Events []domain.EventType
}

// Create registers a webhook, generating its signing secret. The returned
// *domain.Webhook carries the plaintext secret (in its Secret field) so the
// caller can surface it once; it is never returned again.
func (s *WebhookService) Create(ctx context.Context, tenantID uuid.UUID, in CreateWebhookInput) (*domain.Webhook, error) {
	if err := validateWebhookURL(in.URL); err != nil {
		return nil, err
	}
	if err := validateEvents(in.Events); err != nil {
		return nil, err
	}

	secret, err := domain.GenerateWebhookSecret()
	if err != nil {
		return nil, err
	}

	w := &domain.Webhook{
		TenantID: tenantID,
		URL:      in.URL,
		Secret:   secret,
		Events:   dedupeEvents(in.Events),
		Active:   true,
	}
	if err := s.repo.Create(ctx, w); err != nil {
		return nil, err
	}
	// Preserve the plaintext secret for the one-time response.
	w.Secret = secret
	return w, nil
}

// List returns a tenant's webhooks.
func (s *WebhookService) List(ctx context.Context, tenantID uuid.UUID) ([]domain.Webhook, error) {
	return s.repo.ListByTenant(ctx, tenantID)
}

// Delete removes a tenant's webhook.
func (s *WebhookService) Delete(ctx context.Context, tenantID, id uuid.UUID) error {
	return s.repo.DeleteForTenant(ctx, tenantID, id)
}

func validateWebhookURL(raw string) error {
	if raw == "" {
		return fmt.Errorf("%w: url is required", domain.ErrValidation)
	}
	u, err := url.Parse(raw)
	if err != nil {
		return fmt.Errorf("%w: url is not parseable", domain.ErrValidation)
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return fmt.Errorf("%w: url scheme must be http or https", domain.ErrValidation)
	}
	if u.Host == "" {
		return fmt.Errorf("%w: url must include a host", domain.ErrValidation)
	}
	return nil
}

func validateEvents(events []domain.EventType) error {
	if len(events) == 0 {
		return fmt.Errorf("%w: at least one event type is required", domain.ErrValidation)
	}
	for _, e := range events {
		if !e.Valid() {
			return fmt.Errorf("%w: unknown event type %q", domain.ErrValidation, e)
		}
	}
	return nil
}

func dedupeEvents(events []domain.EventType) []domain.EventType {
	seen := make(map[domain.EventType]struct{}, len(events))
	out := make([]domain.EventType, 0, len(events))
	for _, e := range events {
		if _, ok := seen[e]; ok {
			continue
		}
		seen[e] = struct{}{}
		out = append(out, e)
	}
	return out
}
