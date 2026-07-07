// Package middleware holds Gin middleware: API-key auth, rate limiting, request
// ID, recovery, and request logging. Auth stores the resolved tenant on the Gin
// context; downstream handlers read it through the accessors here.
package middleware

import (
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/a4anthony/go-link-shortener/internal/domain"
)

// Gin context keys. Kept unexported and accessed via typed helpers so handlers
// never touch raw string keys.
const (
	ctxKeyTenantID = "tenant_id"
	ctxKeyAPIKeyID = "api_key_id"
	ctxKeyRequestID = "request_id"
)

// setAuth records the authenticated key's tenant and id on the context.
func setAuth(c *gin.Context, key *domain.APIKey) {
	c.Set(ctxKeyTenantID, key.TenantID)
	c.Set(ctxKeyAPIKeyID, key.ID)
}

// TenantID returns the authenticated tenant id and whether one is present.
func TenantID(c *gin.Context) (uuid.UUID, bool) {
	v, ok := c.Get(ctxKeyTenantID)
	if !ok {
		return uuid.Nil, false
	}
	id, ok := v.(uuid.UUID)
	return id, ok
}

// MustTenantID returns the authenticated tenant id, or uuid.Nil if unset. Use
// only on routes guarded by the auth middleware.
func MustTenantID(c *gin.Context) uuid.UUID {
	id, _ := TenantID(c)
	return id
}

// APIKeyID returns the authenticated API key id, if present.
func APIKeyID(c *gin.Context) (uuid.UUID, bool) {
	v, ok := c.Get(ctxKeyAPIKeyID)
	if !ok {
		return uuid.Nil, false
	}
	id, ok := v.(uuid.UUID)
	return id, ok
}

// GetRequestID returns the request id assigned by the RequestID middleware.
func GetRequestID(c *gin.Context) string {
	if v, ok := c.Get(ctxKeyRequestID); ok {
		if s, ok := v.(string); ok {
			return s
		}
	}
	return ""
}
