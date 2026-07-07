package service

import (
	"context"
	"io"
	"log/slog"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/a4anthony/go-link-shortener/internal/domain"
)

// fakeKeyReader is an in-memory APIKeyReader keyed by hash.
type fakeKeyReader struct {
	mu      sync.Mutex
	byHash  map[string]*domain.APIKey
	touched []uuid.UUID
	getErr  error
}

func (f *fakeKeyReader) GetByHash(_ context.Context, hash string) (*domain.APIKey, error) {
	if f.getErr != nil {
		return nil, f.getErr
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	k, ok := f.byHash[hash]
	if !ok {
		return nil, domain.ErrNotFound
	}
	return k, nil
}

func (f *fakeKeyReader) TouchLastUsed(_ context.Context, id uuid.UUID) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.touched = append(f.touched, id)
	return nil
}

func (f *fakeKeyReader) touchedIDs() []uuid.UUID {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]uuid.UUID(nil), f.touched...)
}

func testLogger() *slog.Logger {
	return slog.New(slog.NewJSONHandler(io.Discard, nil))
}

func TestAuthService_Authenticate(t *testing.T) {
	valid, err := domain.GenerateAPIKey()
	require.NoError(t, err)
	tenantID := uuid.New()
	keyID := uuid.New()

	revokedKey, err := domain.GenerateAPIKey()
	require.NoError(t, err)
	revokedAt := time.Now()

	reader := &fakeKeyReader{byHash: map[string]*domain.APIKey{
		valid.Hash: {
			ID: keyID, TenantID: tenantID, Name: "primary", Prefix: valid.Prefix, Hash: valid.Hash,
		},
		revokedKey.Hash: {
			ID: uuid.New(), TenantID: tenantID, Hash: revokedKey.Hash, RevokedAt: &revokedAt,
		},
	}}

	svc := NewAuthService(reader, testLogger())

	t.Run("valid key resolves tenant", func(t *testing.T) {
		key, err := svc.Authenticate(context.Background(), valid.Plaintext)
		require.NoError(t, err)
		assert.Equal(t, tenantID, key.TenantID)
		assert.Equal(t, keyID, key.ID)
	})

	t.Run("malformed token is unauthorized", func(t *testing.T) {
		_, err := svc.Authenticate(context.Background(), "not-a-key")
		assert.ErrorIs(t, err, domain.ErrUnauthorized)
	})

	t.Run("unknown key is unauthorized", func(t *testing.T) {
		other, _ := domain.GenerateAPIKey()
		_, err := svc.Authenticate(context.Background(), other.Plaintext)
		assert.ErrorIs(t, err, domain.ErrUnauthorized)
	})

	t.Run("revoked key is unauthorized", func(t *testing.T) {
		_, err := svc.Authenticate(context.Background(), revokedKey.Plaintext)
		assert.ErrorIs(t, err, domain.ErrUnauthorized)
	})
}

func TestAuthService_TouchesLastUsed(t *testing.T) {
	valid, err := domain.GenerateAPIKey()
	require.NoError(t, err)
	keyID := uuid.New()
	reader := &fakeKeyReader{byHash: map[string]*domain.APIKey{
		valid.Hash: {ID: keyID, TenantID: uuid.New(), Hash: valid.Hash},
	}}
	svc := NewAuthService(reader, testLogger())

	_, err = svc.Authenticate(context.Background(), valid.Plaintext)
	require.NoError(t, err)

	// last-used bookkeeping runs in a goroutine; poll briefly for it.
	assert.Eventually(t, func() bool {
		ids := reader.touchedIDs()
		return len(ids) == 1 && ids[0] == keyID
	}, time.Second, 10*time.Millisecond)
}
