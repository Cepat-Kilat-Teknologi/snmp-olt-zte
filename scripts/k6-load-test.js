import http from 'k6/http';
import { check, sleep, group } from 'k6';
import { Rate, Trend, Counter } from 'k6/metrics';

// Custom metrics
const errorRate = new Rate('error_rate');
const rateLimited = new Counter('rate_limited');
const reqDuration = new Trend('req_duration', true);

const BASE_URL = __ENV.BASE_URL || 'http://localhost:8081';
const API_KEY = __ENV.API_KEY || '';

// Multi-OLT: pass the SAME OLTS JSON the server uses (-e OLTS='[...]') and the
// test targets /api/v1/olt/{id}/board/... with each OLT's valid board/pon
// ranges. Empty OLTS → single-OLT bare /api/v1/board/... paths.
const OLT_TARGETS = parseOltTargets(__ENV.OLTS || '');

function parseBoardsSpec(spec, defPon) {
  return String(spec || '1,2')
    .split(',')
    .map((s) => {
      const [b, p] = s.split(':');
      const board = parseInt(b, 10);
      const ponMax = p ? parseInt(p, 10) : defPon;
      return { board, ponMax };
    })
    .filter((t) => Number.isInteger(t.board) && t.board > 0 && t.ponMax > 0);
}

function parseOltTargets(js) {
  if (!js.trim()) return [];
  try {
    const arr = JSON.parse(js);
    if (!Array.isArray(arr)) return [];
    return arr
      .filter((o) => o && o.id)
      .map((o) => ({ id: o.id, boards: parseBoardsSpec(o.boards, o.ponsPerBoard || 16) }));
  } catch (e) {
    return [];
  }
}

// pickTarget returns { prefix, board, pon } — an OLT-scoped path prefix and a
// board/pon valid for that OLT (single-OLT fallback when OLTS is empty).
function pickTarget() {
  if (OLT_TARGETS.length === 0) {
    return { prefix: '', board: randomBoard(), pon: randomPon() };
  }
  const t = OLT_TARGETS[Math.floor(Math.random() * OLT_TARGETS.length)];
  const card = t.boards[Math.floor(Math.random() * t.boards.length)];
  return {
    prefix: `/olt/${t.id}`,
    board: card.board,
    pon: Math.floor(Math.random() * card.ponMax) + 1,
    key: OLT_KEYS[t.id] || API_KEY,
  };
}

function apiBase(prefix) {
  return `${BASE_URL}/api/v1${prefix || ''}`;
}

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

// Optional per-tenant API keys for OLT-scoped requests: JSON {oltId: apiKey}.
// Set when the server runs with per-tenant API_USERS; falls back to API_KEY.
const OLT_KEYS = (() => {
  const raw = __ENV.OLT_KEYS || '';
  if (!raw.trim()) return {};
  try { const o = JSON.parse(raw); return o && typeof o === 'object' ? o : {}; } catch (e) { return {}; }
})();

function getHeaders(key) {
  const headers = { 'Content-Type': 'application/json' };
  const apiKey = key || API_KEY;
  if (apiKey) {
    headers['X-API-Key'] = apiKey;
  }
  return headers;
}

// 429 is expected under load (rate limiter), not a real error
// 429 (rate limited) and 404 are EXPECTED outcomes under load, not real errors:
// 429 = the global rate limiter doing its job; 404 = a valid board/pon that
// simply has no ONUs (common on sparsely-populated / lab OLTs). Only other 4xx/5xx
// (e.g. 400 validation, 401 auth, 500) count as real errors.
function isSuccess(status) { return status === 200 || status === 429 || status === 404; }
function isRealError(status) { return status >= 400 && status !== 429 && status !== 404; }

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

    // Multi-OLT: /readyz must expose a per-OLT probe snmp_<id> for each OLT.
    if (OLT_TARGETS.length > 0) {
      const ready = http.get(`${BASE_URL}/readyz`, params);
      check(ready, {
        'readyz: 200 or 503': (r) => r.status === 200 || r.status === 503,
        'readyz: per-OLT snmp probes present': (r) => {
          try {
            const deps = JSON.parse(r.body).dependencies || {};
            return OLT_TARGETS.every((t) => deps[`snmp_${t.id}`] !== undefined);
          } catch (e) { return false; }
        },
      });
      trackResponse(ready);
    }
  });

  group('Get ONUs by Board/PON', function () {
    const { prefix, board, pon, key } = pickTarget();
    const res = http.get(`${apiBase(prefix)}/board/${board}/pon/${pon}/`, { headers: getHeaders(key), timeout: '60s' });
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
    const { prefix, board, pon, key } = pickTarget();
    const onuID = randomOnu();
    const res = http.get(`${apiBase(prefix)}/board/${board}/pon/${pon}/onu/${onuID}`, { headers: getHeaders(key), timeout: '60s' });
    check(res, {
      'onu-detail: success': (r) => isSuccess(r.status),
      'onu-detail: response < 10s': (r) => r.timings.duration < 10000,
    });
    trackResponse(res);
  });

  group('Get ONUs Paginated', function () {
    const { prefix, board, pon, key } = pickTarget();
    const page = Math.floor(Math.random() * 3) + 1;
    const res = http.get(`${apiBase(prefix)}/paginate/board/${board}/pon/${pon}/?page=${page}&page_size=10`, { headers: getHeaders(key), timeout: '60s' });
    check(res, {
      'paginate: success': (r) => isSuccess(r.status),
      'paginate: response < 5s': (r) => r.timings.duration < 5000,
    });
    trackResponse(res);
  });

  group('Get Empty ONU IDs', function () {
    const { prefix, board, pon, key } = pickTarget();
    const res = http.get(`${apiBase(prefix)}/board/${board}/pon/${pon}/onu_id/empty`, { headers: getHeaders(key), timeout: '60s' });
    check(res, {
      'empty-onu: success': (r) => isSuccess(r.status),
      'empty-onu: response < 5s': (r) => r.timings.duration < 5000,
    });
    trackResponse(res);
  });

  group('Get ONU Serial Numbers', function () {
    const { prefix, board, pon, key } = pickTarget();
    const res = http.get(`${apiBase(prefix)}/board/${board}/pon/${pon}/onu_id_sn`, { headers: getHeaders(key), timeout: '60s' });
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

    // Multi-OLT: an unknown OLT id must 404.
    if (OLT_TARGETS.length > 0) {
      const res4 = http.get(`${BASE_URL}/api/v1/olt/__nope__/board/1/pon/1/`, params);
      check(res4, {
        'unknown-olt: returns 404 or 429': (r) => r.status === 404 || r.status === 429,
      });
    }
  });

  sleep(0.1);
}
