package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/a4anthony/go-link-shortener/internal/domain"
	"github.com/a4anthony/go-link-shortener/internal/service"
)

// fakeLinkService implements the handler's LinkService interface.
type fakeLinkService struct {
	created   *domain.Link
	createErr error
	getLink   *domain.Link
	getErr    error
	listLinks []domain.Link
	updated   *domain.Link
	updateErr error
	deleteErr error

	lastCreateInput service.CreateLinkInput
	lastUpdateInput service.UpdateLinkInput
}

func (f *fakeLinkService) Create(_ context.Context, _ uuid.UUID, in service.CreateLinkInput) (*domain.Link, error) {
	f.lastCreateInput = in
	return f.created, f.createErr
}
func (f *fakeLinkService) Get(_ context.Context, _, _ uuid.UUID) (*domain.Link, error) {
	return f.getLink, f.getErr
}
func (f *fakeLinkService) List(_ context.Context, _ uuid.UUID, _, _ int) ([]domain.Link, error) {
	return f.listLinks, nil
}
func (f *fakeLinkService) Update(_ context.Context, _, _ uuid.UUID, in service.UpdateLinkInput) (*domain.Link, error) {
	f.lastUpdateInput = in
	return f.updated, f.updateErr
}
func (f *fakeLinkService) Delete(_ context.Context, _, _ uuid.UUID) error {
	return f.deleteErr
}

func setupLinkRouter(svc LinkService) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	// Inject a tenant on the context to stand in for the auth middleware.
	r.Use(func(c *gin.Context) {
		c.Set("tenant_id", uuid.New())
		c.Next()
	})
	h := NewLinkHandler(svc, "https://sho.rt")
	h.Register(r.Group("/api/v1"))
	return r
}

func doJSON(r http.Handler, method, path string, body any) *httptest.ResponseRecorder {
	var buf bytes.Buffer
	if body != nil {
		_ = json.NewEncoder(&buf).Encode(body)
	}
	req := httptest.NewRequest(method, path, &buf)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}

func sampleLink() *domain.Link {
	return &domain.Link{
		ID: uuid.New(), TenantID: uuid.New(), Code: "abc1234",
		TargetURL: "https://example.com", RedirectType: 302,
		ClickCount: 3, CreatedAt: time.Now(), UpdatedAt: time.Now(),
	}
}

func TestLinkHandler_Create(t *testing.T) {
	link := sampleLink()
	svc := &fakeLinkService{created: link}
	r := setupLinkRouter(svc)

	w := doJSON(r, http.MethodPost, "/api/v1/links", map[string]any{"url": "https://example.com"})
	require.Equal(t, http.StatusCreated, w.Code)

	var resp linkResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.Equal(t, "abc1234", resp.Code)
	assert.Equal(t, "https://sho.rt/abc1234", resp.ShortURL)
	assert.Equal(t, "https://example.com", svc.lastCreateInput.TargetURL)
}

func TestLinkHandler_Create_ValidationError(t *testing.T) {
	svc := &fakeLinkService{createErr: domain.ErrValidation}
	r := setupLinkRouter(svc)

	w := doJSON(r, http.MethodPost, "/api/v1/links", map[string]any{"url": "bad"})
	require.Equal(t, http.StatusBadRequest, w.Code)
	assert.Contains(t, w.Body.String(), "validation_error")
}

func TestLinkHandler_Create_MalformedJSON(t *testing.T) {
	r := setupLinkRouter(&fakeLinkService{})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/links", bytes.NewBufferString("{not json"))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestLinkHandler_Get_NotFound(t *testing.T) {
	svc := &fakeLinkService{getErr: domain.ErrNotFound}
	r := setupLinkRouter(svc)
	w := doJSON(r, http.MethodGet, "/api/v1/links/"+uuid.New().String(), nil)
	assert.Equal(t, http.StatusNotFound, w.Code)
	assert.Contains(t, w.Body.String(), "not_found")
}

func TestLinkHandler_Get_BadID(t *testing.T) {
	r := setupLinkRouter(&fakeLinkService{})
	w := doJSON(r, http.MethodGet, "/api/v1/links/not-a-uuid", nil)
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestLinkHandler_List(t *testing.T) {
	svc := &fakeLinkService{listLinks: []domain.Link{*sampleLink(), *sampleLink()}}
	r := setupLinkRouter(svc)
	w := doJSON(r, http.MethodGet, "/api/v1/links?limit=10", nil)
	require.Equal(t, http.StatusOK, w.Code)

	var resp struct {
		Links []linkResponse `json:"links"`
		Count int            `json:"count"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.Equal(t, 2, resp.Count)
	assert.Len(t, resp.Links, 2)
}

func TestLinkHandler_Update_ClearsExpiry(t *testing.T) {
	link := sampleLink()
	svc := &fakeLinkService{updated: link}
	r := setupLinkRouter(svc)

	// expires_at explicitly null should set ClearExpiresAt.
	body := bytes.NewBufferString(`{"expires_at": null, "url": "https://new.com"}`)
	req := httptest.NewRequest(http.MethodPatch, "/api/v1/links/"+uuid.New().String(), body)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)
	assert.True(t, svc.lastUpdateInput.ClearExpiresAt)
	require.NotNil(t, svc.lastUpdateInput.TargetURL)
	assert.Equal(t, "https://new.com", *svc.lastUpdateInput.TargetURL)
}

func TestLinkHandler_Delete(t *testing.T) {
	r := setupLinkRouter(&fakeLinkService{})
	w := doJSON(r, http.MethodDelete, "/api/v1/links/"+uuid.New().String(), nil)
	assert.Equal(t, http.StatusNoContent, w.Code)
}

func TestLinkHandler_Delete_NotFound(t *testing.T) {
	r := setupLinkRouter(&fakeLinkService{deleteErr: domain.ErrNotFound})
	w := doJSON(r, http.MethodDelete, "/api/v1/links/"+uuid.New().String(), nil)
	assert.Equal(t, http.StatusNotFound, w.Code)
}
