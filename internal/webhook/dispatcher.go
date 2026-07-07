package webhook

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"math/rand/v2"
	"net/http"
	"strconv"
	"sync"
	"sync/atomic"
	"time"

	"github.com/google/uuid"

	"github.com/a4anthony/go-link-shortener/internal/config"
	"github.com/a4anthony/go-link-shortener/internal/domain"
)

// Repository is the persistence the dispatcher needs: subscriber lookup and
// delivery bookkeeping.
type Repository interface {
	ListActiveForEvent(ctx context.Context, tenantID uuid.UUID, event domain.EventType) ([]domain.Webhook, error)
	RecordSuccess(ctx context.Context, id uuid.UUID) error
	RecordFailure(ctx context.Context, id uuid.UUID, disableThreshold int) (disabled bool, err error)
}

// job is a single delivery attempt of an event to one webhook.
type job struct {
	webhook domain.Webhook
	event   domain.Event
	payload []byte
	attempt int
}

// Dispatcher fans events out to subscribed webhooks. Dispatch is non-blocking:
// events are queued and processed by background goroutines. Failed deliveries
// are retried with exponential backoff + jitter, and a webhook is dead-lettered
// after too many consecutive failures.
type Dispatcher struct {
	repo   Repository
	client *http.Client
	log    *slog.Logger

	events chan domain.Event
	jobs   chan job

	senders          int
	maxRetries       int
	disableThreshold int
	baseBackoff      time.Duration
	maxBackoff       time.Duration

	quit      chan struct{}
	wg        sync.WaitGroup
	startOnce sync.Once
	stopOnce  sync.Once

	dropped atomic.Int64
}

// NewDispatcher builds a Dispatcher from webhook config.
func NewDispatcher(cfg config.WebhookConfig, repo Repository, log *slog.Logger) *Dispatcher {
	disableThreshold := cfg.DisableAfter
	if disableThreshold < 1 {
		disableThreshold = 10
	}
	return &Dispatcher{
		repo:             repo,
		client:           &http.Client{Timeout: cfg.Timeout},
		log:              log,
		events:           make(chan domain.Event, cfg.QueueSize),
		jobs:             make(chan job, cfg.QueueSize),
		senders:          cfg.Workers,
		maxRetries:       cfg.MaxRetries,
		disableThreshold: disableThreshold,
		baseBackoff:      cfg.BaseBackoff,
		maxBackoff:       cfg.MaxBackoff,
		quit:             make(chan struct{}),
	}
}

// Start launches the resolver and sender goroutines.
func (d *Dispatcher) Start() {
	d.startOnce.Do(func() {
		d.wg.Add(1)
		go d.resolveLoop()
		for i := 0; i < d.senders; i++ {
			d.wg.Add(1)
			go d.sendLoop()
		}
		d.log.Info("webhook dispatcher started", "senders", d.senders, "max_retries", d.maxRetries)
	})
}

// Dispatch queues an event for delivery to all current subscribers. It never
// blocks: if the queue is full the event is dropped and counted. It stamps a
// fresh id/timestamp when absent.
func (d *Dispatcher) Dispatch(event domain.Event) bool {
	if event.ID == uuid.Nil {
		event.ID = uuid.New()
	}
	if event.OccurredAt.IsZero() {
		event.OccurredAt = time.Now()
	}

	select {
	case <-d.quit:
		return false
	default:
	}
	select {
	case d.events <- event:
		return true
	default:
		d.dropped.Add(1)
		d.log.Warn("webhook event dropped: queue full", "type", event.Type, "tenant_id", event.TenantID)
		return false
	}
}

// resolveLoop turns events into per-subscriber delivery jobs.
func (d *Dispatcher) resolveLoop() {
	defer d.wg.Done()
	for {
		select {
		case event := <-d.events:
			d.resolve(event)
		case <-d.quit:
			// Drain any queued events before exiting.
			for {
				select {
				case event := <-d.events:
					d.resolve(event)
				default:
					return
				}
			}
		}
	}
}

func (d *Dispatcher) resolve(event domain.Event) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	hooks, err := d.repo.ListActiveForEvent(ctx, event.TenantID, event.Type)
	if err != nil {
		d.log.Error("resolve webhook subscribers failed", "error", err, "type", event.Type)
		return
	}
	if len(hooks) == 0 {
		return
	}

	payload, err := json.Marshal(event)
	if err != nil {
		d.log.Error("marshal webhook payload failed", "error", err, "type", event.Type)
		return
	}
	for _, w := range hooks {
		d.enqueueJob(job{webhook: w, event: event, payload: payload, attempt: 0})
	}
}

// enqueueJob offers a job to the sender pool without blocking.
func (d *Dispatcher) enqueueJob(j job) {
	select {
	case <-d.quit:
		return
	default:
	}
	select {
	case d.jobs <- j:
	default:
		d.dropped.Add(1)
		d.log.Warn("webhook job dropped: queue full", "webhook_id", j.webhook.ID)
	}
}

// sendLoop delivers jobs from the queue.
func (d *Dispatcher) sendLoop() {
	defer d.wg.Done()
	for {
		select {
		case j := <-d.jobs:
			d.deliver(j)
		case <-d.quit:
			for {
				select {
				case j := <-d.jobs:
					d.deliver(j)
				default:
					return
				}
			}
		}
	}
}

// deliver attempts one HTTP POST. On success it clears the failure counter; on
// failure it schedules a retry (backoff + jitter) or, once retries are
// exhausted, records the failure (which may dead-letter the webhook).
func (d *Dispatcher) deliver(j job) {
	err := d.post(j)
	if err == nil {
		bg, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if serr := d.repo.RecordSuccess(bg, j.webhook.ID); serr != nil {
			d.log.Warn("record webhook success failed", "error", serr, "webhook_id", j.webhook.ID)
		}
		return
	}

	if j.attempt < d.maxRetries {
		delay := d.backoff(j.attempt)
		d.log.Warn("webhook delivery failed; scheduling retry",
			"error", err, "webhook_id", j.webhook.ID, "attempt", j.attempt+1, "delay", delay)
		retry := j
		retry.attempt++
		time.AfterFunc(delay, func() { d.enqueueJob(retry) })
		return
	}

	// Retries exhausted: record a failed delivery, possibly dead-lettering.
	bg, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	disabled, ferr := d.repo.RecordFailure(bg, j.webhook.ID, d.disableThreshold)
	if ferr != nil {
		d.log.Error("record webhook failure failed", "error", ferr, "webhook_id", j.webhook.ID)
		return
	}
	d.log.Error("webhook delivery permanently failed",
		"webhook_id", j.webhook.ID, "url", j.webhook.URL, "dead_lettered", disabled)
}

// post performs a single signed HTTP POST, returning an error for transport
// failures or non-2xx responses.
func (d *Dispatcher) post(j job) error {
	ctx, cancel := context.WithTimeout(context.Background(), d.client.Timeout+time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, j.webhook.URL, bytes.NewReader(j.payload))
	if err != nil {
		return fmt.Errorf("build webhook request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set(SignatureHeader, Sign(j.webhook.Secret, j.payload))
	req.Header.Set(TimestampHeader, strconv.FormatInt(j.event.OccurredAt.Unix(), 10))
	req.Header.Set(EventHeader, string(j.event.Type))
	req.Header.Set(IDHeader, j.event.ID.String())

	resp, err := d.client.Do(req)
	if err != nil {
		return fmt.Errorf("webhook post: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("webhook endpoint returned %d", resp.StatusCode)
	}
	return nil
}

// backoff returns the delay before the given (zero-based) retry attempt using
// exponential growth capped at maxBackoff, with full jitter.
func (d *Dispatcher) backoff(attempt int) time.Duration {
	delay := d.baseBackoff << attempt
	if delay <= 0 || delay > d.maxBackoff {
		delay = d.maxBackoff
	}
	// Full jitter: sleep a random duration in [delay/2, delay].
	half := delay / 2
	return half + time.Duration(rand.Int64N(int64(half)+1))
}

// Shutdown stops accepting events and waits for in-flight work to finish,
// bounded by ctx. Retries already scheduled for the future are abandoned.
func (d *Dispatcher) Shutdown(ctx context.Context) error {
	d.stopOnce.Do(func() { close(d.quit) })

	done := make(chan struct{})
	go func() {
		d.wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		d.log.Info("webhook dispatcher drained", "dropped", d.dropped.Load())
		return nil
	case <-ctx.Done():
		d.log.Warn("webhook dispatcher shutdown deadline exceeded")
		return ctx.Err()
	}
}

// Dropped returns the number of events/jobs dropped due to full queues.
func (d *Dispatcher) Dropped() int64 { return d.dropped.Load() }
