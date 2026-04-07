import http from 'k6/http';
import { check, sleep, group } from 'k6';
import { Rate, Trend, Counter } from 'k6/metrics';

// Custom metrics
const errorRate = new Rate('error_rate');
const rateLimited = new Counter('rate_limited');
const reqDuration = new Trend('req_duration', true);

const BASE_URL = __ENV.BASE_URL || 'http://localhost:8081';
const API_KEY = __ENV.API_KEY || '';

// Load test stages
export const options = {
  stages: [
    // Warm-up
    { duration: '10s', target: 10 },
    // Ramp-up
    { duration: '20s', target: 50 },
    // Sustained load
    { duration: '30s', target: 50 },
    // Spike test
    { duration: '10s', target: 100 },
    // Sustained spike
    { duration: '20s', target: 100 },
    // Cool-down
    { duration: '10s', target: 0 },
  ],
  thresholds: {
    http_req_duration: ['p(95)<5000', 'p(99)<10000'],
    error_rate: ['rate<0.05'],  // real errors (not 429)
  },
};

function getHeaders() {
  const headers = { 'Content-Type': 'application/json' };
  if (API_KEY) {
    headers['X-API-Key'] = API_KEY;
  }
  return headers;
}

// 429 is expected under load (rate limiter), not a real error
function isSuccess(status) { return status === 200 || status === 429; }
function isRealError(status) { return status >= 400 && status !== 429; }

function trackResponse(res, name) {
  const realError = isRealError(res.status);
  errorRate.add(realError);
  reqDuration.add(res.timings.duration);
  if (res.status === 429) rateLimited.add(1);
}

// Random board/pon/onu generators
function randomBoard() { return Math.random() < 0.5 ? 1 : 2; }
function randomPon() { return Math.floor(Math.random() * 16) + 1; }
function randomOnu() { return Math.floor(Math.random() * 50) + 1; }

export default function () {
  const headers = getHeaders();
  const params = { headers, timeout: '60s' };

  group('Health Check', function () {
    const res = http.get(`${BASE_URL}/health`, params);
    check(res, {
      'health: status 200 or 429': (r) => isSuccess(r.status),
    });
    trackResponse(res);
  });

  group('Get ONUs by Board/PON', function () {
    const boardID = randomBoard();
    const ponID = randomPon();
    const res = http.get(`${BASE_URL}/api/v1/board/${boardID}/pon/${ponID}/`, params);
    check(res, {
      'onu-list: success': (r) => isSuccess(r.status),
      'onu-list: has data': (r) => {
        if (r.status === 429) return true; // skip check on rate limit
        try { return r.json('data') !== null; } catch(e) { return false; }
      },
      'onu-list: response < 5s': (r) => r.timings.duration < 5000,
    });
    trackResponse(res);
  });

  group('Get ONU Detail', function () {
    const boardID = randomBoard();
    const ponID = randomPon();
    const onuID = randomOnu();
    const res = http.get(`${BASE_URL}/api/v1/board/${boardID}/pon/${ponID}/onu/${onuID}`, params);
    check(res, {
      'onu-detail: success': (r) => isSuccess(r.status),
      'onu-detail: response < 10s': (r) => r.timings.duration < 10000,
    });
    trackResponse(res);
  });

  group('Get ONUs Paginated', function () {
    const boardID = randomBoard();
    const ponID = randomPon();
    const page = Math.floor(Math.random() * 3) + 1;
    const res = http.get(`${BASE_URL}/api/v1/paginate/board/${boardID}/pon/${ponID}/?page=${page}&page_size=10`, params);
    check(res, {
      'paginate: success': (r) => isSuccess(r.status),
      'paginate: response < 5s': (r) => r.timings.duration < 5000,
    });
    trackResponse(res);
  });

  group('Get Empty ONU IDs', function () {
    const boardID = randomBoard();
    const ponID = randomPon();
    const res = http.get(`${BASE_URL}/api/v1/board/${boardID}/pon/${ponID}/onu_id/empty`, params);
    check(res, {
      'empty-onu: success': (r) => isSuccess(r.status),
      'empty-onu: response < 5s': (r) => r.timings.duration < 5000,
    });
    trackResponse(res);
  });

  group('Get ONU Serial Numbers', function () {
    const boardID = randomBoard();
    const ponID = randomPon();
    const res = http.get(`${BASE_URL}/api/v1/board/${boardID}/pon/${ponID}/onu_id_sn`, params);
    check(res, {
      'onu-sn: success': (r) => isSuccess(r.status),
      'onu-sn: response < 5s': (r) => r.timings.duration < 5000,
    });
    trackResponse(res);
  });

  group('Validation - Invalid Params', function () {
    const res1 = http.get(`${BASE_URL}/api/v1/board/0/pon/1/`, params);
    check(res1, {
      'invalid-board: returns 400 or 429': (r) => r.status === 400 || r.status === 429,
    });

    const res2 = http.get(`${BASE_URL}/api/v1/board/1/pon/99/`, params);
    check(res2, {
      'invalid-pon: returns 400 or 429': (r) => r.status === 400 || r.status === 429,
    });

    const res3 = http.get(`${BASE_URL}/api/v1/board/1/pon/1/onu/0`, params);
    check(res3, {
      'invalid-onu: returns 400 or 429': (r) => r.status === 400 || r.status === 429,
    });
  });

  sleep(0.1);
}
