import http from 'k6/http';
import { check, sleep } from 'k6';
import { API_BASE_URL, TENANT_ID, defaultThresholds, tenantQuery, apiUrl } from './common.js';

export const options = {
  vus: 1,
  duration: '30s',
  thresholds: defaultThresholds,
};

export default function () {
  const health = http.get(`${API_BASE_URL.replace(/\/$/, '')}/health`);
  check(health, { 'gateway health status 2xx': (r) => r.status >= 200 && r.status < 300 });

  const ready = http.get(`${API_BASE_URL.replace(/\/$/, '')}/ready`);
  check(ready, {
    'gateway ready status 2xx or 503': (r) =>
      (r.status >= 200 && r.status < 300) || r.status === 503,
  });

  const endpoints = [
    tenantQuery('/api/v1/companies'),
    tenantQuery('/api/v1/transport-orders'),
    tenantQuery('/api/v1/shipments'),
    tenantQuery('/api/v1/billing-registers'),
  ];

  for (const url of endpoints) {
    const res = http.get(url);
    check(res, { [`${url} status 2xx`]: (r) => r.status >= 200 && r.status < 300 });
  }

  sleep(1);
}
