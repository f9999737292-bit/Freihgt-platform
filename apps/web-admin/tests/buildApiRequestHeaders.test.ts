import { describe, expect, it } from 'vitest'
import {
  GATEWAY_CORS_ALLOWED_REQUEST_HEADERS,
  buildApiRequestHeaders,
  corsPreflightRequestHeaderNames,
  headersAcceptedByGatewayCors,
} from '../utils/buildApiRequestHeaders'

describe('buildApiRequestHeaders', () => {
  it('sends Authorization and tenant context without client X-User-ID', () => {
    const headers = buildApiRequestHeaders({
      token: 'proof-token',
      tenantId: '285f9447-faf7-423e-96dd-e4c5e2b3fc6c',
      companyId: '83cb2447-75e9-41f2-8e0d-93c70f8506be',
      locale: 'ru-RU',
      requestId: '00000000-0000-4000-8000-000000000099',
    })

    expect(headers.Authorization).toBe('Bearer proof-token')
    expect(headers['X-Tenant-ID']).toBe('285f9447-faf7-423e-96dd-e4c5e2b3fc6c')
    expect(headers['X-Company-ID']).toBe('83cb2447-75e9-41f2-8e0d-93c70f8506be')
    expect(headers['X-Locale']).toBe('ru-RU')
    expect(headers['X-Request-ID']).toBe('00000000-0000-4000-8000-000000000099')
    expect(headers['X-User-ID']).toBeUndefined()
    expect(Object.keys(headers).map((k) => k.toLowerCase())).not.toContain('x-user-id')
  })

  it('authenticated header set is accepted by api-gateway CORS allowlist', () => {
    const headers = buildApiRequestHeaders({
      token: 'proof-token',
      tenantId: '285f9447-faf7-423e-96dd-e4c5e2b3fc6c',
      companyId: '83cb2447-75e9-41f2-8e0d-93c70f8506be',
      locale: 'ru-RU',
      requestId: '00000000-0000-4000-8000-000000000099',
    })

    const preflightHeaders = corsPreflightRequestHeaderNames(headers)
    expect(preflightHeaders).not.toContain('x-user-id')
    expect(headersAcceptedByGatewayCors(headers)).toBe(true)
    expect(preflightHeaders).toEqual([
      'authorization',
      'content-type',
      'x-company-id',
      'x-locale',
      'x-request-id',
      'x-tenant-id',
    ])
  })

  it('does not emit Authorization when skipAuth is set', () => {
    const headers = buildApiRequestHeaders({
      token: 'proof-token',
      tenantId: '285f9447-faf7-423e-96dd-e4c5e2b3fc6c',
      skipAuth: true,
    })

    expect(headers.Authorization).toBeUndefined()
    expect(headers['X-User-ID']).toBeUndefined()
  })

  it('documents gateway CORS contract for regression checks', () => {
    expect([...GATEWAY_CORS_ALLOWED_REQUEST_HEADERS]).toEqual([
      'content-type',
      'authorization',
      'x-tenant-id',
      'x-company-id',
      'x-request-id',
      'x-locale',
    ])
  })
})
