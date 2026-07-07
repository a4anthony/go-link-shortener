package repository

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

// slidingWindowScript implements an atomic sliding-window-log limiter. It trims
// entries older than the window, counts the rest, and — if under the limit —
// records the new request. It returns {allowed, remaining, retryAfterMillis}.
//
// KEYS[1] = bucket key
// ARGV[1] = now (unix millis)
// ARGV[2] = window (millis)
// ARGV[3] = limit
// ARGV[4] = unique member for this request
var slidingWindowScript = redis.NewScript(`
local key    = KEYS[1]
local now    = tonumber(ARGV[1])
local window = tonumber(ARGV[2])
local limit  = tonumber(ARGV[3])
local member = ARGV[4]

redis.call('ZREMRANGEBYSCORE', key, 0, now - window)
local count = redis.call('ZCARD', key)

if count < limit then
    redis.call('ZADD', key, now, member)
    redis.call('PEXPIRE', key, window)
    return {1, limit - count - 1, 0}
end

local oldest = redis.call('ZRANGE', key, 0, 0, 'WITHSCORES')
local retry = window
if oldest[2] ~= nil then
    retry = (tonumber(oldest[2]) + window) - now
    if retry < 0 then retry = 0 end
end
return {0, 0, retry}
`)

// RedisRateLimiter is a per-key sliding-window rate limiter backed by Redis.
type RedisRateLimiter struct {
	rdb *redis.Client
}

// NewRedisRateLimiter builds a RedisRateLimiter.
func NewRedisRateLimiter(rdb *redis.Client) *RedisRateLimiter {
	return &RedisRateLimiter{rdb: rdb}
}

// Allow records a request against key and reports whether it is permitted within
// limit requests per window. When denied, retryAfter is the time until the
// oldest in-window request expires.
func (l *RedisRateLimiter) Allow(ctx context.Context, key string, limit int, window time.Duration) (allowed bool, remaining int, retryAfter time.Duration, err error) {
	member, err := uniqueMember()
	if err != nil {
		return false, 0, 0, err
	}

	now := time.Now().UnixMilli()
	res, err := slidingWindowScript.Run(ctx, l.rdb,
		[]string{"ratelimit:" + key},
		now, window.Milliseconds(), limit, member,
	).Int64Slice()
	if err != nil {
		return false, 0, 0, fmt.Errorf("rate limit eval: %w", err)
	}
	if len(res) != 3 {
		return false, 0, 0, fmt.Errorf("rate limit script returned %d values, want 3", len(res))
	}

	allowed = res[0] == 1
	remaining = int(res[1])
	retryAfter = time.Duration(res[2]) * time.Millisecond
	return allowed, remaining, retryAfter, nil
}

// uniqueMember returns a collision-free ZSET member so simultaneous requests at
// the same millisecond are each counted.
func uniqueMember() (string, error) {
	buf := make([]byte, 8)
	if _, err := rand.Read(buf); err != nil {
		return "", fmt.Errorf("rate limit member: %w", err)
	}
	return fmt.Sprintf("%d-%s", time.Now().UnixNano(), hex.EncodeToString(buf)), nil
}
