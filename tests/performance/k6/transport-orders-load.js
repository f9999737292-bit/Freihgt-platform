import http from 'k6/http';
import { check, sleep } from 'k6';
import { defaultThresholds, tenantQuery } from './common.js';

export const options = {
  vus: 10,
  duration: '2m',
  thresholds: defaultThresholds,
};

const transportOrderId = __ENV.TRANSPORT_ORDER_ID || '';

export default function () {
  const list = http.get(tenantQuery('/api/v1/transport-orders'));
  check(list, { 'list transport orders 2xx': (r) => r.status >= 200 && r.status < 300 });

  if (transportOrderId) {
    const get = http.get(tenantQuery(`/api/v1/transport-orders/${transportOrderId}`));
    check(get, { 'get transport order 2xx': (r) => r.status >= 200 && r.status < 300 });
  }

  sleep(0.5);
}
