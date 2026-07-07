package analytics

import (
	"context"
	"log/slog"
	"sync"
	"sync/atomic"
	"time"

	"github.com/a4anthony/go-link-shortener/internal/config"
	"github.com/a4anthony/go-link-shortener/internal/domain"
)

// flushTimeout bounds a single batch write, which runs detached from any request.
const flushTimeout = 5 * time.Second

// ClickWriter persists a batch of enriched clicks and reports which links became
// click-exhausted as a result (so their cache entries can be invalidated).
type ClickWriter interface {
	RecordBatch(ctx context.Context, clicks []domain.Click) (exhaustedCodes []string, err error)
}

// CacheInvalidator evicts a link code from the redirect cache. Optional.
type CacheInvalidator interface {
	Invalidate(ctx context.Context, code string) error
}

// Observer receives pipeline events for metrics. All methods must be cheap and
// non-blocking. Optional; a nil observer disables observation.
type Observer interface {
	IncEnqueued()
	IncDropped()
	IncFlushed(n int)
}

// BatchHook is notified after each batch is durably written, with the clicks
// that were flushed. It powers batched link.clicked webhooks. Optional.
type BatchHook interface {
	ClicksFlushed(clicks []domain.Click)
}

// Pipeline is a buffered-channel + worker-pool + batcher that ingests clicks
// asynchronously. Enqueue never blocks: when the buffer is full, the click is
// dropped and counted rather than back-pressuring the redirect.
type Pipeline struct {
	input  chan ClickEvent
	writer ClickWriter
	cache  CacheInvalidator
	geo       GeoResolver
	salt      string
	obs       Observer
	batchHook BatchHook
	log       *slog.Logger

	workers       int
	batchSize     int
	flushInterval time.Duration

	quit      chan struct{}
	wg        sync.WaitGroup
	startOnce sync.Once
	stopOnce  sync.Once

	enqueued atomic.Int64
	dropped  atomic.Int64
	flushed  atomic.Int64
}

// NewPipeline builds a Pipeline from analytics config. geo defaults to a no-op
// resolver if nil; cache and obs may be nil.
func NewPipeline(cfg config.AnalyticsConfig, writer ClickWriter, cache CacheInvalidator, geo GeoResolver, salt string, obs Observer, log *slog.Logger) *Pipeline {
	if geo == nil {
		geo = NoopResolver{}
	}
	return &Pipeline{
		input:         make(chan ClickEvent, cfg.BufferSize),
		writer:        writer,
		cache:         cache,
		geo:           geo,
		salt:          salt,
		obs:           obs,
		log:           log,
		workers:       cfg.Workers,
		batchSize:     cfg.BatchSize,
		flushInterval: cfg.FlushInterval,
		quit:          make(chan struct{}),
	}
}

// SetBatchHook registers a hook invoked with each durably-written batch. It must
// be called before Start.
func (p *Pipeline) SetBatchHook(h BatchHook) { p.batchHook = h }

// Start launches the worker pool. It is safe to call once; subsequent calls are
// no-ops.
func (p *Pipeline) Start() {
	p.startOnce.Do(func() {
		for i := 0; i < p.workers; i++ {
			p.wg.Add(1)
			go p.worker()
		}
		p.log.Info("analytics pipeline started", "workers", p.workers, "buffer", cap(p.input), "batch_size", p.batchSize)
	})
}

// Enqueue submits a click for asynchronous processing. It never blocks: it
// returns false (and increments the drop counter) if the buffer is full or the
// pipeline is shutting down.
func (p *Pipeline) Enqueue(e ClickEvent) bool {
	select {
	case <-p.quit:
		return false
	default:
	}

	select {
	case p.input <- e:
		p.enqueued.Add(1)
		if p.obs != nil {
			p.obs.IncEnqueued()
		}
		return true
	default:
		p.dropped.Add(1)
		if p.obs != nil {
			p.obs.IncDropped()
		}
		return false
	}
}

// worker drains events into a local batch, flushing on size or on the flush
// interval, and on shutdown drains whatever remains before exiting.
func (p *Pipeline) worker() {
	defer p.wg.Done()

	batch := make([]domain.Click, 0, p.batchSize)
	ticker := time.NewTicker(p.flushInterval)
	defer ticker.Stop()

	flush := func() {
		if len(batch) == 0 {
			return
		}
		p.flush(batch)
		batch = batch[:0]
	}

	for {
		select {
		case e := <-p.input:
			batch = append(batch, enrich(e, p.geo, p.salt))
			if len(batch) >= p.batchSize {
				flush()
			}
		case <-ticker.C:
			flush()
		case <-p.quit:
			// Drain everything still buffered, then flush and exit.
			for {
				select {
				case e := <-p.input:
					batch = append(batch, enrich(e, p.geo, p.salt))
					if len(batch) >= p.batchSize {
						flush()
					}
				default:
					flush()
					return
				}
			}
		}
	}
}

// flush writes one batch and invalidates the cache for any newly-exhausted links.
func (p *Pipeline) flush(batch []domain.Click) {
	ctx, cancel := context.WithTimeout(context.Background(), flushTimeout)
	defer cancel()

	toWrite := make([]domain.Click, len(batch))
	copy(toWrite, batch)

	exhausted, err := p.writer.RecordBatch(ctx, toWrite)
	if err != nil {
		p.log.Error("analytics batch flush failed", "error", err, "count", len(toWrite))
		return
	}
	p.flushed.Add(int64(len(toWrite)))
	if p.obs != nil {
		p.obs.IncFlushed(len(toWrite))
	}
	if p.batchHook != nil {
		p.batchHook.ClicksFlushed(toWrite)
	}

	for _, code := range exhausted {
		if p.cache != nil {
			if err := p.cache.Invalidate(ctx, code); err != nil {
				p.log.Warn("failed to invalidate exhausted link cache", "error", err, "code", code)
			}
		}
	}
}

// Shutdown stops accepting events and waits for the workers to drain and flush,
// bounded by ctx. It returns ctx.Err() if the deadline is hit before draining.
func (p *Pipeline) Shutdown(ctx context.Context) error {
	p.stopOnce.Do(func() { close(p.quit) })

	done := make(chan struct{})
	go func() {
		p.wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		p.log.Info("analytics pipeline drained",
			"enqueued", p.enqueued.Load(), "flushed", p.flushed.Load(), "dropped", p.dropped.Load())
		return nil
	case <-ctx.Done():
		p.log.Warn("analytics pipeline shutdown deadline exceeded",
			"queue_depth", p.QueueDepth(), "flushed", p.flushed.Load())
		return ctx.Err()
	}
}

// QueueDepth returns the number of buffered, not-yet-processed events.
func (p *Pipeline) QueueDepth() int { return len(p.input) }

// Enqueued returns the total number of accepted events.
func (p *Pipeline) Enqueued() int64 { return p.enqueued.Load() }

// Dropped returns the total number of events dropped due to a full buffer.
func (p *Pipeline) Dropped() int64 { return p.dropped.Load() }

// Flushed returns the total number of events written to storage.
func (p *Pipeline) Flushed() int64 { return p.flushed.Load() }
