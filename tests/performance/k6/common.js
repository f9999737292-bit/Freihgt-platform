export const API_BASE_URL = __ENV.API_BASE_URL || 'http://localhost:8080';
export const TENANT_ID = __ENV.TENANT_ID || '74519f22-ff9b-4a8b-8fff-a958c689682f';

export const defaultThresholds = {
  http_req_failed: ['rate<0.05'],
  http_req_duration: ['p(95)<500'],
};

export const loadThresholds = {
  http_req_failed: ['rate<0.02'],
  http_req_duration: ['p(95)<1000', 'p(99)<2000'],
};

export function apiUrl(path) {
  const base = API_BASE_URL.replace(/\/$/, '');
  const suffix = path.startsWith('/') ? path : `/${path}`;
  return `${base}${suffix}`;
}

export function tenantQuery(path) {
  const separator = path.includes('?') ? '&' : '?';
  return `${apiUrl(path)}${separator}tenant_id=${TENANT_ID}`;
}

export function checkResponse(res, name) {
  return {
    [`${name} status 2xx`]: (r) => r.status >= 200 && r.status < 300,
  };
}
