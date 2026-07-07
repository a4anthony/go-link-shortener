// k6 load test for the redirect hot path.
//
// Usage:
//   1. Start the stack:            docker compose up --build
//   2. Create a link (grab the dev API key from the server logs):
//        curl -s -X POST http://localhost:8080/api/v1/links \
//          -H "Authorization: Bearer sk_live_demo-seed-key" \
//          -H "Content-Type: application/json" \
//          -d '{"url":"https://example.com"}'
//   3. Run the test with the returned code:
//        k6 run -e CODE=<code> scripts/loadtest.js
//
// Reports p50/p95/p99 latency for GET /:code, the redirect hot path.

import http from 'k6/http';
import { check } from 'k6';

const BASE = __ENV.BASE_URL || 'http://localhost:8080';
const CODE = __ENV.CODE || 'demo';

export const options = {
  scenarios: {
    redirect: {
      executor: 'ramping-vus',
      startVUs: 0,
      stages: [
        { duration: '10s', target: 50 },
        { duration: '30s', target: 200 },
        { duration: '10s', target: 0 },
      ],
    },
  },
  thresholds: {
    // Local target from the spec: p99 < 50ms on the redirect hot path.
    http_req_duration: ['p(50)<10', 'p(95)<30', 'p(99)<50'],
    checks: ['rate>0.99'],
  },
};

export default function () {
  // redirects=0 so we measure the 302 itself, not the followed target.
  const res = http.get(`${BASE}/${CODE}`, { redirects: 0 });
  check(res, {
    'is redirect': (r) => r.status === 301 || r.status === 302,
    'has location': (r) => !!r.headers['Location'],
  });
}
