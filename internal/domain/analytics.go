package domain

import (
	"time"

	"github.com/google/uuid"
)

// Bucket is a time-series aggregation granularity.
type Bucket string

const (
	BucketHour Bucket = "hour"
	BucketDay  Bucket = "day"
)

// Valid reports whether b is a supported bucket.
func (b Bucket) Valid() bool {
	return b == BucketHour || b == BucketDay
}

// TimePoint is one bucketed count in a click time-series.
type TimePoint struct {
	Time  time.Time `json:"time"`
	Count int64     `json:"count"`
}

// LabelCount is a labelled aggregate (e.g. referrer -> count).
type LabelCount struct {
	Label string `json:"label"`
	Count int64  `json:"count"`
}

// LinkStats is the analytics rollup for a single link over a time window.
type LinkStats struct {
	LinkID       uuid.UUID    `json:"link_id"`
	From         time.Time    `json:"from"`
	To           time.Time    `json:"to"`
	Bucket       Bucket       `json:"bucket"`
	TotalClicks  int64        `json:"total_clicks"`
	Series       []TimePoint  `json:"series"`
	TopReferrers []LabelCount `json:"top_referrers"`
	TopCountries []LabelCount `json:"top_countries"`
	Devices      []LabelCount `json:"devices"`
}

// TenantOverview is a tenant-wide analytics summary.
type TenantOverview struct {
	TotalLinks  int64        `json:"total_links"`
	TotalClicks int64        `json:"total_clicks"`
	TopLinks    []LabelCount `json:"top_links"`
}
