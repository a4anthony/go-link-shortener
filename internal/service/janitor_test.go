package service

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// fakePurger records purge calls and signals each sweep on a channel.
type fakePurger struct {
	calls   chan time.Time // receives the cutoff of every sweep
	tenants chan uuid.UUID
}

func newFakePurger() *fakePurger {
	return &fakePurger{
		calls:   make(chan time.Time, 16),
		tenants: make(chan uuid.UUID, 16),
	}
}

func (f *fakePurger) PurgeExpired(_ context.Context, tenantID uuid.UUID, cutoff time.Time) (int64, error) {
	f.calls <- cutoff
	f.tenants <- tenantID
	return 1, nil
}

func TestLinkJanitor_SweepsImmediatelyAndOnTicks(t *testing.T) {
	purger := newFakePurger()
	tenant := uuid.New()
	j := NewLinkJanitor(purger, tenant, 5*time.Millisecond, 24*time.Hour, testLogger())
	base := time.Date(2026, 7, 10, 12, 0, 0, 0, time.UTC)
	j.now = func() time.Time { return base }

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		j.Run(ctx)
		close(done)
	}()

	// First sweep is immediate; at least one more arrives from the ticker.
	for i := 0; i < 2; i++ {
		select {
		case cutoff := <-purger.calls:
			assert.True(t, cutoff.Equal(base.Add(-24*time.Hour)), "cutoff must be now-retention")
			assert.Equal(t, tenant, <-purger.tenants)
		case <-time.After(2 * time.Second):
			t.Fatalf("sweep %d never happened", i+1)
		}
	}

	cancel()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("janitor did not stop on context cancellation")
	}
}

func TestLinkJanitor_StopsCleanlyWithoutTick(t *testing.T) {
	purger := newFakePurger()
	j := NewLinkJanitor(purger, uuid.New(), time.Hour, time.Hour, testLogger())

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		j.Run(ctx)
		close(done)
	}()

	// Only the immediate sweep runs before cancellation.
	require.NotNil(t, <-purger.calls)
	cancel()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("janitor did not stop on context cancellation")
	}
}
