package handler

import (
	"context"
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/a4anthony/go-link-shortener/internal/domain"
)

// RedirectResolver resolves a public code to its live link, or returns
// domain.ErrNotFound / domain.ErrGone.
type RedirectResolver interface {
	Resolve(ctx context.Context, code string) (*domain.Link, error)
}

// ClickRecorder is notified of a served redirect so analytics can record it
// asynchronously. Optional; a nil recorder disables click capture. It must never
// block the redirect.
type ClickRecorder interface {
	Record(c *gin.Context, link *domain.Link)
}

// RedirectHandler serves the public GET /:code hot path.
type RedirectHandler struct {
	svc    RedirectResolver
	clicks ClickRecorder
}

// NewRedirectHandler builds a RedirectHandler. clicks may be nil.
func NewRedirectHandler(svc RedirectResolver, clicks ClickRecorder) *RedirectHandler {
	return &RedirectHandler{svc: svc, clicks: clicks}
}

// Register mounts the public redirect route on the engine root.
func (h *RedirectHandler) Register(r gin.IRoutes) {
	r.GET("/:code", h.Redirect)
}

// Redirect resolves the code and issues the configured 301/302 redirect. Missing
// codes return 404; expired or click-exhausted links return 410 Gone.
func (h *RedirectHandler) Redirect(c *gin.Context) {
	code := c.Param("code")
	link, err := h.svc.Resolve(c.Request.Context(), code)
	if err != nil {
		respondError(c, err)
		return
	}

	// Record the click out-of-band; this must not block or fail the redirect.
	if h.clicks != nil {
		h.clicks.Record(c, link)
	}

	c.Redirect(redirectStatus(link.RedirectType), link.TargetURL)
}

// redirectStatus clamps to a valid 3xx redirect status, defaulting to 302.
func redirectStatus(rt int) int {
	if rt == domain.RedirectPermanent {
		return http.StatusMovedPermanently
	}
	return http.StatusFound
}
