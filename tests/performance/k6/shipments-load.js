import http from 'k6/http';
import { check, sleep } from 'k6';
import { defaultThresholds, tenantQuery } from './common.js';

export const options = {
  vus: 10,
  duration: '2m',
  thresholds: defaultThresholds,
};

const shipmentId = __ENV.SHIPMENT_ID || '';

export default function () {
  const list = http.get(tenantQuery('/api/v1/shipments'));
  check(list, { 'list shipments 2xx': (r) => r.status >= 200 && r.status < 300 });

  if (shipmentId) {
    const get = http.get(tenantQuery(`/api/v1/shipments/${shipmentId}`));
    check(get, { 'get shipment 2xx': (r) => r.status >= 200 && r.status < 300 });
  }

  sleep(0.5);
}
