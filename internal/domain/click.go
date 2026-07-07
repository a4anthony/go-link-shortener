package domain

import (
	"time"

	"github.com/google/uuid"
)

// Click is a stored, fully-enriched redirect event. It is written to Postgres in
// batches by the analytics pipeline; the raw IP is never persisted, only a
// salted hash of its truncated form.
type Click struct {
	LinkID     uuid.UUID `json:"link_id"`
	TenantID   uuid.UUID `json:"tenant_id"`
	OccurredAt time.Time `json:"occurred_at"`
	Referrer   string    `json:"referrer"`
	UserAgent  string    `json:"user_agent"`
	Browser    string    `json:"browser"`
	OS         string    `json:"os"`
	Device     string    `json:"device"`
	Country    string    `json:"country"`
	IPHash     string    `json:"ip_hash"`
}
