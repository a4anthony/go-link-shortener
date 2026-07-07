// Package service holds business logic. Interfaces for the persistence a service
// needs are declared here (the consumer side); repository types satisfy them
// structurally, and main.go wires the concrete implementations in.
package service

import (
	"context"
	"errors"
	"log/slog"

	"github.com/google/uuid"

	"github.com/a4anthony/go-link-shortener/internal/domain"
)

// APIKeyReader is the persistence AuthService depends on.
type APIKeyReader interface {
	GetByHash(ctx context.Context, hash string) (*domain.APIKey, error)
	TouchLastUsed(ctx context.Context, id uuid.UUID) error
}

// AuthService authenticates inbound API keys against their stored hashes.
type AuthService struct {
	keys APIKeyReader
	log  *slog.Logger
}

// NewAuthService builds an AuthService.
func NewAuthService(keys APIKeyReader, log *slog.Logger) *AuthService {
	return &AuthService{keys: keys, log: log}
}

// Authenticate resolves a plaintext bearer token to its API key. It returns
// domain.ErrUnauthorized for any malformed, unknown, or revoked key so callers
// cannot distinguish the failure modes. On success it records last-used
// asynchronously so the auth path stays fast.
func (s *AuthService) Authenticate(ctx context.Context, token string) (*domain.APIKey, error) {
	if !domain.ValidKeyFormat(token) {
		return nil, domain.ErrUnauthorized
	}

	hash := domain.HashAPIKey(token)
	key, err := s.keys.GetByHash(ctx, hash)
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			return nil, domain.ErrUnauthorized
		}
		return nil, err
	}
	if key.Revoked() {
		return nil, domain.ErrUnauthorized
	}

	// Best-effort last-used bookkeeping; never block or fail auth on it.
	go func(id uuid.UUID) {
		bg, cancel := context.WithTimeout(context.WithoutCancel(ctx), touchTimeout)
		defer cancel()
		if err := s.keys.TouchLastUsed(bg, id); err != nil {
			s.log.Debug("touch api key last_used failed", "error", err, "key_id", id)
		}
	}(key.ID)

	return key, nil
}
