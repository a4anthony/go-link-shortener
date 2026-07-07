package metrics

import (
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// gatherValue sums the samples for a metric family by name from the registry.
func gatherValue(t *testing.T, reg *prometheus.Registry, name string) float64 {
	t.Helper()
	families, err := reg.Gather()
	require.NoError(t, err)
	var total float64
	for _, mf := range families {
		if mf.GetName() != name {
			continue
		}
		for _, m := range mf.GetMetric() {
			switch {
			case m.Counter != nil:
				total += m.Counter.GetValue()
			case m.Gauge != nil:
				total += m.Gauge.GetValue()
			case m.Histogram != nil:
				total += float64(m.Histogram.GetSampleCount())
			}
		}
	}
	return total
}

func TestMetrics_ObserversRecord(t *testing.T) {
	reg := prometheus.NewRegistry()
	m := New(reg)

	m.ObserveCacheHit()
	m.ObserveCacheHit()
	m.ObserveCacheMiss()
	m.IncEnqueued()
	m.IncDropped()
	m.IncFlushed(5)
	m.SetQueueDepth(42)
	m.ObserveRedirect("served")
	m.ObserveHTTP("GET", "/x", "200", 10*time.Millisecond)

	assert.Equal(t, float64(2), gatherValue(t, reg, "redirect_cache_hits_total"))
	assert.Equal(t, float64(1), gatherValue(t, reg, "redirect_cache_misses_total"))
	assert.Equal(t, float64(1), gatherValue(t, reg, "analytics_events_enqueued_total"))
	assert.Equal(t, float64(1), gatherValue(t, reg, "analytics_events_dropped_total"))
	assert.Equal(t, float64(5), gatherValue(t, reg, "analytics_events_flushed_total"))
	assert.Equal(t, float64(42), gatherValue(t, reg, "analytics_queue_depth"))
	assert.Equal(t, float64(1), gatherValue(t, reg, "redirects_total"))
	assert.Equal(t, float64(1), gatherValue(t, reg, "http_requests_total"))
}

func TestNew_IsolatedRegistry(t *testing.T) {
	// Two independent registries can each hold their own Metrics without a
	// duplicate-registration panic.
	assert.NotPanics(t, func() {
		New(prometheus.NewRegistry())
		New(prometheus.NewRegistry())
	})
}
