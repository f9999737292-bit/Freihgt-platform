import http from 'k6/http';
import { check, sleep } from 'k6';
import { loadThresholds, tenantQuery } from './common.js';

export const options = {
  vus: 20,
  duration: '5m',
  thresholds: loadThresholds,
};

export default function () {
  const endpoints = [
    { name: 'companies', url: tenantQuery('/api/v1/companies') },
    { name: 'transport orders', url: tenantQuery('/api/v1/transport-orders') },
    { name: 'freight requests', url: tenantQuery('/api/v1/freight-requests') },
    { name: 'shipments', url: tenantQuery('/api/v1/shipments') },
    { name: 'documents', url: tenantQuery('/api/v1/documents') },
    { name: 'billing registers', url: tenantQuery('/api/v1/billing-registers') },
  ];

  for (const endpoint of endpoints) {
    const res = http.get(endpoint.url);
    check(res, { [`${endpoint.name} 2xx`]: (r) => r.status >= 200 && r.status < 300 });
  }

  sleep(0.3);
}
