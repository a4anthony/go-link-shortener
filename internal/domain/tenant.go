// Package domain holds the core entities shared across the handler, service,
// and repository layers. It has no dependencies on those layers, which keeps the
// dependency graph acyclic: repositories and services both import domain, never
// the reverse.
package domain

import (
	"time"

	"github.com/google/uuid"
)

// Tenant is an organisation account. All other resources (links, api keys,
// webhooks, clicks) belong to exactly one tenant and are isolated by tenant_id.
type Tenant struct {
	ID        uuid.UUID `json:"id"`
	Name      string    `json:"name"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}
