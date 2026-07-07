// Package metrics defines the Prometheus collectors for the service and adapts
// them to the observer interfaces the service/analytics layers expect. A single
// Metrics value is registered once at startup and shared read-only.
package metrics

import (
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

// Metrics bundles every collector the service exports on /metrics.
type Metrics struct {
	httpDuration *prometheus.HistogramVec
	httpRequests *prometheus.CounterVec

	redirects   *prometheus.CounterVec
	cacheHits   prometheus.Counter
	cacheMisses prometheus.Counter

	analyticsEnqueued  prometheus.Counter
	analyticsDropped   prometheus.Counter
	analyticsFlushed   prometheus.Counter
	analyticsQueueDepth prometheus.Gauge
}

// New registers all collectors on reg and returns the Metrics facade. Passing a
// dedicated registry (rather than the global one) keeps tests isolated.
func New(reg prometheus.Registerer) *Metrics {
	f := promauto.With(reg)
	return &Metrics{
		httpDuration: f.NewHistogramVec(prometheus.HistogramOpts{
			Name:    "http_request_duration_seconds",
			Help:    "HTTP request latency by method, route, and status.",
			Buckets: prometheus.DefBuckets,
		}, []string{"method", "route", "status"}),
		httpRequests: f.NewCounterVec(prometheus.CounterOpts{
			Name: "http_requests_total",
			Help: "Total HTTP requests by method, route, and status.",
		}, []string{"method", "route", "status"}),
		redirects: f.NewCounterVec(prometheus.CounterOpts{
			Name: "redirects_total",
			Help: "Redirect outcomes by result (served, not_found, gone, error).",
		}, []string{"result"}),
		cacheHits: f.NewCounter(prometheus.CounterOpts{
			Name: "redirect_cache_hits_total",
			Help: "Redirect cache hits (positive and negative).",
		}),
		cacheMisses: f.NewCounter(prometheus.CounterOpts{
			Name: "redirect_cache_misses_total",
			Help: "Redirect cache misses that fell back to Postgres.",
		}),
		analyticsEnqueued: f.NewCounter(prometheus.CounterOpts{
			Name: "analytics_events_enqueued_total",
			Help: "Click events accepted into the analytics pipeline.",
		}),
		analyticsDropped: f.NewCounter(prometheus.CounterOpts{
			Name: "analytics_events_dropped_total",
			Help: "Click events dropped because the pipeline buffer was full.",
		}),
		analyticsFlushed: f.NewCounter(prometheus.CounterOpts{
			Name: "analytics_events_flushed_total",
			Help: "Click events durably written to Postgres.",
		}),
		analyticsQueueDepth: f.NewGauge(prometheus.GaugeOpts{
			Name: "analytics_queue_depth",
			Help: "Current number of buffered, unprocessed click events.",
		}),
	}
}

// ObserveHTTP records one HTTP request's latency and outcome.
func (m *Metrics) ObserveHTTP(method, route, status string, dur time.Duration) {
	m.httpDuration.WithLabelValues(method, route, status).Observe(dur.Seconds())
	m.httpRequests.WithLabelValues(method, route, status).Inc()
}

// ObserveRedirect counts a redirect outcome.
func (m *Metrics) ObserveRedirect(result string) {
	m.redirects.WithLabelValues(result).Inc()
}

// ObserveCacheHit implements service.CacheObserver.
func (m *Metrics) ObserveCacheHit() { m.cacheHits.Inc() }

// ObserveCacheMiss implements service.CacheObserver.
func (m *Metrics) ObserveCacheMiss() { m.cacheMisses.Inc() }

// IncEnqueued implements analytics.Observer.
func (m *Metrics) IncEnqueued() { m.analyticsEnqueued.Inc() }

// IncDropped implements analytics.Observer.
func (m *Metrics) IncDropped() { m.analyticsDropped.Inc() }

// IncFlushed implements analytics.Observer.
func (m *Metrics) IncFlushed(n int) { m.analyticsFlushed.Add(float64(n)) }

// SetQueueDepth records the current analytics queue depth.
func (m *Metrics) SetQueueDepth(n int) { m.analyticsQueueDepth.Set(float64(n)) }
