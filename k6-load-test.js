import http from 'k6/http';
import { check, group, sleep } from 'k6';
import { Counter, Trend, Rate } from 'k6/metrics';

// Custom metrics
const errors = new Counter('custom_errors');
const timeouts = new Counter('custom_timeouts');
const cacheHitRate = new Rate('cache_hit_rate');
const snmpDuration = new Trend('snmp_duration');

// Configuration from environment variables
const BASE_URL = __ENV.BASE_URL || 'http://localhost:8081';
const API_KEY = __ENV.API_KEY || '';

// Common request params
function params(tags) {
  const p = {
    timeout: '120s',
    tags: tags || {},
    headers: {
      'Accept': 'application/json',
    },
  };
  if (API_KEY) {
    p.headers['X-API-Key'] = API_KEY;
  }
  return p;
}

// Test scenarios
export const options = {
  // Compute extra percentiles in the summary; without this, p(99) is not
  // calculated for trend metrics (http_req_duration etc.) and the
  // handleSummary helper falls back to N/A.
  summaryTrendStats: ['min', 'med', 'avg', 'p(90)', 'p(95)', 'p(99)', 'max'],

  scenarios: {
    // Scenario 1: Health check (always fast, baseline)
    health_check: {
      executor: 'constant-vus',
      vus: 1,
      duration: '3m30s',
      exec: 'healthCheck',
    },

    // Scenario 2: ONU list (main workload, SNMP + cache)
    onu_list: {
      executor: 'ramping-vus',
      startVUs: 0,
      stages: [
        { duration: '30s', target: 4 },
        { duration: '1m', target: 4 },
        { duration: '30s', target: 8 },
        { duration: '1m', target: 8 },
        { duration: '30s', target: 0 },
      ],
      gracefulRampDown: '30s',
      exec: 'onuList',
    },

    // Scenario 3: ONU detail (single ONU, lighter SNMP)
    onu_detail: {
      executor: 'ramping-vus',
      startVUs: 0,
      stages: [
        { duration: '30s', target: 2 },
        { duration: '2m', target: 2 },
        { duration: '30s', target: 0 },
      ],
      gracefulRampDown: '10s',
      exec: 'onuDetail',
    },

    // Scenario 4: Pagination (cached data, should be fast)
    pagination: {
      executor: 'constant-vus',
      vus: 2,
      duration: '3m',
      startTime: '30s',
      exec: 'paginatedList',
    },

    // Scenario 5: Mixed operations (empty IDs, serial numbers, cache ops)
    // Note: Each iteration involves multiple sequential SNMP calls (~30-40s each)
    mixed_ops: {
      executor: 'per-vu-iterations',
      vus: 1,
      iterations: 3,
      maxDuration: '5m',
      startTime: '30s',
      exec: 'mixedOperations',
    },

    // Scenario 6: Contract conformance — deliberately hit invalid inputs to
    // exercise the v3.0.0 error envelope (error_code, data, request_id).
    // Very cheap — just makes sure the contract doesn't regress under load.
    contract_check: {
      executor: 'constant-vus',
      vus: 1,
      duration: '2m',
      startTime: '30s',
      exec: 'contractCheck',
    },
  },

  thresholds: {
    // Per-scenario failure rates.
    //
    // We deliberately do NOT set a global `http_req_failed` threshold here:
    // contract_check intentionally generates 400/404 responses to verify
    // the v3.0.0 error envelope, and onu_list/onu_detail random board+pon
    // combinations naturally produce some 404s (no ONUs on that PON).
    // A global threshold would false-alarm on those expected failures.
    //
    // Scenario-scoped thresholds let us tune each workload independently:
    //   - health_check: any failure is real (probes should never error)
    //   - onu_list/onu_detail: allow some 404s (random board/pon hit empty PONs)
    //   - pagination: same as onu_list
    //   - contract_check: ALL responses are intentional 4xx — no threshold
    'http_req_failed{scenario:health_check}': ['rate<0.01'],
    'http_req_failed{scenario:onu_list}':     ['rate<0.30'],
    'http_req_failed{scenario:onu_detail}':   ['rate<0.40'],
    // pagination randomly picks page 1-5; pages 3-5 often empty for the
    // small board/pon test data → up to 60% legitimate 404s.
    'http_req_failed{scenario:pagination}':   ['rate<0.70'],
    'http_req_failed{scenario:mixed_ops}':    ['rate<0.30'],

    // Per-scenario response times
    'http_req_duration{scenario:health_check}': ['p(95)<500'],
    'http_req_duration{scenario:onu_list}': ['p(95)<60000'],
    'http_req_duration{scenario:onu_detail}': ['p(95)<30000'],
    'http_req_duration{scenario:pagination}': ['p(95)<60000'],
    'http_req_duration{scenario:contract_check}': ['p(95)<500'],

    // Custom — track real check failures (validateErrorEnvelope mismatches,
    // missing fields, etc.) rather than HTTP status alone.
    custom_errors: ['count<30'],
  },
};

// ─── Helpers ─────────────────────────────────────────────

function randomBoard() {
  return Math.random() < 0.5 ? 1 : 2;
}

function randomPon() {
  return Math.floor(Math.random() * 16) + 1;
}

function randomOnu() {
  return Math.floor(Math.random() * 10) + 1;
}

// validateErrorEnvelope asserts the v3.0.0 error response format. Used when
// a request returns 4xx so we can catch regressions in the response contract.
// Reference: isp-adapter-standard wiki.
function validateErrorEnvelope(response, name) {
  check(response, {
    [`${name} error: valid json`]: (r) => {
      try { JSON.parse(r.body); return true; } catch { return false; }
    },
    [`${name} error: has error_code`]: (r) => {
      try {
        const body = JSON.parse(r.body);
        return typeof body.error_code === 'string' && body.error_code.length > 0;
      } catch { return false; }
    },
    [`${name} error: has data`]: (r) => {
      try {
        return JSON.parse(r.body).data !== undefined;
      } catch { return false; }
    },
    [`${name} error: has request_id`]: (r) => {
      try {
        const body = JSON.parse(r.body);
        return typeof body.request_id === 'string' && body.request_id.length > 0;
      } catch { return false; }
    },
  });
}

function checkResponse(response, name) {
  const ok = check(response, {
    [`${name}: status 200`]: (r) => r.status === 200,
    [`${name}: has body`]: (r) => r.body && r.body.length > 0,
    [`${name}: valid json`]: (r) => {
      try { JSON.parse(r.body); return true; } catch { return false; }
    },
  });

  if (!ok) {
    errors.add(1);
    if (response.status === 408 || response.timings.duration > 90000) {
      timeouts.add(1);
    }
  }

  // Track if response was likely from cache (fast) or SNMP (slow)
  if (response.status === 200) {
    const isCacheHit = response.timings.duration < 100;
    cacheHitRate.add(isCacheHit);
    if (!isCacheHit) {
      snmpDuration.add(response.timings.duration);
    }
  }

  return ok;
}

// ─── Scenario Functions ──────────────────────────────────

export function healthCheck() {
  // Cycle through the full set of unauthenticated probe/metadata endpoints
  // so the k6 run exercises everything the agent integration standard adds:
  //   - /health    (legacy liveness)
  //   - /healthz   (k8s-style liveness)
  //   - /readyz    (readiness with dependency probes)
  //   - /version   (build metadata)
  //   - /metrics   (Prometheus scrape)
  // These are all cheap so we can keep a 1s pause between them.

  const health = http.get(`${BASE_URL}/health`, { tags: { name: 'health' } });
  check(health, {
    'health: status 200': (r) => r.status === 200,
    'health: body healthy': (r) => r.body && r.body.includes('"healthy"'),
  });

  const healthz = http.get(`${BASE_URL}/healthz`, { tags: { name: 'healthz' } });
  check(healthz, {
    'healthz: status 200': (r) => r.status === 200,
    'healthz: body healthy': (r) => r.body && r.body.includes('"healthy"'),
  });

  const readyz = http.get(`${BASE_URL}/readyz`, { tags: { name: 'readyz' } });
  check(readyz, {
    // readyz is allowed to return 503 when a dependency is degraded — both
    // 200 and 503 are expected outcomes, so we assert on one of them.
    'readyz: status 200 or 503': (r) => r.status === 200 || r.status === 503,
    'readyz: has status field': (r) => {
      try {
        const body = JSON.parse(r.body);
        return body.status === 'ready' || body.status === 'not_ready';
      } catch {
        return false;
      }
    },
  });

  const version = http.get(`${BASE_URL}/version`, { tags: { name: 'version' } });
  check(version, {
    'version: status 200': (r) => r.status === 200,
    'version: has version field': (r) => {
      try {
        const body = JSON.parse(r.body);
        return typeof body.version === 'string' && body.version.length > 0;
      } catch {
        return false;
      }
    },
    'version: has api_version field': (r) => {
      try {
        return JSON.parse(r.body).api_version === 'v1';
      } catch {
        return false;
      }
    },
  });

  const metrics = http.get(`${BASE_URL}/metrics`, { tags: { name: 'metrics' } });
  check(metrics, {
    'metrics: status 200': (r) => r.status === 200,
    'metrics: has http_requests_total': (r) => r.body && r.body.includes('http_requests_total'),
  });

  // X-API-Version headers should be present on every response. Check one.
  check(version, {
    'version: has X-API-Version header': (r) => r.headers['X-Api-Version'] === 'v1',
    'version: has X-App-Version header': (r) =>
      r.headers['X-App-Version'] && r.headers['X-App-Version'].length > 0,
  });

  sleep(1);
}

export function onuList() {
  const board = randomBoard();
  const pon = randomPon();
  const url = `${BASE_URL}/api/v1/board/${board}/pon/${pon}`;

  const response = http.get(url, params({ name: `onu_list_b${board}_p${pon}` }));
  checkResponse(response, 'onu_list');

  // Validate the v3.0.0 success envelope: {code, status:"success", data:[...]}
  if (response.status === 200) {
    check(response, {
      'onu_list: has code field': (r) => JSON.parse(r.body).code === 200,
      'onu_list: status = success': (r) => JSON.parse(r.body).status === 'success',
      'onu_list: has data array': (r) => Array.isArray(JSON.parse(r.body).data),
    });
  } else if (response.status >= 400 && response.status < 500) {
    // Validate the v3.0.0 error envelope: {code, status, error_code, data, request_id}
    validateErrorEnvelope(response, 'onu_list');
  }

  sleep(1);
}

export function onuDetail() {
  const board = randomBoard();
  const pon = randomPon();
  const onu = randomOnu();
  const url = `${BASE_URL}/api/v1/board/${board}/pon/${pon}/onu/${onu}`;

  const response = http.get(url, params({ name: `onu_detail_b${board}_p${pon}_o${onu}` }));
  checkResponse(response, 'onu_detail');

  // Validate detailed response fields
  if (response.status === 200) {
    check(response, {
      'onu_detail: status = success': (r) => JSON.parse(r.body).status === 'success',
      'onu_detail: has onu_type': (r) => {
        const body = JSON.parse(r.body);
        return body.data && body.data.onu_type !== undefined;
      },
      'onu_detail: has rx_power': (r) => {
        const body = JSON.parse(r.body);
        return body.data && body.data.rx_power !== undefined;
      },
    });
  } else if (response.status === 404) {
    validateErrorEnvelope(response, 'onu_detail');
  }

  sleep(1);
}

export function paginatedList() {
  const board = randomBoard();
  const pon = randomPon();
  const page = Math.floor(Math.random() * 5) + 1;
  const limit = [5, 10, 20, 50][Math.floor(Math.random() * 4)];
  const url = `${BASE_URL}/api/v1/paginate/board/${board}/pon/${pon}?page=${page}&limit=${limit}`;

  const response = http.get(url, params({ name: `paginate_b${board}_p${pon}` }));

  if (response.status === 200) {
    check(response, {
      'paginate: status = success': (r) => JSON.parse(r.body).status === 'success',
      'paginate: has meta': (r) => {
        const body = JSON.parse(r.body);
        return body.meta && body.meta.page !== undefined;
      },
      'paginate: has data': (r) => {
        const body = JSON.parse(r.body);
        return body.data !== undefined;
      },
      'paginate: limit respected': (r) => {
        const body = JSON.parse(r.body);
        return !body.data || body.data.length <= limit;
      },
    });
  } else if (response.status === 404) {
    validateErrorEnvelope(response, 'paginate');
  } else {
    errors.add(1);
  }

  sleep(0.5);
}

export function mixedOperations() {
  const board = randomBoard();
  const pon = randomPon();

  group('mixed_ops', () => {
    // 1. Get empty ONU IDs
    group('empty_onu_ids', () => {
      const url = `${BASE_URL}/api/v1/board/${board}/pon/${pon}/onu_id/empty`;
      const response = http.get(url, params({ name: 'empty_onu_ids' }));
      checkResponse(response, 'empty_onu_ids');
      sleep(0.5);
    });

    // 2. Get ONU IDs with serial numbers
    group('onu_id_sn', () => {
      const url = `${BASE_URL}/api/v1/board/${board}/pon/${pon}/onu_id_sn`;
      const response = http.get(url, params({ name: 'onu_id_sn' }));
      checkResponse(response, 'onu_id_sn');
      sleep(0.5);
    });

    // 3. Update empty ONU ID cache (POST)
    group('update_empty_onu', () => {
      const url = `${BASE_URL}/api/v1/board/${board}/pon/${pon}/onu_id/update`;
      const response = http.post(url, null, params({ name: 'update_empty_onu' }));
      checkResponse(response, 'update_empty_onu');
      sleep(0.5);
    });

    // 4. Delete cache then re-fetch (cache miss → SNMP)
    group('cache_invalidation', () => {
      const deleteUrl = `${BASE_URL}/api/v1/board/${board}/pon/${pon}/cache/clear`;
      const deleteResp = http.del(deleteUrl, null, params({ name: 'delete_cache' }));
      check(deleteResp, {
        'delete_cache: status 200': (r) => r.status === 200,
      });
      sleep(0.5);

      // Re-fetch to trigger SNMP (cache miss)
      const fetchUrl = `${BASE_URL}/api/v1/board/${board}/pon/${pon}`;
      const fetchResp = http.get(fetchUrl, params({ name: 'fetch_after_invalidation' }));
      checkResponse(fetchResp, 'fetch_after_invalidation');
      sleep(0.5);
    });
  });
}

// contractCheck hits endpoints with deliberately invalid parameters so we
// can validate the v3.0.0 error response envelope under load. Expected
// outcomes are 400 (validation error) for board/pon out of range and 404
// (resource not found) for board/pon combinations that have no ONUs.
export function contractCheck() {
  // Case 1: board_id out of range → 400 VALIDATION_ERROR
  const badBoard = http.get(`${BASE_URL}/api/v1/board/99/pon/1`, params({ name: 'bad_board' }));
  check(badBoard, {
    'bad_board: status 400': (r) => r.status === 400,
  });
  if (badBoard.status === 400) {
    validateErrorEnvelope(badBoard, 'bad_board');
    check(badBoard, {
      'bad_board: error_code = VALIDATION_ERROR': (r) =>
        JSON.parse(r.body).error_code === 'VALIDATION_ERROR',
    });
  }

  // Case 2: pon_id out of range → 400 VALIDATION_ERROR
  const badPon = http.get(`${BASE_URL}/api/v1/board/1/pon/99`, params({ name: 'bad_pon' }));
  check(badPon, {
    'bad_pon: status 400': (r) => r.status === 400,
  });
  if (badPon.status === 400) {
    validateErrorEnvelope(badPon, 'bad_pon');
  }

  // Case 3: onu_id out of range → 400 VALIDATION_ERROR
  const badOnu = http.get(`${BASE_URL}/api/v1/board/1/pon/1/onu/999`, params({ name: 'bad_onu' }));
  check(badOnu, {
    'bad_onu: status 400': (r) => r.status === 400,
  });
  if (badOnu.status === 400) {
    validateErrorEnvelope(badOnu, 'bad_onu');
  }

  // X-Request-ID should echo back on error responses — test that too.
  const customReqID = `k6-contract-${Date.now()}`;
  const withReqID = http.get(
    `${BASE_URL}/api/v1/board/99/pon/1`,
    {
      ...params({ name: 'req_id_echo' }),
      headers: {
        'X-Request-ID': customReqID,
        'Accept': 'application/json',
        ...(API_KEY && { 'X-API-Key': API_KEY }),
      },
    },
  );
  check(withReqID, {
    'req_id_echo: header X-Request-ID echoed': (r) => r.headers['X-Request-Id'] === customReqID,
    'req_id_echo: body request_id matches': (r) => {
      try {
        return JSON.parse(r.body).request_id === customReqID;
      } catch { return false; }
    },
  });

  sleep(1);
}

// ─── Default function (fallback) ─────────────────────────

export default function () {
  onuList();
}

// ─── Summary ─────────────────────────────────────────────

// metricVal pulls a metric out of k6's summary `data.metrics` map. k6 v0.x
// nested values under `.values`; k6 v1.x flattens them onto the metric node
// itself. Some metric types also rename fields between `--summary-export`
// JSON output and the runtime handleSummary input — for example Rate's
// runtime field is `rate` while the JSON export uses `value`. This helper
// transparently tries all known shapes so the summary keeps working across
// k6 upgrades and works in both contexts.
function metricVal(metric, ...keys) {
  if (!metric) return undefined;
  const sources = [metric.values, metric].filter(Boolean);
  for (const src of sources) {
    for (const key of keys) {
      if (src[key] !== undefined) return src[key];
    }
  }
  return undefined;
}

function fmt(n) {
  return typeof n === 'number' ? n.toFixed(0) : 'N/A';
}

export function handleSummary(data) {
  const m = data.metrics || {};
  const dur = m.http_req_duration || {};
  const reqs = m.http_reqs || {};

  // Per-scenario http_req_failed counts (intentional 4xx in contract_check
  // and natural 404s in onu_list/onu_detail/pagination are excluded from
  // "real" failures by our scenario-scoped thresholds).
  const realFails = (metricVal(m['http_req_failed{scenario:health_check}'], 'passes') || 0) +
                    (metricVal(m['http_req_failed{scenario:mixed_ops}'], 'passes') || 0);

  // Rate-type metric: runtime field is `rate`, JSON-export field is `value`.
  const failRate = metricVal(m.http_req_failed, 'rate', 'value') || 0;

  console.log('\n========================================');
  console.log('  LOAD TEST SUMMARY');
  console.log('========================================');
  console.log(`Total Requests:        ${metricVal(reqs, 'count') || 0}`);
  console.log(`Request Rate:          ${(metricVal(reqs, 'rate') || 0).toFixed(2)} req/s`);
  console.log(`Failed (4xx+5xx):      ${metricVal(m.http_req_failed, 'passes') || 0}  ` +
              `(${(failRate * 100).toFixed(1)}%, mostly intentional from contract_check)`);
  console.log(`Real Failures:         ${realFails}  (health_check + mixed_ops only)`);
  console.log(`Custom Errors:         ${metricVal(m.custom_errors, 'count') || 0}`);
  console.log(`Timeouts:              ${metricVal(m.custom_timeouts, 'count') || 0}`);
  console.log('');
  console.log('Response Times (all requests):');
  console.log(`  min   ${fmt(metricVal(dur, 'min'))}ms`);
  console.log(`  med   ${fmt(metricVal(dur, 'med'))}ms`);
  console.log(`  avg   ${fmt(metricVal(dur, 'avg'))}ms`);
  console.log(`  p(90) ${fmt(metricVal(dur, 'p(90)'))}ms`);
  console.log(`  p(95) ${fmt(metricVal(dur, 'p(95)'))}ms`);
  console.log(`  p(99) ${fmt(metricVal(dur, 'p(99)'))}ms`);
  console.log(`  max   ${fmt(metricVal(dur, 'max'))}ms`);
  console.log('');

  const cacheRate = metricVal(m.cache_hit_rate, 'rate', 'value');
  if (cacheRate !== undefined) {
    console.log(`Cache Hit Rate:        ${(cacheRate * 100).toFixed(1)}%`);
  }

  const snmp = m.snmp_duration;
  if (snmp) {
    console.log(`SNMP avg:              ${fmt(metricVal(snmp, 'avg'))}ms`);
    console.log(`SNMP p(95):            ${fmt(metricVal(snmp, 'p(95)'))}ms`);
    console.log(`SNMP max:              ${fmt(metricVal(snmp, 'max'))}ms`);
  }

  console.log('');
  console.log('Per-scenario p(95):');
  for (const scn of ['health_check', 'contract_check', 'onu_list', 'onu_detail', 'pagination', 'mixed_ops']) {
    const sdur = m[`http_req_duration{scenario:${scn}}`];
    if (sdur) {
      console.log(`  ${scn.padEnd(18)} avg=${fmt(metricVal(sdur, 'avg'))}ms ` +
                  `p(95)=${fmt(metricVal(sdur, 'p(95)'))}ms ` +
                  `max=${fmt(metricVal(sdur, 'max'))}ms`);
    }
  }
  console.log('========================================\n');

  // Return default k6 summary output (don't override stdout)
  return {};
}
