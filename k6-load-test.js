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
  },

  thresholds: {
    // Global
    http_req_failed: ['rate<0.10'],

    // Per-scenario response times
    'http_req_duration{scenario:health_check}': ['p(95)<500'],
    'http_req_duration{scenario:onu_list}': ['p(95)<60000'],
    'http_req_duration{scenario:onu_detail}': ['p(95)<30000'],
    'http_req_duration{scenario:pagination}': ['p(95)<60000'],

    // Custom
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
  const response = http.get(`${BASE_URL}/health`, {
    tags: { name: 'health' },
  });

  check(response, {
    'health: status 200': (r) => r.status === 200,
    'health: body ok': (r) => r.body && r.body.includes('"ok"'),
  });

  sleep(1);
}

export function onuList() {
  const board = randomBoard();
  const pon = randomPon();
  const url = `${BASE_URL}/api/v1/board/${board}/pon/${pon}`;

  const response = http.get(url, params({ name: `onu_list_b${board}_p${pon}` }));
  checkResponse(response, 'onu_list');

  // Validate response structure
  if (response.status === 200) {
    check(response, {
      'onu_list: has code field': (r) => JSON.parse(r.body).code === 200,
      'onu_list: has data array': (r) => Array.isArray(JSON.parse(r.body).data),
    });
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
      'onu_detail: has onu_type': (r) => {
        const body = JSON.parse(r.body);
        return body.data && body.data.onu_type !== undefined;
      },
      'onu_detail: has rx_power': (r) => {
        const body = JSON.parse(r.body);
        return body.data && body.data.rx_power !== undefined;
      },
    });
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
  } else if (response.status !== 404) {
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

// ─── Default function (fallback) ─────────────────────────

export default function () {
  onuList();
}

// ─── Summary ─────────────────────────────────────────────

export function handleSummary(data) {
  const duration = data.metrics.http_req_duration?.values || {};
  const reqs = data.metrics.http_reqs?.values || {};

  console.log('\n========================================');
  console.log('  LOAD TEST SUMMARY');
  console.log('========================================');
  console.log(`Total Requests:  ${reqs.count || 0}`);
  console.log(`Request Rate:    ${(reqs.rate || 0).toFixed(2)} req/s`);
  console.log(`Failed:          ${(data.metrics.http_req_failed?.values?.passes || 0)}`);
  console.log(`Custom Errors:   ${data.metrics.custom_errors?.values?.count || 0}`);
  console.log(`Timeouts:        ${data.metrics.custom_timeouts?.values?.count || 0}`);
  console.log('');
  console.log('Response Times:');
  console.log(`  Min:   ${(duration.min || 0).toFixed(0)}ms`);
  console.log(`  Avg:   ${(duration.avg || 0).toFixed(0)}ms`);
  console.log(`  p(95): ${(duration['p(95)'] || 0).toFixed(0)}ms`);
  console.log(`  p(99): ${(duration['p(99)'] || 0).toFixed(0)}ms`);
  console.log(`  Max:   ${(duration.max || 0).toFixed(0)}ms`);
  console.log('');

  const cacheRate = data.metrics.cache_hit_rate?.values?.rate;
  if (cacheRate !== undefined) {
    console.log(`Cache Hit Rate:  ${(cacheRate * 100).toFixed(1)}%`);
  }

  const snmp = data.metrics.snmp_duration?.values;
  if (snmp) {
    console.log(`SNMP Avg:        ${snmp.avg.toFixed(0)}ms`);
    console.log(`SNMP p(95):      ${snmp['p(95)'].toFixed(0)}ms`);
  }

  console.log('========================================\n');

  // Return default k6 summary output (don't override stdout)
  return {};
}
