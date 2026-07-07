package service

import (
	"context"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/a4anthony/go-link-shortener/internal/domain"
)

type fakeWebhookRepo struct {
	created   *domain.Webhook
	list      []domain.Webhook
	deleteErr error
}

func (f *fakeWebhookRepo) Create(_ context.Context, w *domain.Webhook) error {
	w.ID = uuid.New()
	f.created = w
	return nil
}
func (f *fakeWebhookRepo) ListByTenant(context.Context, uuid.UUID) ([]domain.Webhook, error) {
	return f.list, nil
}
func (f *fakeWebhookRepo) GetByIDForTenant(context.Context, uuid.UUID, uuid.UUID) (*domain.Webhook, error) {
	return nil, nil
}
func (f *fakeWebhookRepo) DeleteForTenant(context.Context, uuid.UUID, uuid.UUID) error {
	return f.deleteErr
}

func TestWebhookService_Create(t *testing.T) {
	repo := &fakeWebhookRepo{}
	svc := NewWebhookService(repo)

	w, err := svc.Create(context.Background(), uuid.New(), CreateWebhookInput{
		URL:    "https://hooks.example.com/x",
		Events: []domain.EventType{domain.EventLinkCreated, domain.EventLinkClicked},
	})
	require.NoError(t, err)
	assert.True(t, w.Active)
	assert.True(t, strings.HasPrefix(w.Secret, domain.WebhookSecretPrefix), "secret returned once")
	assert.Len(t, w.Events, 2)
}

func TestWebhookService_Create_DeduplicatesEvents(t *testing.T) {
	svc := NewWebhookService(&fakeWebhookRepo{})
	w, err := svc.Create(context.Background(), uuid.New(), CreateWebhookInput{
		URL:    "https://hooks.example.com/x",
		Events: []domain.EventType{domain.EventLinkCreated, domain.EventLinkCreated},
	})
	require.NoError(t, err)
	assert.Len(t, w.Events, 1)
}

func TestWebhookService_Create_Validation(t *testing.T) {
	svc := NewWebhookService(&fakeWebhookRepo{})
	tests := []struct {
		name string
		in   CreateWebhookInput
	}{
		{"empty url", CreateWebhookInput{URL: "", Events: []domain.EventType{domain.EventLinkCreated}}},
		{"bad scheme", CreateWebhookInput{URL: "ftp://x.com", Events: []domain.EventType{domain.EventLinkCreated}}},
		{"no events", CreateWebhookInput{URL: "https://x.com"}},
		{"unknown event", CreateWebhookInput{URL: "https://x.com", Events: []domain.EventType{"link.exploded"}}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := svc.Create(context.Background(), uuid.New(), tt.in)
			assert.ErrorIs(t, err, domain.ErrValidation)
		})
	}
}

func TestWebhookService_ListAndDelete(t *testing.T) {
	repo := &fakeWebhookRepo{list: []domain.Webhook{{ID: uuid.New()}}}
	svc := NewWebhookService(repo)

	list, err := svc.List(context.Background(), uuid.New())
	require.NoError(t, err)
	assert.Len(t, list, 1)

	require.NoError(t, svc.Delete(context.Background(), uuid.New(), uuid.New()))

	repo.deleteErr = domain.ErrNotFound
	assert.ErrorIs(t, svc.Delete(context.Background(), uuid.New(), uuid.New()), domain.ErrNotFound)
}
