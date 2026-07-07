package analytics

import (
	"net"
	"time"

	"github.com/google/uuid"

	"github.com/a4anthony/go-link-shortener/internal/domain"
)

// ClickEvent is the raw redirect capture produced on the hot path. It holds only
// what can be read cheaply from the request; the expensive enrichment (UA
// parsing, geo lookup, IP hashing) is deferred to the pipeline workers.
type ClickEvent struct {
	LinkID     uuid.UUID
	TenantID   uuid.UUID
	Code       string
	OccurredAt time.Time
	Referrer   string
	UserAgent  string
	IP         string // raw client IP; never stored, only hashed/geo-resolved
}

// enrich turns a raw ClickEvent into a storable domain.Click by parsing the
// user agent, resolving the country from the full IP, and hashing the truncated
// IP. Geo resolution uses the full IP for accuracy but the IP is never persisted.
func enrich(e ClickEvent, geo GeoResolver, salt string) domain.Click {
	ua := ParseUserAgent(e.UserAgent)

	country := ""
	if ip := net.ParseIP(e.IP); ip != nil {
		if c, err := geo.Country(ip); err == nil {
			country = c
		}
	}

	return domain.Click{
		LinkID:     e.LinkID,
		TenantID:   e.TenantID,
		OccurredAt: e.OccurredAt,
		Referrer:   e.Referrer,
		UserAgent:  e.UserAgent,
		Browser:    ua.Browser,
		OS:         ua.OS,
		Device:     ua.Device,
		Country:    country,
		IPHash:     HashIP(e.IP, salt),
	}
}
