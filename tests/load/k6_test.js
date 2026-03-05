import http from 'k6/http';
import { check, sleep } from 'k6';
import { Rate, Trend } from 'k6/metrics';

const errorRate = new Rate('errors');
const apiLatency = new Trend('api_latency', true);

export const options = {
  stages: [
    { duration: '30s', target: 10 },
    { duration: '1m',  target: 50 },
    { duration: '2m',  target: 100 },
    { duration: '30s', target: 0 },
  ],
  thresholds: {
    http_req_duration: ['p(95)<500', 'p(99)<1000'],
    errors: ['rate<0.01'],
  },
};

const BASE_URL = __ENV.BASE_URL || 'http://localhost:8080';

export function setup() {
  const res = http.post(`${BASE_URL}/api/v1/auth/register`, JSON.stringify({
    email: `load-test-${Date.now()}@example.com`,
    password: 'LoadTest123!',
    name: 'Load Test User',
  }), { headers: { 'Content-Type': 'application/json' } });

  const cookies = {};
  if (res.cookies) {
    for (const [name, values] of Object.entries(res.cookies)) {
      if (values.length > 0) {
        cookies[name] = values[0].value;
      }
    }
  }
  return { cookies };
}

export default function (data) {
  const jar = http.cookieJar();
  const url = `${BASE_URL}`;

  if (data.cookies) {
    for (const [name, value] of Object.entries(data.cookies)) {
      jar.set(url, name, value);
    }
  }

  // Health check
  const healthRes = http.get(`${BASE_URL}/health`);
  check(healthRes, { 'health ok': (r) => r.status === 200 });
  apiLatency.add(healthRes.timings.duration);
  errorRate.add(healthRes.status !== 200);

  // Auth check
  const authRes = http.get(`${BASE_URL}/api/v1/auth/check`);
  apiLatency.add(authRes.timings.duration);
  errorRate.add(authRes.status !== 200 && authRes.status !== 401);

  // Metrics endpoint
  const metricsRes = http.get(`${BASE_URL}/metrics`);
  check(metricsRes, { 'metrics ok': (r) => r.status === 200 });
  apiLatency.add(metricsRes.timings.duration);

  sleep(0.5);
}
