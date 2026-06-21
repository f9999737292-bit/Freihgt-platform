import http from 'k6/http';
import { check, sleep } from 'k6';
import { defaultThresholds, tenantQuery } from './common.js';

export const options = {
  vus: 10,
  duration: '2m',
  thresholds: defaultThresholds,
};

export default function () {
  const events = http.get(tenantQuery('/api/v1/rfx-events'));
  check(events, { 'list rfx events 2xx': (r) => r.status >= 200 && r.status < 300 });

  const freight = http.get(tenantQuery('/api/v1/freight-requests'));
  check(freight, { 'list freight requests 2xx': (r) => r.status >= 200 && r.status < 300 });

  sleep(0.5);
}
