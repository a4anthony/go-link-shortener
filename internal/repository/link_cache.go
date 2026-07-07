package repository

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"

	"github.com/a4anthony/go-link-shortener/internal/domain"
)

// negativeMarker is the cached value that means "no live link for this code".
// Caching misses blunts cache-penetration attacks from floods of bogus codes.
const negativeMarker = "\x00NEG"

// LinkCache is a Redis-backed cache for the redirect hot path. It stores a
// compact snapshot of a link keyed by code, plus short-lived negative markers.
type LinkCache struct {
	rdb    *redis.Client
	ttl    time.Duration
	negTTL time.Duration
}

// NewLinkCache builds a LinkCache. ttl is the positive-entry lifetime; negTTL is
// the (shorter) negative-entry lifetime.
func NewLinkCache(rdb *redis.Client, ttl, negTTL time.Duration) *LinkCache {
	return &LinkCache{rdb: rdb, ttl: ttl, negTTL: negTTL}
}

func cacheKey(code string) string { return "link:" + code }

// cachedLink is the compact on-wire representation stored in Redis. It carries
// exactly what the redirect path and downstream analytics need.
type cachedLink struct {
	ID           string     `json:"id"`
	TenantID     string     `json:"tenant_id"`
	Code         string     `json:"code"`
	TargetURL    string     `json:"target_url"`
	RedirectType int        `json:"redirect_type"`
	ExpiresAt    *time.Time `json:"expires_at,omitempty"`
	MaxClicks    *int64     `json:"max_clicks,omitempty"`
	ClickCount   int64      `json:"click_count"`
}

// Get looks up a code. The bool reports whether the cache had an authoritative
// answer: (nil, false, nil) is a miss (consult the DB); (nil, true, nil) is a
// negative hit (the link does not exist); (link, true, nil) is a positive hit.
// A Redis error is returned with found=false so the caller falls back to the DB.
func (c *LinkCache) Get(ctx context.Context, code string) (*domain.Link, bool, error) {
	val, err := c.rdb.Get(ctx, cacheKey(code)).Result()
	if errors.Is(err, redis.Nil) {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, fmt.Errorf("cache get: %w", err)
	}
	if val == negativeMarker {
		return nil, true, nil
	}

	var cl cachedLink
	if err := json.Unmarshal([]byte(val), &cl); err != nil {
		// Corrupt entry: treat as a miss so the DB can repopulate it.
		return nil, false, fmt.Errorf("cache decode: %w", err)
	}
	link, err := cl.toDomain()
	if err != nil {
		return nil, false, err
	}
	return link, true, nil
}

// Set caches a link snapshot for the positive TTL.
func (c *LinkCache) Set(ctx context.Context, l *domain.Link) error {
	payload, err := json.Marshal(fromDomain(l))
	if err != nil {
		return fmt.Errorf("cache encode: %w", err)
	}
	if err := c.rdb.Set(ctx, cacheKey(l.Code), payload, c.ttl).Err(); err != nil {
		return fmt.Errorf("cache set: %w", err)
	}
	return nil
}

// SetNegative caches a not-found marker for the negative TTL.
func (c *LinkCache) SetNegative(ctx context.Context, code string) error {
	if err := c.rdb.Set(ctx, cacheKey(code), negativeMarker, c.negTTL).Err(); err != nil {
		return fmt.Errorf("cache set negative: %w", err)
	}
	return nil
}

// Invalidate removes a code's cache entry (called on update/delete).
func (c *LinkCache) Invalidate(ctx context.Context, code string) error {
	if err := c.rdb.Del(ctx, cacheKey(code)).Err(); err != nil {
		return fmt.Errorf("cache invalidate: %w", err)
	}
	return nil
}

func fromDomain(l *domain.Link) cachedLink {
	return cachedLink{
		ID:           l.ID.String(),
		TenantID:     l.TenantID.String(),
		Code:         l.Code,
		TargetURL:    l.TargetURL,
		RedirectType: l.RedirectType,
		ExpiresAt:    l.ExpiresAt,
		MaxClicks:    l.MaxClicks,
		ClickCount:   l.ClickCount,
	}
}

func (cl cachedLink) toDomain() (*domain.Link, error) {
	id, err := uuid.Parse(cl.ID)
	if err != nil {
		return nil, fmt.Errorf("cache decode id: %w", err)
	}
	tenantID, err := uuid.Parse(cl.TenantID)
	if err != nil {
		return nil, fmt.Errorf("cache decode tenant id: %w", err)
	}
	return &domain.Link{
		ID:           id,
		TenantID:     tenantID,
		Code:         cl.Code,
		TargetURL:    cl.TargetURL,
		RedirectType: cl.RedirectType,
		ExpiresAt:    cl.ExpiresAt,
		MaxClicks:    cl.MaxClicks,
		ClickCount:   cl.ClickCount,
	}, nil
}
