import http from 'k6/http';
import { check, sleep } from 'k6';
import { defaultThresholds, tenantQuery } from './common.js';

export const options = {
  vus: 10,
  duration: '2m',
  thresholds: defaultThresholds,
};

const billingRegisterId = __ENV.BILLING_REGISTER_ID || '';

export default function () {
  const list = http.get(tenantQuery('/api/v1/billing-registers'));
  check(list, { 'list billing registers 2xx': (r) => r.status >= 200 && r.status < 300 });

  if (billingRegisterId) {
    const get = http.get(tenantQuery(`/api/v1/billing-registers/${billingRegisterId}`));
    check(get, { 'get billing register 2xx': (r) => r.status >= 200 && r.status < 300 });
  }

  sleep(0.5);
}
