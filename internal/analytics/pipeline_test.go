package analytics

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

	"github.com/a4anthony/go-link-shortener/internal/config"
	"github.com/a4anthony/go-link-shortener/internal/domain"
)

func testLogger() *slog.Logger { return slog.New(slog.NewJSONHandler(io.Discard, nil)) }

type fakeWriter struct {
	mu        sync.Mutex
	clicks    []domain.Click
	exhausted []string
	err       error
}

func (f *fakeWriter) RecordBatch(_ context.Context, clicks []domain.Click) ([]string, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.err != nil {
		return nil, f.err
	}
	f.clicks = append(f.clicks, clicks...)
	return f.exhausted, nil
}

func (f *fakeWriter) count() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return len(f.clicks)
}

func (f *fakeWriter) snapshot() []domain.Click {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]domain.Click(nil), f.clicks...)
}

type fakeInvalidator struct {
	mu    sync.Mutex
	codes []string
}

func (f *fakeInvalidator) Invalidate(_ context.Context, code string) error {
	f.mu.Lock()
	f.codes = append(f.codes, code)
	f.mu.Unlock()
	return nil
}

func (f *fakeInvalidator) got() []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]string(nil), f.codes...)
}

func sampleEvent() ClickEvent {
	return ClickEvent{
		LinkID: uuid.New(), TenantID: uuid.New(), Code: "abc1234",
		OccurredAt: time.Now(),
		Referrer:   "https://news.example.com",
		UserAgent:  "Mozilla/5.0 (iPhone; CPU iPhone OS 17_0 like Mac OS X) Version/17.0 Mobile Safari/604.1",
		IP:         "203.0.113.10",
	}
}

func newTestPipeline(w ClickWriter, cache CacheInvalidator, cfg config.AnalyticsConfig) *Pipeline {
	return NewPipeline(cfg, w, cache, NoopResolver{}, "salt", nil, testLogger())
}

func TestPipeline_FlushBySize(t *testing.T) {
	w := &fakeWriter{}
	p := newTestPipeline(w, nil, config.AnalyticsConfig{
		BufferSize: 100, Workers: 1, BatchSize: 5, FlushInterval: time.Hour,
	})
	p.Start()
	t.Cleanup(func() { _ = p.Shutdown(context.Background()) })

	for i := 0; i < 5; i++ {
		require.True(t, p.Enqueue(sampleEvent()))
	}
	assert.Eventually(t, func() bool { return w.count() == 5 }, time.Second, 10*time.Millisecond)
	assert.Equal(t, int64(5), p.Flushed())
}

func TestPipeline_FlushByTime(t *testing.T) {
	w := &fakeWriter{}
	p := newTestPipeline(w, nil, config.AnalyticsConfig{
		BufferSize: 100, Workers: 1, BatchSize: 1000, FlushInterval: 30 * time.Millisecond,
	})
	p.Start()
	t.Cleanup(func() { _ = p.Shutdown(context.Background()) })

	for i := 0; i < 3; i++ {
		require.True(t, p.Enqueue(sampleEvent()))
	}
	// Size threshold won't trigger; the ticker must flush.
	assert.Eventually(t, func() bool { return w.count() == 3 }, time.Second, 10*time.Millisecond)
}

func TestPipeline_DropsWhenFull(t *testing.T) {
	w := &fakeWriter{}
	// Do NOT start workers, so nothing drains the buffer.
	p := newTestPipeline(w, nil, config.AnalyticsConfig{
		BufferSize: 2, Workers: 1, BatchSize: 10, FlushInterval: time.Hour,
	})

	var accepted, dropped int
	for i := 0; i < 5; i++ {
		if p.Enqueue(sampleEvent()) {
			accepted++
		} else {
			dropped++
		}
	}
	assert.Equal(t, 2, accepted, "buffer capacity accepted")
	assert.Equal(t, 3, dropped, "overflow dropped, never blocked")
	assert.Equal(t, int64(2), p.Enqueued())
	assert.Equal(t, int64(3), p.Dropped())
}

func TestPipeline_ShutdownDrains(t *testing.T) {
	w := &fakeWriter{}
	p := newTestPipeline(w, nil, config.AnalyticsConfig{
		BufferSize: 100, Workers: 2, BatchSize: 1000, FlushInterval: time.Hour,
	})
	p.Start()

	const n = 25
	for i := 0; i < n; i++ {
		require.True(t, p.Enqueue(sampleEvent()))
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	require.NoError(t, p.Shutdown(ctx))
	assert.Equal(t, n, w.count(), "shutdown must drain and flush all buffered events")
}

func TestPipeline_EnrichesEvents(t *testing.T) {
	w := &fakeWriter{}
	p := newTestPipeline(w, nil, config.AnalyticsConfig{
		BufferSize: 10, Workers: 1, BatchSize: 1, FlushInterval: time.Hour,
	})
	p.Start()
	t.Cleanup(func() { _ = p.Shutdown(context.Background()) })

	require.True(t, p.Enqueue(sampleEvent()))
	assert.Eventually(t, func() bool { return w.count() == 1 }, time.Second, 10*time.Millisecond)

	got := w.snapshot()[0]
	assert.Equal(t, "Safari", got.Browser)
	assert.Equal(t, "iOS", got.OS)
	assert.Equal(t, DeviceMobile, got.Device)
	assert.NotEmpty(t, got.IPHash, "IP should be hashed")
	assert.NotContains(t, got.IPHash, "203.0.113", "raw IP must never be stored")
}

func TestPipeline_InvalidatesExhaustedLinks(t *testing.T) {
	w := &fakeWriter{exhausted: []string{"gone01"}}
	cache := &fakeInvalidator{}
	p := newTestPipeline(w, cache, config.AnalyticsConfig{
		BufferSize: 10, Workers: 1, BatchSize: 1, FlushInterval: time.Hour,
	})
	p.Start()
	t.Cleanup(func() { _ = p.Shutdown(context.Background()) })

	require.True(t, p.Enqueue(sampleEvent()))
	assert.Eventually(t, func() bool { return len(cache.got()) == 1 }, time.Second, 10*time.Millisecond)
	assert.Equal(t, []string{"gone01"}, cache.got())
}

func TestPipeline_EnqueueAfterShutdownIsRejected(t *testing.T) {
	w := &fakeWriter{}
	p := newTestPipeline(w, nil, config.AnalyticsConfig{
		BufferSize: 10, Workers: 1, BatchSize: 10, FlushInterval: time.Hour,
	})
	p.Start()
	require.NoError(t, p.Shutdown(context.Background()))

	assert.False(t, p.Enqueue(sampleEvent()), "enqueue after shutdown must be rejected")
}
