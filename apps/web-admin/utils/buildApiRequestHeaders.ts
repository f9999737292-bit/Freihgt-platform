import {
  API_HEADER_AUTHORIZATION,
  API_HEADER_REQUEST_ID,
  API_HEADER_TENANT_ID,
} from '~/constants/apiHeaders'

/** Headers allowed by services/api-gateway/internal/http/middleware/cors.go */
export const GATEWAY_CORS_ALLOWED_REQUEST_HEADERS = [
  'content-type',
  'authorization',
  'x-tenant-id',
  'x-company-id',
  'x-request-id',
  'x-locale',
] as const

export interface ApiRequestHeaderInput {
  token?: string | null
  tenantId?: string | null
  companyId?: string | null
  locale?: string
  requestId?: string
  extra?: Record<string, string>
  skipAuth?: boolean
  skipTenant?: boolean
}

export function buildApiRequestHeaders(input: ApiRequestHeaderInput = {}): Record<string, string> {
  const headers: Record<string, string> = {
    Accept: 'application/json',
    'Content-Type': 'application/json',
    [API_HEADER_REQUEST_ID]: input.requestId ?? crypto.randomUUID(),
    'X-Locale': input.locale ?? 'ru-RU',
    ...input.extra,
  }

  if (!input.skipAuth && input.token) {
    headers[API_HEADER_AUTHORIZATION] = `Bearer ${input.token}`
  }
  if (!input.skipTenant && input.tenantId) {
    headers[API_HEADER_TENANT_ID] = input.tenantId
  }
  if (input.companyId) {
    headers['X-Company-ID'] = input.companyId
  }

  return headers
}

export function corsPreflightRequestHeaderNames(headers: Record<string, string>): string[] {
  return Object.keys(headers)
    .map((name) => name.toLowerCase())
    .filter((name) => name !== 'accept')
    .sort()
}

export function headersAcceptedByGatewayCors(headers: Record<string, string>): boolean {
  const names = corsPreflightRequestHeaderNames(headers)
  return names.every((name) =>
    (GATEWAY_CORS_ALLOWED_REQUEST_HEADERS as readonly string[]).includes(name),
  )
}
