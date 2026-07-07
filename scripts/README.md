# Load testing

Two equivalent load tests for the redirect hot path (`GET /:code`). Both measure
the redirect response itself (they do **not** follow the `Location`), so the
numbers reflect this service, not the target site.

## Prerequisites

Start the stack and grab the dev API key printed in the server logs:

```bash
docker compose up --build
# look for: "dev seed ready — use this API key to authenticate" api_key=sk_live_dev...
```

## k6 (preferred)

```bash
# Create a link:
curl -s -X POST http://localhost:8080/api/v1/links \
  -H "Authorization: Bearer sk_live_demo-seed-key" \
  -H "Content-Type: application/json" \
  -d '{"url":"https://example.com"}'

# Run against the returned code:
k6 run -e CODE=<code> scripts/loadtest.js
```

The k6 script asserts the spec target of **p99 < 50ms** for the redirect path.

## hey (shell alternative)

```bash
scripts/loadtest.sh [requests] [concurrency]   # defaults: 50000 200
```

It creates the link for you and prints hey's latency histogram (p50/p95/p99).
