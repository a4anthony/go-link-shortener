package webhook

import (
	"context"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/a4anthony/go-link-shortener/internal/config"
	"github.com/a4anthony/go-link-shortener/internal/domain"
)

func testLogger() *slog.Logger { return slog.New(slog.NewJSONHandler(io.Discard, nil)) }

type fakeWebhookRepo struct {
	mu        sync.Mutex
	hooks     []domain.Webhook
	successes []uuid.UUID
	failures  []uuid.UUID
	failCount map[uuid.UUID]int
}

func newFakeRepo(hooks ...domain.Webhook) *fakeWebhookRepo {
	return &fakeWebhookRepo{hooks: hooks, failCount: map[uuid.UUID]int{}}
}

func (f *fakeWebhookRepo) ListActiveForEvent(context.Context, uuid.UUID, domain.EventType) ([]domain.Webhook, error) {
	return f.hooks, nil
}
func (f *fakeWebhookRepo) RecordSuccess(_ context.Context, id uuid.UUID) error {
	f.mu.Lock()
	f.successes = append(f.successes, id)
	f.mu.Unlock()
	return nil
}
func (f *fakeWebhookRepo) RecordFailure(_ context.Context, id uuid.UUID, threshold int) (bool, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.failCount[id]++
	f.failures = append(f.failures, id)
	return f.failCount[id] >= threshold, nil
}
func (f *fakeWebhookRepo) successCount() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return len(f.successes)
}
func (f *fakeWebhookRepo) failureCount() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return len(f.failures)
}

func fastConfig() config.WebhookConfig {
	return config.WebhookConfig{
		Workers: 2, QueueSize: 100, MaxRetries: 3,
		BaseBackoff: time.Millisecond, MaxBackoff: 5 * time.Millisecond,
		Timeout: 2 * time.Second, DisableAfter: 1,
	}
}

func webhookFor(url, secret string) domain.Webhook {
	return domain.Webhook{
		ID: uuid.New(), TenantID: uuid.New(), URL: url, Secret: secret,
		Events: []domain.EventType{domain.EventLinkCreated}, Active: true,
	}
}

func TestDispatcher_DeliversSignedPayload(t *testing.T) {
	const secret = "whsec_abc"
	var (
		gotSig   atomic.Value
		gotEvent atomic.Value
		validSig atomic.Bool
		hits      atomic.Int32
	)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		sig := r.Header.Get(SignatureHeader)
		gotSig.Store(sig)
		gotEvent.Store(r.Header.Get(EventHeader))
		validSig.Store(Verify(secret, body, sig))
		hits.Add(1)
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	repo := newFakeRepo(webhookFor(srv.URL, secret))
	d := NewDispatcher(fastConfig(), repo, testLogger())
	d.Start()
	t.Cleanup(func() { _ = d.Shutdown(context.Background()) })

	require.True(t, d.Dispatch(domain.Event{Type: domain.EventLinkCreated, TenantID: uuid.New(), Data: map[string]any{"code": "abc"}}))

	assert.Eventually(t, func() bool { return hits.Load() == 1 }, 2*time.Second, 10*time.Millisecond)
	assert.True(t, validSig.Load(), "HMAC signature should verify")
	assert.Equal(t, "link.created", gotEvent.Load())
	assert.Eventually(t, func() bool { return repo.successCount() == 1 }, time.Second, 10*time.Millisecond)
}

func TestDispatcher_RetriesThenSucceeds(t *testing.T) {
	var hits atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if hits.Add(1) < 3 {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	repo := newFakeRepo(webhookFor(srv.URL, "s"))
	d := NewDispatcher(fastConfig(), repo, testLogger())
	d.Start()
	t.Cleanup(func() { _ = d.Shutdown(context.Background()) })

	require.True(t, d.Dispatch(domain.Event{Type: domain.EventLinkCreated, TenantID: uuid.New()}))

	assert.Eventually(t, func() bool { return repo.successCount() == 1 }, 3*time.Second, 10*time.Millisecond)
	assert.GreaterOrEqual(t, hits.Load(), int32(3), "should have retried until success")
	assert.Equal(t, 0, repo.failureCount(), "eventual success records no failure")
}

func TestDispatcher_DeadLettersAfterRetries(t *testing.T) {
	var hits atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		hits.Add(1)
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	repo := newFakeRepo(webhookFor(srv.URL, "s"))
	d := NewDispatcher(fastConfig(), repo, testLogger())
	d.Start()
	t.Cleanup(func() { _ = d.Shutdown(context.Background()) })

	require.True(t, d.Dispatch(domain.Event{Type: domain.EventLinkCreated, TenantID: uuid.New()}))

	// maxRetries=3 => 4 attempts, then a recorded failure (which dead-letters).
	assert.Eventually(t, func() bool { return repo.failureCount() == 1 }, 3*time.Second, 10*time.Millisecond)
	assert.Equal(t, 0, repo.successCount())
	assert.GreaterOrEqual(t, hits.Load(), int32(4), "all attempts should have been made")
}

func TestDispatcher_DropsWhenQueueFull(t *testing.T) {
	repo := newFakeRepo()
	cfg := fastConfig()
	cfg.QueueSize = 0 // unbuffered; no started workers to receive
	d := NewDispatcher(cfg, repo, testLogger())
	// Intentionally not started: no resolver draining events.

	assert.False(t, d.Dispatch(domain.Event{Type: domain.EventLinkCreated, TenantID: uuid.New()}))
	assert.Equal(t, int64(1), d.Dropped())
}
