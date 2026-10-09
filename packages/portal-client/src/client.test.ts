import { readFileSync } from 'node:fs'
import { describe, expect, it, vi } from 'vitest'

import type { ServerUserSnapshot } from '@freight-platform/shared-ts/types'

import { PortalClient, userFromLogin } from './client'
import { PortalClientError } from './errors'
import { buildPortalHeaders, FORBIDDEN_AUTHORITY_HEADERS } from './headers'

const user: ServerUserSnapshot = {
  id: 'user-1',
  tenantId: 'tenant-from-server',
  email: 'carrier@example.com',
  fullName: 'Carrier User',
  roles: ['CARRIER_DISPATCHER'],
}

function jsonResponse(body: unknown, status = 200): Response {
  return new Response(JSON.stringify(body), {
    status,
    headers: { 'Content-Type': 'application/json' },
  })
}

describe('portal headers', () => {
  it('sends only bearer, company, locale, and request id', () => {
    const headers = buildPortalHeaders({
      accessToken: 'tok',
      companyId: 'co-1',
      allowedCompanyIds: ['co-1'],
      locale: 'ru-RU',
      requestId: 'req-1',
    })
    expect(headers.Authorization).toBe('Bearer tok')
    expect(headers['X-Company-ID']).toBe('co-1')
    expect(headers['X-Locale']).toBe('ru-RU')
    expect(headers['X-Request-ID']).toBe('req-1')
    for (const forbidden of ['X-Tenant-ID', 'X-User-ID', 'X-User-Email']) {
      expect(headers[forbidden]).toBeUndefined()
    }
  })

  it.each(FORBIDDEN_AUTHORITY_HEADERS)('refuses %s', (header) => {
    expect(() => buildPortalHeaders({
      allowedCompanyIds: [],
      extra: { [header]: 'spoof' },
    })).toThrow(PortalClientError)
  })

  it('refuses a company id outside server memberships', () => {
    expect(() => buildPortalHeaders({
      companyId: 'co-other',
      allowedCompanyIds: ['co-1'],
    })).toThrowError(/not a server-returned carrier membership/)
  })
})

describe('PortalClient', () => {
  it('does not call fetch when a company spoof is requested', async () => {
    const fetchImpl = vi.fn()
    const client = new PortalClient({
      baseUrl: 'http://gateway.test',
      fetchImpl,
      getAccessToken: () => 'tok',
      getAllowedCompanyIds: () => ['co-1'],
    })
    await expect(client.request('/api/v1/carrier/rfx-events', {
      companyId: 'co-other',
      query: { carrier_company_id: 'co-other' },
    })).rejects.toMatchObject({ code: 'COMPANY_SPOOF' })
    expect(fetchImpl).not.toHaveBeenCalled()
  })

  it('rejects a cross-tenant override before calling the gateway', async () => {
    const fetchImpl = vi.fn()
    const client = new PortalClient({
      baseUrl: 'http://gateway.test',
      fetchImpl,
      getAccessToken: () => 'tok',
      getAllowedCompanyIds: () => [],
    })
    await expect(client.listMemberships(user, { tenantId: 'other-tenant' })).rejects.toMatchObject({
      code: 'TENANT_MISMATCH',
    })
    expect(fetchImpl).not.toHaveBeenCalled()
  })

  it('loads memberships with the snapshot tenant query and without authority headers', async () => {
    const fetchImpl = vi.fn(async () => jsonResponse({
      items: [{
        membership_id: 'm-1',
        company_id: 'co-1',
        legal_name: 'Carrier Co',
        membership_status: 'ACTIVE',
        roles: [{ code: 'CARRIER_DISPATCHER' }],
      }],
    }))
    const client = new PortalClient({
      baseUrl: 'http://gateway.test',
      fetchImpl,
      getAccessToken: () => 'tok',
      getAllowedCompanyIds: () => [],
      getLocale: () => 'ru-RU',
    })
    const memberships = await client.listMemberships(user)
    expect(memberships[0]?.companyId).toBe('co-1')
    const [url, init] = fetchImpl.mock.calls[0] as unknown as [string, RequestInit]
    expect(url).toContain('tenant_id=tenant-from-server')
    expect(url).not.toContain('other-tenant')
    const headers = init.headers as Record<string, string>
    expect(headers['X-Tenant-ID']).toBeUndefined()
    expect(headers['X-User-ID']).toBeUndefined()
    expect(headers['X-User-Email']).toBeUndefined()
    expect(headers.Authorization).toBe('Bearer tok')
  })

  it('posts the canonical login body and keeps the returned tenant off the headers', async () => {
    const fetchImpl = vi.fn(async () => jsonResponse({
      access_token: 'tok',
      token_type: 'Bearer',
      expires_in: 3600,
      user: {
        id: 'user-1',
        tenant_id: 'tenant-from-server',
        email: 'carrier@example.com',
        full_name: 'Carrier User',
        roles: ['CARRIER_ADMIN'],
      },
    }))
    const client = new PortalClient({
      baseUrl: 'http://gateway.test',
      fetchImpl,
      getAccessToken: () => null,
      getAllowedCompanyIds: () => [],
    })
    const payload = await client.login({
      tenantId: 'tenant-from-server',
      email: 'carrier@example.com',
      password: 'secret',
    })
    expect(userFromLogin(payload).tenantId).toBe('tenant-from-server')
    const [, init] = fetchImpl.mock.calls[0] as unknown as [string, RequestInit]
    expect(JSON.parse(String(init.body))).toEqual({
      tenant_id: 'tenant-from-server',
      email: 'carrier@example.com',
      password: 'secret',
    })
    const headers = init.headers as Record<string, string>
    expect(headers.Authorization).toBeUndefined()
    expect(headers['X-Tenant-ID']).toBeUndefined()
  })

  it('rejects a domain-service URL', async () => {
    const client = new PortalClient({
      baseUrl: 'http://gateway.test',
      getAccessToken: () => 'tok',
      getAllowedCompanyIds: () => [],
    })
    await expect(client.request('http://rfx-service:8080/v1/carrier/rfx-events')).rejects.toMatchObject({
      code: 'GATEWAY_ONLY',
    })
  })

  it('clears through the unauthorized callback on 401', async () => {
    const onUnauthorized = vi.fn()
    const client = new PortalClient({
      baseUrl: 'http://gateway.test',
      fetchImpl: async () => jsonResponse({ message: 'unauthorized' }, 401),
      getAccessToken: () => 'tok',
      getAllowedCompanyIds: () => ['co-1'],
      onUnauthorized,
    })
    await expect(client.request('/api/v1/carrier/rfx-events/event-1', {
      companyId: 'co-1',
    })).rejects.toMatchObject({ status: 401 })
    expect(onUnauthorized).toHaveBeenCalledOnce()
  })
})

describe('client source', () => {
  it('does not mention localStorage', () => {
    const source = readFileSync(new URL('./client.ts', import.meta.url), 'utf8')
    expect(source.includes('localStorage')).toBe(false)
    expect(source.includes('X-Tenant-ID')).toBe(false)
    expect(source.includes('X-User-ID')).toBe(false)
    expect(source.includes('X-User-Email')).toBe(false)
  })
})
