package handler

import (
	"context"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/a4anthony/go-link-shortener/internal/domain"
	"github.com/a4anthony/go-link-shortener/internal/middleware"
	"github.com/a4anthony/go-link-shortener/internal/service"
)

// WebhookService is the behaviour the webhook handler needs.
type WebhookService interface {
	Create(ctx context.Context, tenantID uuid.UUID, in service.CreateWebhookInput) (*domain.Webhook, error)
	List(ctx context.Context, tenantID uuid.UUID) ([]domain.Webhook, error)
	Delete(ctx context.Context, tenantID, id uuid.UUID) error
}

// WebhookHandler serves the /api/v1/webhooks resource.
type WebhookHandler struct {
	svc WebhookService
}

// NewWebhookHandler builds a WebhookHandler.
func NewWebhookHandler(svc WebhookService) *WebhookHandler {
	return &WebhookHandler{svc: svc}
}

// Register mounts webhook routes on the authenticated group.
func (h *WebhookHandler) Register(g *gin.RouterGroup) {
	g.POST("/webhooks", h.Create)
	g.GET("/webhooks", h.List)
	g.DELETE("/webhooks/:id", h.Delete)
}

type createWebhookRequest struct {
	URL    string   `json:"url"`
	Events []string `json:"events"`
}

type webhookResponse struct {
	ID        uuid.UUID `json:"id"`
	URL       string    `json:"url"`
	Events    []string  `json:"events"`
	Active    bool      `json:"active"`
	CreatedAt time.Time `json:"created_at"`
	// Secret is populated only in the create response, shown exactly once.
	Secret string `json:"secret,omitempty"`
}

func toWebhookResponse(w *domain.Webhook, includeSecret bool) webhookResponse {
	events := make([]string, len(w.Events))
	for i, e := range w.Events {
		events[i] = string(e)
	}
	resp := webhookResponse{
		ID:        w.ID,
		URL:       w.URL,
		Events:    events,
		Active:    w.Active,
		CreatedAt: w.CreatedAt,
	}
	if includeSecret {
		resp.Secret = w.Secret
	}
	return resp
}

// Create handles POST /api/v1/webhooks.
func (h *WebhookHandler) Create(c *gin.Context) {
	var req createWebhookRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		respondValidation(c, "request body must be valid JSON")
		return
	}

	events := make([]domain.EventType, len(req.Events))
	for i, e := range req.Events {
		events[i] = domain.EventType(e)
	}

	w, err := h.svc.Create(c.Request.Context(), middleware.MustTenantID(c), service.CreateWebhookInput{
		URL:    req.URL,
		Events: events,
	})
	if err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusCreated, toWebhookResponse(w, true))
}

// List handles GET /api/v1/webhooks.
func (h *WebhookHandler) List(c *gin.Context) {
	hooks, err := h.svc.List(c.Request.Context(), middleware.MustTenantID(c))
	if err != nil {
		respondError(c, err)
		return
	}
	out := make([]webhookResponse, 0, len(hooks))
	for i := range hooks {
		out = append(out, toWebhookResponse(&hooks[i], false))
	}
	c.JSON(http.StatusOK, gin.H{"webhooks": out, "count": len(out)})
}

// Delete handles DELETE /api/v1/webhooks/:id.
func (h *WebhookHandler) Delete(c *gin.Context) {
	id, ok := parseID(c)
	if !ok {
		return
	}
	if err := h.svc.Delete(c.Request.Context(), middleware.MustTenantID(c), id); err != nil {
		respondError(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}
