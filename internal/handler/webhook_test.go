package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/a4anthony/go-link-shortener/internal/domain"
	"github.com/a4anthony/go-link-shortener/internal/service"
)

type fakeWebhookService struct {
	created   *domain.Webhook
	createErr error
	list      []domain.Webhook
	deleteErr error
	lastInput service.CreateWebhookInput
}

func (f *fakeWebhookService) Create(_ context.Context, _ uuid.UUID, in service.CreateWebhookInput) (*domain.Webhook, error) {
	f.lastInput = in
	return f.created, f.createErr
}
func (f *fakeWebhookService) List(context.Context, uuid.UUID) ([]domain.Webhook, error) {
	return f.list, nil
}
func (f *fakeWebhookService) Delete(context.Context, uuid.UUID, uuid.UUID) error {
	return f.deleteErr
}

func setupWebhookRouter(svc WebhookService) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(func(c *gin.Context) { c.Set("tenant_id", uuid.New()); c.Next() })
	NewWebhookHandler(svc).Register(r.Group("/api/v1"))
	return r
}

func TestWebhookHandler_Create(t *testing.T) {
	created := &domain.Webhook{
		ID: uuid.New(), URL: "https://hooks.example.com", Secret: "whsec_shown_once",
		Events: []domain.EventType{domain.EventLinkCreated}, Active: true,
	}
	svc := &fakeWebhookService{created: created}
	r := setupWebhookRouter(svc)

	w := doJSON(r, http.MethodPost, "/api/v1/webhooks", map[string]any{
		"url":    "https://hooks.example.com",
		"events": []string{"link.created"},
	})
	require.Equal(t, http.StatusCreated, w.Code)

	var resp webhookResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.Equal(t, "whsec_shown_once", resp.Secret, "secret returned on create")
	assert.Equal(t, []string{"link.created"}, resp.Events)
	assert.Equal(t, domain.EventLinkCreated, svc.lastInput.Events[0])
}

func TestWebhookHandler_Create_Validation(t *testing.T) {
	svc := &fakeWebhookService{createErr: domain.ErrValidation}
	r := setupWebhookRouter(svc)
	w := doJSON(r, http.MethodPost, "/api/v1/webhooks", map[string]any{"url": "bad"})
	assert.Equal(t, http.StatusBadRequest, w.Code)
	assert.Contains(t, w.Body.String(), "validation_error")
}

func TestWebhookHandler_List_HidesSecret(t *testing.T) {
	svc := &fakeWebhookService{list: []domain.Webhook{
		{ID: uuid.New(), URL: "https://x.com", Secret: "whsec_secret", Events: []domain.EventType{domain.EventLinkClicked}},
	}}
	r := setupWebhookRouter(svc)
	w := doJSON(r, http.MethodGet, "/api/v1/webhooks", nil)
	require.Equal(t, http.StatusOK, w.Code)
	assert.NotContains(t, w.Body.String(), "whsec_secret", "list must never expose secrets")
}

func TestWebhookHandler_Delete(t *testing.T) {
	r := setupWebhookRouter(&fakeWebhookService{})
	w := doJSON(r, http.MethodDelete, "/api/v1/webhooks/"+uuid.New().String(), nil)
	assert.Equal(t, http.StatusNoContent, w.Code)
}

func TestWebhookHandler_Delete_NotFound(t *testing.T) {
	r := setupWebhookRouter(&fakeWebhookService{deleteErr: domain.ErrNotFound})
	w := doJSON(r, http.MethodDelete, "/api/v1/webhooks/"+uuid.New().String(), nil)
	assert.Equal(t, http.StatusNotFound, w.Code)
}
