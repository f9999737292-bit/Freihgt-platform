import { describe, expect, it, vi } from 'vitest'
import { HttpClient } from '@/api/client'
import { createDriverApi } from '@/api/driverApi'

describe('AUTH_REQUIRED_TEST', () => {
  it('returns unauthorized when token missing', async () => {
    const http = new HttpClient({ getToken: () => null, isOnline: () => true })
    const api = createDriverApi(http)
    const result = await api.getMyShipments()
    expect(result.outcome).toBe('SERVER_REJECTED')
    expect(result.error?.status).toBe(401)
  })
})

describe('UNAUTHORIZED_TEST', () => {
  it('maps 401 response to SERVER_REJECTED', async () => {
    vi.stubGlobal('fetch', vi.fn(async () =>
      new Response(JSON.stringify({ error: { code: 'UNAUTHORIZED', message: 'bad token' } }), {
        status: 401,
        headers: { 'Content-Type': 'application/json' },
      }),
    ))

    const http = new HttpClient({ getToken: () => 'token', isOnline: () => true })
    const api = createDriverApi(http)
    const result = await api.getMyProfile()
    expect(result.outcome).toBe('SERVER_REJECTED')
    expect(result.error?.code).toBe('UNAUTHORIZED')
  })
})

describe('MY_SHIPMENTS_TEST', () => {
  it('loads assigned shipments', async () => {
    vi.stubGlobal('fetch', vi.fn(async () =>
      new Response(JSON.stringify({ items: [{ id: 's1', shipmentNumber: 'SHP-1', status: 'IN_TRANSIT' }], total: 1 }), {
        status: 200,
        headers: { 'Content-Type': 'application/json' },
      }),
    ))

    const http = new HttpClient({ getToken: () => 'token', isOnline: () => true })
    const api = createDriverApi(http)
    const result = await api.getMyShipments()
    expect(result.outcome).toBe('SUCCESS')
    expect(result.data?.items).toHaveLength(1)
  })
})

describe('REPORT_DELAY_SUCCESS_TEST', () => {
  it('submits delay payload with idempotency header', async () => {
    const fetchMock = vi.fn(async (_input: RequestInfo | URL, _init?: RequestInit) =>
      new Response(JSON.stringify({ id: 'd1', shipmentId: 's1', replayed: false }), {
        status: 201,
        headers: { 'Content-Type': 'application/json' },
      }),
    )
    vi.stubGlobal('fetch', fetchMock)

    const http = new HttpClient({ getToken: () => 'token', isOnline: () => true })
    const api = createDriverApi(http)
    const result = await api.reportDelay('s1', {
      reasonCode: 'TRAFFIC',
      idempotencyKey: 'driver-mobile-op:delay:s1:abc',
    })

    expect(result.outcome).toBe('SUCCESS')
    const firstCall = fetchMock.mock.calls[0] as [RequestInfo | URL, RequestInit | undefined]
    const headers = (firstCall[1]?.headers ?? {}) as Record<string, string>
    expect(headers['Idempotency-Key']).toBe('driver-mobile-op:delay:s1:abc')
  })
})

describe('REPORT_DELAY_FAILURE_TEST', () => {
  it('maps validation failure', async () => {
    vi.stubGlobal('fetch', vi.fn(async () =>
      new Response(JSON.stringify({ error: { code: 'VALIDATION', message: 'bad reason' } }), {
        status: 400,
        headers: { 'Content-Type': 'application/json' },
      }),
    ))

    const http = new HttpClient({ getToken: () => 'token', isOnline: () => true })
    const api = createDriverApi(http)
    const result = await api.reportDelay('s1', {
      reasonCode: 'OTHER',
      idempotencyKey: 'key-1',
    })
    expect(result.outcome).toBe('SERVER_REJECTED')
  })
})

describe('REPORT_DELAY_DOUBLE_SUBMIT_TEST', () => {
  it('reuses stable idempotency key on replay response', async () => {
    vi.stubGlobal('fetch', vi.fn(async () =>
      new Response(JSON.stringify({ id: 'd1', replayed: true }), {
        status: 200,
        headers: { 'Content-Type': 'application/json' },
      }),
    ))

    const key = 'driver-mobile-op:delay:s1:same-key'
    const http = new HttpClient({ getToken: () => 'token', isOnline: () => true })
    const api = createDriverApi(http)
    const result = await api.reportDelay('s1', { reasonCode: 'TRAFFIC', idempotencyKey: key })
    expect(result.outcome).toBe('SUCCESS')
    expect(result.data?.replayed).toBe(true)
  })
})

describe('REPORT_PROBLEM_SUCCESS_TEST', () => {
  it('submits exception payload', async () => {
    vi.stubGlobal('fetch', vi.fn(async () =>
      new Response(JSON.stringify({ id: 'e1', category: 'VEHICLE_BREAKDOWN', replayed: false }), {
        status: 201,
        headers: { 'Content-Type': 'application/json' },
      }),
    ))

    const http = new HttpClient({ getToken: () => 'token', isOnline: () => true })
    const api = createDriverApi(http)
    const result = await api.reportProblem('s1', {
      category: 'VEHICLE_BREAKDOWN',
      idempotencyKey: 'driver-mobile-op:problem:s1:abc',
    })
    expect(result.outcome).toBe('SUCCESS')
  })
})

describe('REPORT_PROBLEM_FAILURE_TEST', () => {
  it('maps forbidden response', async () => {
    vi.stubGlobal('fetch', vi.fn(async () =>
      new Response(JSON.stringify({ error: { code: 'FORBIDDEN', message: 'not assigned' } }), {
        status: 403,
        headers: { 'Content-Type': 'application/json' },
      }),
    ))

    const http = new HttpClient({ getToken: () => 'token', isOnline: () => true })
    const api = createDriverApi(http)
    const result = await api.reportProblem('s1', {
      category: 'ACCIDENT',
      idempotencyKey: 'key-2',
    })
    expect(result.outcome).toBe('SERVER_REJECTED')
    expect(result.error?.status).toBe(403)
  })
})

describe('OFFLINE_STATE_TEST', () => {
  it('does not send request when offline', async () => {
    const fetchMock = vi.fn()
    vi.stubGlobal('fetch', fetchMock)
    const http = new HttpClient({ getToken: () => 'token', isOnline: () => false })
    const api = createDriverApi(http)
    const result = await api.reportDelay('s1', { reasonCode: 'TRAFFIC', idempotencyKey: 'k' })
    expect(result.outcome).toBe('REQUEST_NOT_SENT')
    expect(fetchMock).not.toHaveBeenCalled()
  })
})

describe('TENANT_NOT_CLIENT_CONTROLLED_TEST', () => {
  it('driver API requests do not include X-Tenant-ID header', async () => {
    const fetchMock = vi.fn(async (_input: RequestInfo | URL, _init?: RequestInit) =>
      new Response(JSON.stringify({ items: [], total: 0 }), { status: 200 }),
    )
    vi.stubGlobal('fetch', fetchMock)

    const http = new HttpClient({ getToken: () => 'token', isOnline: () => true })
    const api = createDriverApi(http)
    await api.getMyShipments()

    expect(fetchMock).toHaveBeenCalled()
    const firstCall = fetchMock.mock.calls[0] as [RequestInfo | URL, RequestInit | undefined]
    const headers = (firstCall[1]?.headers ?? {}) as Record<string, string>
    expect(headers['X-Tenant-ID']).toBeUndefined()
    expect(headers.Authorization).toBe('Bearer token')
  })
})
