package domain

import (
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"time"

	"github.com/google/uuid"
)

// WebhookSecretPrefix identifies webhook signing secrets.
const WebhookSecretPrefix = "whsec_"

// GenerateWebhookSecret mints a random signing secret for a webhook.
func GenerateWebhookSecret() (string, error) {
	buf := make([]byte, 24)
	if _, err := rand.Read(buf); err != nil {
		return "", fmt.Errorf("generate webhook secret: %w", err)
	}
	return WebhookSecretPrefix + base64.RawURLEncoding.EncodeToString(buf), nil
}

// EventType identifies a webhook event.
type EventType string

const (
	// EventLinkCreated fires when a link is created.
	EventLinkCreated EventType = "link.created"
	// EventLinkClicked fires (batched) when a link receives clicks.
	EventLinkClicked EventType = "link.clicked"
)

// Valid reports whether t is a supported event type.
func (t EventType) Valid() bool {
	return t == EventLinkCreated || t == EventLinkClicked
}

// Event is a webhook payload envelope dispatched to subscribed endpoints.
type Event struct {
	ID         uuid.UUID      `json:"id"`
	Type       EventType      `json:"type"`
	TenantID   uuid.UUID      `json:"-"`
	OccurredAt time.Time      `json:"occurred_at"`
	Data       map[string]any `json:"data"`
}

// Webhook is a tenant-registered endpoint subscribed to one or more event types.
// After too many consecutive delivery failures it is disabled (dead-lettered).
type Webhook struct {
	ID           uuid.UUID   `json:"id"`
	TenantID     uuid.UUID   `json:"tenant_id"`
	URL          string      `json:"url"`
	Secret       string      `json:"-"`
	Events       []EventType `json:"events"`
	Active       bool        `json:"active"`
	FailureCount int         `json:"failure_count"`
	DisabledAt   *time.Time  `json:"disabled_at,omitempty"`
	CreatedAt    time.Time   `json:"created_at"`
}

// SubscribesTo reports whether the webhook is subscribed to event type t.
func (w *Webhook) SubscribesTo(t EventType) bool {
	for _, e := range w.Events {
		if e == t {
			return true
		}
	}
	return false
}
