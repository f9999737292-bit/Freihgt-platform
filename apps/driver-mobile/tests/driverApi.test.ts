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

const STOP_ID = '11111111-1111-4111-8111-111111111111'
const ACTION_ID = '66666666-6666-4666-8666-666666666666'
const SHIPMENT_ID = '33333333-3333-4333-8333-333333333333'
const CARGO_ID = '44444444-4444-4444-8444-444444444444'

function stopFetch(body: unknown) {
  const fetchMock = vi.fn(async () =>
    new Response(JSON.stringify(body), { status: 200, headers: { 'Content-Type': 'application/json' } }),
  )
  vi.stubGlobal('fetch', fetchMock)
  return fetchMock
}

function sentRequest(fetchMock: ReturnType<typeof vi.fn>) {
  const call = fetchMock.mock.calls[0] as [RequestInfo | URL, RequestInit]
  const headers = (call[1]?.headers ?? {}) as Record<string, string>
  const raw = call[1]?.body
  return {
    url: new URL(String(call[0])),
    method: call[1]?.method,
    headers,
    body: typeof raw === 'string' ? JSON.parse(raw) as Record<string, unknown> : undefined,
  }
}

describe('GET_STOPS_TEST', () => {
  it('loads current and next stops from the public driver route', async () => {
    const fetchMock = stopFetch({ current: null, next: null })
    const http = new HttpClient({ getToken: () => 'token', isOnline: () => true })
    const result = await createDriverApi(http).getMyStops()
    const sent = sentRequest(fetchMock)
    expect(result.outcome).toBe('SUCCESS')
    expect(sent.method).toBe('GET')
    expect(sent.url.pathname).toBe('/api/v1/driver/me/stops')
    expect(sent.headers.Authorization).toBe('Bearer token')
    expect(sent.headers['Idempotency-Key']).toBeUndefined()
  })
})

describe('ARRIVE_STOP_TEST', () => {
  it('posts the stop command with an idempotency key', async () => {
    const fetchMock = stopFetch({
      taskId: 'task',
      executionStopId: STOP_ID,
      status: 'ARRIVED',
      version: 4,
      replayed: false,
    })
    const http = new HttpClient({ getToken: () => 'token', isOnline: () => true })
    const body = { occurredAt: '2026-10-07T08:00:00.000Z', expectedVersion: 3 }
    const result = await createDriverApi(http).arriveStop(STOP_ID, body, 'driver-mobile-op:arrive:key')
    const sent = sentRequest(fetchMock)
    expect(result.outcome).toBe('SUCCESS')
    expect(sent.method).toBe('POST')
    expect(sent.url.pathname).toBe(`/api/v1/driver/me/stops/${STOP_ID}/arrive`)
    expect(sent.headers['Idempotency-Key']).toBe('driver-mobile-op:arrive:key')
    expect(sent.body).toEqual(body)
  })
})

describe('START_SERVICE_TEST', () => {
  it('posts start-service with the server version', async () => {
    const fetchMock = stopFetch({
      taskId: 'task',
      executionStopId: STOP_ID,
      status: 'SERVICE_STARTED',
      version: 5,
      replayed: false,
    })
    const http = new HttpClient({ getToken: () => 'token', isOnline: () => true })
    const body = { occurredAt: '2026-10-07T08:05:00.000Z', expectedVersion: 4 }
    await createDriverApi(http).startStopService(STOP_ID, body, 'driver-mobile-op:start:key')
    const sent = sentRequest(fetchMock)
    expect(sent.method).toBe('POST')
    expect(sent.url.pathname).toBe(`/api/v1/driver/me/stops/${STOP_ID}/start-service`)
    expect(sent.headers['Idempotency-Key']).toBe('driver-mobile-op:start:key')
    expect(sent.body).toEqual(body)
  })
})

describe('COMPLETE_STOP_TEST', () => {
  it('posts complete with the server version', async () => {
    const fetchMock = stopFetch({
      taskId: 'task',
      executionStopId: STOP_ID,
      status: 'COMPLETED',
      version: 6,
      replayed: false,
    })
    const http = new HttpClient({ getToken: () => 'token', isOnline: () => true })
    const body = { occurredAt: '2026-10-07T08:20:00.000Z', expectedVersion: 5 }
    await createDriverApi(http).completeStop(STOP_ID, body, 'driver-mobile-op:complete:key')
    const sent = sentRequest(fetchMock)
    expect(sent.method).toBe('POST')
    expect(sent.url.pathname).toBe(`/api/v1/driver/me/stops/${STOP_ID}/complete`)
    expect(sent.headers['Idempotency-Key']).toBe('driver-mobile-op:complete:key')
    expect(sent.body).toEqual(body)
  })
})

describe('CONFIRM_ACTION_TEST', () => {
  it('posts confirm for the server action id', async () => {
    const fetchMock = stopFetch({
      taskId: 'task',
      executionStopId: STOP_ID,
      status: 'SERVICE_STARTED',
      version: 6,
      actionStatus: 'CONFIRMED',
      replayed: false,
    })
    const http = new HttpClient({ getToken: () => 'token', isOnline: () => true })
    const body = { occurredAt: '2026-10-07T08:10:00.000Z', expectedVersion: 5 }
    await createDriverApi(http).confirmStopAction(STOP_ID, ACTION_ID, body, 'driver-mobile-op:confirm:key')
    const sent = sentRequest(fetchMock)
    expect(sent.method).toBe('POST')
    expect(sent.url.pathname).toBe(`/api/v1/driver/me/stops/${STOP_ID}/actions/${ACTION_ID}/confirm`)
    expect(sent.headers['Idempotency-Key']).toBe('driver-mobile-op:confirm:key')
    expect(sent.body).toEqual(body)
  })
})

describe('FAIL_ACTION_TEST', () => {
  it('posts a bounded fail reason and no comment', async () => {
    const fetchMock = stopFetch({
      taskId: 'task',
      executionStopId: STOP_ID,
      status: 'SERVICE_STARTED',
      version: 6,
      actionStatus: 'FAILED',
      replayed: false,
    })
    const http = new HttpClient({ getToken: () => 'token', isOnline: () => true })
    const body = { occurredAt: '2026-10-07T08:12:00.000Z', expectedVersion: 5, reasonCode: 'TRAFFIC' as const }
    await createDriverApi(http).failStopAction(STOP_ID, ACTION_ID, body, 'driver-mobile-op:fail:key')
    const sent = sentRequest(fetchMock)
    expect(sent.method).toBe('POST')
    expect(sent.url.pathname).toBe(`/api/v1/driver/me/stops/${STOP_ID}/actions/${ACTION_ID}/fail`)
    expect(sent.headers['Idempotency-Key']).toBe('driver-mobile-op:fail:key')
    expect(sent.body).toEqual(body)
    expect(sent.body).not.toHaveProperty('reasonComment')
    expect(sent.body).not.toHaveProperty('comment')
  })
})

describe('DELIVERY_DISPOSITION_TEST', () => {
  it('posts pallet quantities and an empty evidence list', async () => {
    const fetchMock = stopFetch({
      executionId: 'exec',
      caseId: null,
      status: 'OPEN',
      acceptedQuantity: 1,
      rejectedQuantity: 1,
      revisionId: 'rev',
      replayed: false,
    })
    const http = new HttpClient({ getToken: () => 'token', isOnline: () => true })
    const body = {
      shipmentId: SHIPMENT_ID,
      cargoId: CARGO_ID,
      acceptedQuantity: 1,
      rejectedQuantity: 1,
      uom: 'PALLET' as const,
      reasonCode: 'DAMAGE' as const,
      reasonComment: 'torn wrap',
      occurredAt: '2026-10-07T08:15:00.000Z',
      evidence: [] as [],
    }
    await createDriverApi(http).reportDeliveryDisposition(STOP_ID, ACTION_ID, body, 'driver-mobile-op:disposition:key')
    const sent = sentRequest(fetchMock)
    expect(sent.method).toBe('POST')
    expect(sent.url.pathname).toBe(
      `/api/v1/driver/me/stops/${STOP_ID}/actions/${ACTION_ID}/delivery-disposition`,
    )
    expect(sent.headers['Idempotency-Key']).toBe('driver-mobile-op:disposition:key')
    expect(sent.body).toEqual(body)
  })
})
