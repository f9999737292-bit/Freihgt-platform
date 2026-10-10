import { PortalClientError } from './errors'

/** Headers the browser must never send as authority. The gateway derives them from the JWT. */
export const FORBIDDEN_AUTHORITY_HEADERS = [
  'x-tenant-id',
  'x-user-id',
  'x-user-email',
  'x-user-role',
  'x-roles',
  'x-role',
  'x-actor-kind',
] as const

const FORBIDDEN = new Set<string>(FORBIDDEN_AUTHORITY_HEADERS)

export const COMPANY_SCOPE_FIELDS = [
  'carrier_company_id',
  'company_id',
  'participant_company_id',
  'shipper_company_id',
] as const

export interface PortalHeaderInput {
  accessToken?: string | null
  companyId?: string | null
  allowedCompanyIds: readonly string[]
  locale?: string | null
  requestId?: string | null
  extra?: Record<string, string | undefined | null>
}

export function assertNoAuthorityHeaders(headers: Record<string, string>): void {
  for (const key of Object.keys(headers)) {
    if (FORBIDDEN.has(key.toLowerCase())) {
      throw new PortalClientError(`refusing authority header ${key}`, 0, 'AUTHORITY_HEADER')
    }
  }
}

export function assertCompanyInMembership(companyId: string, allowedCompanyIds: readonly string[]): void {
  if (!allowedCompanyIds.includes(companyId)) {
    throw new PortalClientError(
      'company is not a server-returned carrier membership',
      403,
      'COMPANY_SPOOF',
    )
  }
}

export function buildPortalHeaders(input: PortalHeaderInput): Record<string, string> {
  if (input.extra) {
    assertNoAuthorityHeaders(
      Object.fromEntries(
        Object.entries(input.extra).filter((entry): entry is [string, string] => typeof entry[1] === 'string'),
      ),
    )
  }

  const headers: Record<string, string> = {
    Accept: 'application/json',
  }
  if (input.accessToken) {
    headers.Authorization = `Bearer ${input.accessToken}`
  }
  if (input.companyId) {
    assertCompanyInMembership(input.companyId, input.allowedCompanyIds)
    headers['X-Company-ID'] = input.companyId
  }
  if (input.locale) {
    headers['X-Locale'] = input.locale
  }
  headers['X-Request-ID'] = input.requestId?.trim() || crypto.randomUUID()
  assertNoAuthorityHeaders(headers)
  return headers
}

export function assertScopedCompanyFields(
  value: unknown,
  allowedCompanyIds: readonly string[],
): void {
  if (!value || typeof value !== 'object') return
  if (Array.isArray(value)) {
    for (const item of value) assertScopedCompanyFields(item, allowedCompanyIds)
    return
  }
  for (const [key, field] of Object.entries(value as Record<string, unknown>)) {
    if ((COMPANY_SCOPE_FIELDS as readonly string[]).includes(key) && typeof field === 'string' && field !== '') {
      assertCompanyInMembership(field, allowedCompanyIds)
    } else if (field && typeof field === 'object') {
      assertScopedCompanyFields(field, allowedCompanyIds)
    }
  }
}
