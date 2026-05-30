/**
 * k6 load test — authz check endpoint
 *
 * Default scenario: ramp to 50 VUs over ~160 s, hitting
 *   GET /api/v1/domains/{domainID}/authz/check
 *
 * Environment variables:
 *   BASE_URL           Server base URL (default: http://127.0.0.1:8080)
 *   API_BEARER_TOKEN   Bearer token (leave empty if auth is disabled)
 *
 * Saturation sweep — override VUs via k6 --vus / --stage flags, e.g.:
 *   k6 run --vus 200 --duration 60s test/load/authz.js
 *   k6 run --vus 500 --duration 30s test/load/authz.js
 *
 * Soak test (30 min):
 *   k6 run --vus 50 --duration 30m test/load/authz.js
 *
 * Results: record numbers in test/load/RESULTS.md after each run.
 */

import http from 'k6/http';
import { check, fail } from 'k6';

const BASE_URL = (__ENV.BASE_URL || 'http://127.0.0.1:8080').replace(/\/$/, '');
const TOKEN = __ENV.API_BEARER_TOKEN || '';

// ---------------------------------------------------------------------------
// Scenario configuration
// ---------------------------------------------------------------------------

export const options = {
  stages: [
    { duration: '10s', target: 10 },  // ramp up
    { duration: '60s', target: 10 },  // steady low
    { duration: '20s', target: 50 },  // ramp to peak
    { duration: '60s', target: 50 },  // steady peak
    { duration: '10s', target: 0 },   // ramp down
  ],
  thresholds: {
    // 99th-percentile latency under 50 ms
    'http_req_duration{scenario:default}': ['p(99)<50'],
    // Error rate below 1%
    http_req_failed: ['rate<0.01'],
  },
};

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

function headers(withBody) {
  const h = {};
  if (TOKEN) {
    h['Authorization'] = `Bearer ${TOKEN}`;
  }
  if (withBody) {
    h['Content-Type'] = 'application/json';
  }
  return h;
}

function post(path, body) {
  const res = http.post(`${BASE_URL}${path}`, JSON.stringify(body), {
    headers: headers(true),
  });
  if (res.status !== 201) {
    fail(`setup: POST ${path} returned ${res.status}: ${res.body}`);
  }
  const id = JSON.parse(res.body).id;
  if (!id) {
    fail(`setup: POST ${path} response missing id field`);
  }
  return id;
}

function postNoBody(path) {
  const res = http.post(`${BASE_URL}${path}`, null, { headers: headers(false) });
  if (res.status !== 204) {
    fail(`setup: POST ${path} returned ${res.status}: ${res.body}`);
  }
}

// ---------------------------------------------------------------------------
// Setup — seed one domain / user / resource / permission and grant it.
// Runs once before VUs start; the returned object is passed to default().
// ---------------------------------------------------------------------------

export function setup() {
  const domainID = post('/api/v1/domains', { title: 'k6-bench-domain' });

  // Seed a pool of users so VUs spread load across different rows instead
  // of hammering a single hot user.
  const USER_POOL = 10;
  const userIDs = [];
  for (let i = 0; i < USER_POOL; i++) {
    const uid = post(`/api/v1/domains/${domainID}/users`, { title: `k6-bench-user-${i}` });
    userIDs.push(uid);
  }

  const resourceID = post(`/api/v1/domains/${domainID}/resources`, { title: 'k6-bench-resource' });
  const permID = post(`/api/v1/domains/${domainID}/permissions`, {
    title: 'k6-bench-perm',
    resource_id: resourceID,
    access_mask: '1',
  });

  // Grant the permission to every user.
  for (const uid of userIDs) {
    postNoBody(`/api/v1/domains/${domainID}/users/${uid}/permissions/${permID}`);
  }

  return { domainID, userIDs, resourceID, permID, bit: '1' };
}

// ---------------------------------------------------------------------------
// Default function — called once per VU per iteration
// ---------------------------------------------------------------------------

export default function (data) {
  const { domainID, userIDs, resourceID, bit } = data;
  // Each VU picks a different user from the pool to avoid a single hot row.
  const userID = userIDs[(__VU - 1) % userIDs.length];
  const url =
    `${BASE_URL}/api/v1/domains/${domainID}/authz/check` +
    `?user_id=${userID}&resource_id=${resourceID}&access_bit=${bit}`;

  const res = http.get(url, { headers: headers(false) });

  check(res, {
    'status 200': (r) => r.status === 200,
    'allowed true': (r) => {
      try {
        return JSON.parse(r.body).allowed === true;
      } catch (_) {
        return false;
      }
    },
  });
}

// ---------------------------------------------------------------------------
// Teardown — delete seeded entities in FK-safe order.
// Runs once after all VUs finish.
// ---------------------------------------------------------------------------

function del(path) {
  const res = http.del(`${BASE_URL}${path}`, null, { headers: headers(false) });
  if (res.status !== 204 && res.status !== 404) {
    console.warn(`teardown: DELETE ${path} returned ${res.status}`);
  }
}

export function teardown(data) {
  const { domainID, userIDs, resourceID, permID } = data;

  // Revoke grants first (FK: user_permissions references users and permissions).
  for (const uid of userIDs) {
    del(`/api/v1/domains/${domainID}/users/${uid}/permissions/${permID}`);
  }
  // Delete the permission (FK: permissions references resources).
  del(`/api/v1/domains/${domainID}/permissions/${permID}`);
  // Delete users.
  for (const uid of userIDs) {
    del(`/api/v1/domains/${domainID}/users/${uid}`);
  }
  // Delete resource.
  del(`/api/v1/domains/${domainID}/resources/${resourceID}`);
  // Delete domain.
  del(`/api/v1/domains/${domainID}`);
}
