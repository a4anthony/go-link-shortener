package middleware

import (
	"context"
	"strings"

	"github.com/gin-gonic/gin"

	"github.com/a4anthony/go-link-shortener/internal/domain"
	"github.com/a4anthony/go-link-shortener/internal/httpx"
)

// Authenticator resolves a plaintext bearer token to its API key. The auth
// service satisfies this; declaring it here keeps middleware decoupled from the
// service package's concrete type.
type Authenticator interface {
	Authenticate(ctx context.Context, token string) (*domain.APIKey, error)
}

// APIKeyAuth returns middleware that requires a valid `Authorization: Bearer
// <api_key>` header. On success it records the tenant on the context; on failure
// it aborts with a 401 error envelope.
func APIKeyAuth(auth Authenticator) gin.HandlerFunc {
	return func(c *gin.Context) {
		token, ok := bearerToken(c)
		if !ok {
			httpx.AbortError(c, 401, httpx.CodeUnauthorized, "missing or malformed Authorization header")
			return
		}

		key, err := auth.Authenticate(c.Request.Context(), token)
		if err != nil {
			httpx.AbortError(c, 401, httpx.CodeUnauthorized, "invalid API key")
			return
		}

		setAuth(c, key)
		c.Next()
	}
}

// bearerToken extracts the token from an `Authorization: Bearer <token>` header.
func bearerToken(c *gin.Context) (string, bool) {
	const prefix = "Bearer "
	h := c.GetHeader("Authorization")
	if len(h) <= len(prefix) || !strings.EqualFold(h[:len(prefix)], prefix) {
		return "", false
	}
	token := strings.TrimSpace(h[len(prefix):])
	if token == "" {
		return "", false
	}
	return token, true
}
