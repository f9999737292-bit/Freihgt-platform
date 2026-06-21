import http from 'k6/http';
import { check, sleep } from 'k6';
import { defaultThresholds, tenantQuery, apiUrl, TENANT_ID } from './common.js';

export const options = {
  vus: 10,
  duration: '2m',
  thresholds: defaultThresholds,
};

const companyId = __ENV.COMPANY_ID || '';
const createData = (__ENV.CREATE_DATA || 'false').toLowerCase() === 'true';

export default function () {
  const list = http.get(tenantQuery('/api/v1/companies'));
  check(list, { 'list companies 2xx': (r) => r.status >= 200 && r.status < 300 });

  if (companyId) {
    const get = http.get(tenantQuery(`/api/v1/companies/${companyId}`));
    check(get, { 'get company 2xx': (r) => r.status >= 200 && r.status < 300 });
  }

  if (createData) {
    const payload = JSON.stringify({
      tenant_id: TENANT_ID,
      legal_name: `Load Test Co ${__VU}-${__ITER}`,
      company_type: 'SHIPPER',
      country_code: 'RU',
    });
    const created = http.post(apiUrl('/api/v1/companies'), payload, {
      headers: { 'Content-Type': 'application/json' },
    });
    check(created, { 'create company 2xx': (r) => r.status >= 200 && r.status < 300 });
  }

  sleep(0.5);
}
