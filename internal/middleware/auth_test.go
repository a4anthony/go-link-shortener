package middleware

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"

	"github.com/a4anthony/go-link-shortener/internal/domain"
)

type fakeAuth struct {
	key *domain.APIKey
	err error
}

func (f fakeAuth) Authenticate(_ context.Context, _ string) (*domain.APIKey, error) {
	return f.key, f.err
}

func setupRouter(auth Authenticator) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.GET("/protected", APIKeyAuth(auth), func(c *gin.Context) {
		id := MustTenantID(c)
		c.JSON(http.StatusOK, gin.H{"tenant_id": id.String()})
	})
	return r
}

func TestAPIKeyAuth(t *testing.T) {
	tenantID := uuid.New()
	okAuth := fakeAuth{key: &domain.APIKey{ID: uuid.New(), TenantID: tenantID}}
	badAuth := fakeAuth{err: domain.ErrUnauthorized}

	tests := []struct {
		name       string
		auth       Authenticator
		header     string
		wantStatus int
	}{
		{"valid bearer", okAuth, "Bearer sk_live_validtoken0000", http.StatusOK},
		{"missing header", okAuth, "", http.StatusUnauthorized},
		{"wrong scheme", okAuth, "Basic sk_live_x", http.StatusUnauthorized},
		{"empty token", okAuth, "Bearer ", http.StatusUnauthorized},
		{"auth rejects", badAuth, "Bearer sk_live_whatever0000", http.StatusUnauthorized},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := setupRouter(tt.auth)
			req := httptest.NewRequest(http.MethodGet, "/protected", nil)
			if tt.header != "" {
				req.Header.Set("Authorization", tt.header)
			}
			w := httptest.NewRecorder()
			r.ServeHTTP(w, req)

			assert.Equal(t, tt.wantStatus, w.Code)
			if tt.wantStatus == http.StatusOK {
				assert.Contains(t, w.Body.String(), tenantID.String())
			} else {
				assert.Contains(t, w.Body.String(), "\"error\"")
			}
		})
	}
}

func TestBearerToken(t *testing.T) {
	tests := []struct {
		header    string
		wantToken string
		wantOK    bool
	}{
		{"Bearer abc", "abc", true},
		{"bearer abc", "abc", true}, // scheme is case-insensitive
		{"Bearer   abc  ", "abc", true},
		{"Basic abc", "", false},
		{"", "", false},
		{"Bearer ", "", false},
	}
	for _, tt := range tests {
		t.Run(tt.header, func(t *testing.T) {
			gin.SetMode(gin.TestMode)
			c, _ := gin.CreateTestContext(httptest.NewRecorder())
			c.Request = httptest.NewRequest(http.MethodGet, "/", nil)
			if tt.header != "" {
				c.Request.Header.Set("Authorization", tt.header)
			}
			token, ok := bearerToken(c)
			assert.Equal(t, tt.wantOK, ok)
			assert.Equal(t, tt.wantToken, token)
		})
	}
}
