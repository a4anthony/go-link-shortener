package handler

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/a4anthony/go-link-shortener/internal/domain"
	"github.com/a4anthony/go-link-shortener/internal/middleware"
	"github.com/a4anthony/go-link-shortener/internal/service"
)

// LinkService is the behaviour the link handler needs from the service layer.
type LinkService interface {
	Create(ctx context.Context, tenantID uuid.UUID, in service.CreateLinkInput) (*domain.Link, error)
	Get(ctx context.Context, tenantID, id uuid.UUID) (*domain.Link, error)
	List(ctx context.Context, tenantID uuid.UUID, limit, offset int) ([]domain.Link, error)
	Update(ctx context.Context, tenantID, id uuid.UUID, in service.UpdateLinkInput) (*domain.Link, error)
	Delete(ctx context.Context, tenantID, id uuid.UUID) error
}

// LinkHandler serves the /api/v1/links resource.
type LinkHandler struct {
	svc     LinkService
	baseURL string
}

// NewLinkHandler builds a LinkHandler. baseURL is used to render absolute short
// URLs in responses.
func NewLinkHandler(svc LinkService, baseURL string) *LinkHandler {
	return &LinkHandler{svc: svc, baseURL: strings.TrimRight(baseURL, "/")}
}

// Register mounts the link routes on the given (already authenticated) group.
func (h *LinkHandler) Register(g *gin.RouterGroup) {
	g.POST("/links", h.Create)
	g.GET("/links", h.List)
	g.GET("/links/:id", h.Get)
	g.PATCH("/links/:id", h.Update)
	g.DELETE("/links/:id", h.Delete)
}

type createLinkRequest struct {
	URL          string     `json:"url"`
	CustomAlias  string     `json:"custom_alias"`
	RedirectType int        `json:"redirect_type"`
	ExpiresAt    *time.Time `json:"expires_at"`
	MaxClicks    *int64     `json:"max_clicks"`
}

type linkResponse struct {
	ID           uuid.UUID  `json:"id"`
	Code         string     `json:"code"`
	ShortURL     string     `json:"short_url"`
	TargetURL    string     `json:"target_url"`
	RedirectType int        `json:"redirect_type"`
	ExpiresAt    *time.Time `json:"expires_at,omitempty"`
	MaxClicks    *int64     `json:"max_clicks,omitempty"`
	ClickCount   int64      `json:"click_count"`
	CreatedAt    time.Time  `json:"created_at"`
	UpdatedAt    time.Time  `json:"updated_at"`
}

func (h *LinkHandler) toResponse(l *domain.Link) linkResponse {
	return linkResponse{
		ID:           l.ID,
		Code:         l.Code,
		ShortURL:     h.baseURL + "/" + l.Code,
		TargetURL:    l.TargetURL,
		RedirectType: l.RedirectType,
		ExpiresAt:    l.ExpiresAt,
		MaxClicks:    l.MaxClicks,
		ClickCount:   l.ClickCount,
		CreatedAt:    l.CreatedAt,
		UpdatedAt:    l.UpdatedAt,
	}
}

// Create handles POST /api/v1/links.
func (h *LinkHandler) Create(c *gin.Context) {
	var req createLinkRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		respondValidation(c, "request body must be valid JSON")
		return
	}

	link, err := h.svc.Create(c.Request.Context(), middleware.MustTenantID(c), service.CreateLinkInput{
		TargetURL:    req.URL,
		CustomAlias:  strings.TrimSpace(req.CustomAlias),
		RedirectType: req.RedirectType,
		ExpiresAt:    req.ExpiresAt,
		MaxClicks:    req.MaxClicks,
	})
	if err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusCreated, h.toResponse(link))
}

// List handles GET /api/v1/links?limit=&offset=.
func (h *LinkHandler) List(c *gin.Context) {
	limit := atoiDefault(c.Query("limit"), 50)
	offset := atoiDefault(c.Query("offset"), 0)

	links, err := h.svc.List(c.Request.Context(), middleware.MustTenantID(c), limit, offset)
	if err != nil {
		respondError(c, err)
		return
	}
	out := make([]linkResponse, 0, len(links))
	for i := range links {
		out = append(out, h.toResponse(&links[i]))
	}
	c.JSON(http.StatusOK, gin.H{"links": out, "count": len(out)})
}

// Get handles GET /api/v1/links/:id.
func (h *LinkHandler) Get(c *gin.Context) {
	id, ok := parseID(c)
	if !ok {
		return
	}
	link, err := h.svc.Get(c.Request.Context(), middleware.MustTenantID(c), id)
	if err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, h.toResponse(link))
}

// Update handles PATCH /api/v1/links/:id. Absent fields are unchanged; a field
// present with a JSON null clears it (for expires_at and max_clicks).
func (h *LinkHandler) Update(c *gin.Context) {
	id, ok := parseID(c)
	if !ok {
		return
	}

	raw, err := io.ReadAll(c.Request.Body)
	if err != nil {
		respondValidation(c, "could not read request body")
		return
	}
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(raw, &fields); err != nil {
		respondValidation(c, "request body must be a valid JSON object")
		return
	}

	in, verr := buildUpdateInput(fields)
	if verr != "" {
		respondValidation(c, verr)
		return
	}

	link, err := h.svc.Update(c.Request.Context(), middleware.MustTenantID(c), id, in)
	if err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, h.toResponse(link))
}

// Delete handles DELETE /api/v1/links/:id.
func (h *LinkHandler) Delete(c *gin.Context) {
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

// buildUpdateInput translates a presence-aware field map into an UpdateLinkInput.
// It returns a non-empty validation message on malformed values.
func buildUpdateInput(fields map[string]json.RawMessage) (service.UpdateLinkInput, string) {
	var in service.UpdateLinkInput

	if raw, ok := fields["url"]; ok {
		var s string
		if err := json.Unmarshal(raw, &s); err != nil {
			return in, "url must be a string"
		}
		in.TargetURL = &s
	}
	if raw, ok := fields["redirect_type"]; ok {
		var n int
		if err := json.Unmarshal(raw, &n); err != nil {
			return in, "redirect_type must be an integer"
		}
		in.RedirectType = &n
	}
	if raw, ok := fields["expires_at"]; ok {
		if isJSONNull(raw) {
			in.ClearExpiresAt = true
		} else {
			var t time.Time
			if err := json.Unmarshal(raw, &t); err != nil {
				return in, "expires_at must be an RFC3339 timestamp or null"
			}
			in.ExpiresAt = &t
		}
	}
	if raw, ok := fields["max_clicks"]; ok {
		if isJSONNull(raw) {
			in.ClearMaxClicks = true
		} else {
			var n int64
			if err := json.Unmarshal(raw, &n); err != nil {
				return in, "max_clicks must be an integer or null"
			}
			in.MaxClicks = &n
		}
	}
	return in, ""
}

func isJSONNull(raw json.RawMessage) bool {
	return strings.TrimSpace(string(raw)) == "null"
}

// parseID extracts and validates the :id path parameter, writing a 400 on
// failure and returning ok=false.
func parseID(c *gin.Context) (uuid.UUID, bool) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		respondValidation(c, "id must be a valid UUID")
		return uuid.Nil, false
	}
	return id, true
}

func atoiDefault(s string, def int) int {
	if s == "" {
		return def
	}
	n, err := strconv.Atoi(s)
	if err != nil {
		return def
	}
	return n
}
