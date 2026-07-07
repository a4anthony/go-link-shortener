# Design notes

This document covers the non-obvious trade-offs behind the service. The
higher-level tour lives in the [README](../README.md).

## Layering and dependency direction

`handler → service → repository`. Services own the business rules and **declare
the interfaces** for the persistence they need (e.g. `service.LinkRepository`,
`service.LinkCache`). The `repository` package provides concrete pgx/Redis
implementations that satisfy those interfaces structurally, and
`cmd/server/main.go` wires them together explicitly.

Consequences:

- The dependency graph is acyclic: `domain` is a leaf; `repository` and
  `service` both depend on `domain`, never on each other. `service` never
  imports `repository`.
- Every service is unit-testable with in-memory fakes (no DB), and the
  repositories are covered by testcontainers integration tests against real
  Postgres/Redis.
- Adding a backend (say, a different cache) means implementing an interface and
  changing one line in `main.go`.

## Redirect hot path

`GET /:code` is the path that must be fast. It:

1. Reads Redis (`link:<code>`). A **positive hit** returns immediately; a
   **negative hit** (a cached "not found" marker) returns 404 without touching
   Postgres.
2. On a miss, reads Postgres by code, **backfills** the cache, and serves.
3. Maps expired / click-exhausted links to **410 Gone**.

### 301 vs 302 and cache TTL

Redirect status is per link (default **302**). This matters because clients and
intermediaries cache **301 (permanent)** aggressively and often indefinitely —
great for throughput, bad if the destination ever needs to change or if you want
to keep counting clicks (a cached 301 never reaches us again). **302** keeps the
link under our control and keeps analytics complete, at the cost of not being
cached downstream. The default is 302 for correctness and analytics; operators
can opt into 301 per link for static, never-changing destinations.

Our own Redis TTL is a separate axis. Entries use a moderate positive TTL
(default 1h) and a short negative TTL (default 30s). The short negative TTL
bounds how long a bogus/again-valid code stays "not found" while still absorbing
floods of junk codes. Because mutations and exhaustion **invalidate** the entry
explicitly, TTL is a backstop for correctness, not the primary consistency
mechanism — so it can be generous without risking long-lived stale reads.

### Fail-open

If Redis is unavailable, the redirect path logs and falls back to Postgres. A
degraded cache should slow the system, not break it.

### Click-count consistency under caching

`click_count` is incremented asynchronously by the analytics pipeline, so a
cached link's count can briefly lag. To keep the **max-clicks / 410** behaviour
correct, the pipeline detects when a batch pushes a link to its limit and
**invalidates that link's cache entry**, forcing the next redirect to re-read
Postgres and observe the exhaustion. The window is bounded by the flush interval
(default 2s), a deliberate trade of exact-at-the-instant enforcement for a
non-blocking hot path.

## Analytics: buffered-channel batching over per-click inserts

A naive design writes one row per click synchronously. That couples redirect
latency to database write latency and collapses under load. Instead:

- The redirect handler builds a lightweight `ClickEvent` (link, tenant, time,
  referrer, raw UA, client IP) and **enqueues** it on a buffered channel. This is
  the only work on the hot path.
- A **worker pool** drains the channel. Each worker accumulates a local batch and
  flushes on **size or time** (whichever comes first) using a single Postgres
  `COPY`, which is dramatically cheaper than N inserts.
- **Enrichment happens on the workers, not the hot path**: user-agent parsing,
  geo lookup, and IP anonymisation all run off the redirect path.

### Backpressure: drop, never block

`Enqueue` is non-blocking. If the buffer is full it **drops the event and
increments a counter** (`analytics_events_dropped_total`) rather than blocking
the redirect. Losing a sampled analytics event under extreme load is acceptable;
adding latency (or failing) on the redirect is not. The queue depth is exported
as a gauge so saturation is observable before it matters.

### Graceful shutdown

On `SIGTERM` the server stops accepting HTTP, then drains the analytics channel
and the webhook queue within a deadline before closing pools — so in-flight
clicks and deliveries are flushed rather than lost on deploys.

## Privacy

Raw IPs are never stored. The pipeline truncates the client IP (zeroes the last
IPv4 octet / last 80 IPv6 bits) and stores a **salted SHA-256** of the truncated
value. Geo resolution uses the full IP transiently but only the country code is
persisted. The geo resolver is an interface with a no-op default and a
MaxMind-compatible adapter.

## Multi-tenancy and API keys

Every repository query is tenant-scoped: methods take a `tenant_id` and put it in
the `WHERE` clause, so a resource belonging to another tenant is
indistinguishable from a missing one (`404`/`ErrNotFound`, never `403` — we don't
confirm existence across tenants). The single exception is the public redirect
lookup, which is by globally-unique code and has no tenant context by design.

API keys carry an identifiable prefix and are **hashed at rest** (SHA-256);
authentication hashes the presented token and looks it up by an indexed hash
column, so verification is O(1) and the plaintext is shown only once at creation.

## Rate limiting

Per-tenant limiting uses a Redis **sliding-window log** implemented as an atomic
Lua script (trim by score, count, conditionally add). This gives accurate
windowing without the boundary bursts of fixed windows, in a single round trip.
Denied requests get `429` with `Retry-After` and `X-RateLimit-*` headers. The
middleware **fails open** on a limiter backend error.

## Webhooks

- **Signing.** Each delivery is signed with HMAC-SHA256 over the raw body using
  the webhook's secret (`X-Webhook-Signature: sha256=…`); receivers recompute and
  constant-time compare.
- **Delivery.** Dispatch is non-blocking (queued, resolved and sent by background
  goroutines). Failures retry with **exponential backoff + full jitter**, capped;
  jitter avoids thundering-herd retries against a recovering endpoint.
- **Dead-lettering.** After enough consecutive failed deliveries a webhook is
  disabled (`active = false`, `disabled_at` set) and skipped by the dispatch
  lookup — a bad endpoint stops consuming retry budget indefinitely.
- **`link.clicked` is batched**, emitted per link from each analytics flush rather
  than once per click, so a hot link doesn't generate a webhook storm.

## Testing strategy

- **Unit tests** with in-memory fakes cover services, middleware, the shortcode
  generator, the analytics pipeline (batching, drop-on-full, drain-on-shutdown),
  and the rate-limit/webhook logic.
- **Integration tests** (build tag `integration`, testcontainers) exercise the
  repositories against real Postgres/Redis: the cross-tenant isolation proof, the
  redirect cache backfill/invalidation flow, the full analytics flow, the sliding
  window, and webhook dead-lettering.
- **Benchmarks** cover the redirect service on cache-hit and cache-miss paths.
