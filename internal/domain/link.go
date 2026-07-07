package domain

import (
	"time"

	"github.com/google/uuid"
)

// Redirect status codes a link may emit.
const (
	RedirectFound     = 302 // temporary; not cached aggressively by clients
	RedirectPermanent = 301 // permanent; cacheable
)

// Link is a short code that redirects to a target URL, owned by a tenant.
type Link struct {
	ID           uuid.UUID  `json:"id"`
	TenantID     uuid.UUID  `json:"tenant_id"`
	Code         string     `json:"code"`
	TargetURL    string     `json:"target_url"`
	RedirectType int        `json:"redirect_type"`
	ExpiresAt    *time.Time `json:"expires_at,omitempty"`
	MaxClicks    *int64     `json:"max_clicks,omitempty"`
	ClickCount   int64      `json:"click_count"`
	CreatedAt    time.Time  `json:"created_at"`
	UpdatedAt    time.Time  `json:"updated_at"`
	DeletedAt    *time.Time `json:"-"`
}

// IsExpired reports whether the link has passed its expiry instant.
func (l *Link) IsExpired(now time.Time) bool {
	return l.ExpiresAt != nil && !now.Before(*l.ExpiresAt)
}

// IsExhausted reports whether the link has reached its click limit.
func (l *Link) IsExhausted() bool {
	return l.MaxClicks != nil && l.ClickCount >= *l.MaxClicks
}

// IsActive reports whether the link can still serve redirects at the given time.
func (l *Link) IsActive(now time.Time) bool {
	return l.DeletedAt == nil && !l.IsExpired(now) && !l.IsExhausted()
}
