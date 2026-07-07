#!/usr/bin/env bash
# hey-based load test for the redirect hot path (alternative to k6/loadtest.js).
#
# Requires: hey (https://github.com/rakyll/hey) and a running stack
# (`docker compose up`). It creates a link with the dev API key, then hammers
# GET /:code and reports the latency distribution.
#
# Usage: scripts/loadtest.sh [requests] [concurrency]
set -euo pipefail

BASE_URL="${BASE_URL:-http://localhost:8080}"
API_KEY="${API_KEY:-sk_live_demo-seed-key}"
REQUESTS="${1:-50000}"
CONCURRENCY="${2:-200}"

command -v hey >/dev/null 2>&1 || { echo "hey is not installed: go install github.com/rakyll/hey@latest"; exit 1; }

echo "Creating a short link..."
CODE=$(curl -sf -X POST "${BASE_URL}/api/v1/links" \
  -H "Authorization: Bearer ${API_KEY}" \
  -H "Content-Type: application/json" \
  -d '{"url":"https://example.com"}' | sed -n 's/.*"code":"\([^"]*\)".*/\1/p')

if [[ -z "${CODE}" ]]; then
  echo "Failed to create link. Is the stack up and the API key correct?" >&2
  exit 1
fi
echo "Created link with code: ${CODE}"
echo "Load testing GET ${BASE_URL}/${CODE} ..."

# -disable-redirects so we measure the 302 itself, not the followed target.
hey -n "${REQUESTS}" -c "${CONCURRENCY}" -disable-redirects "${BASE_URL}/${CODE}"
