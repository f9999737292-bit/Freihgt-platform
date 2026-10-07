import { describe, expect, it, vi } from 'vitest'
import { HttpClient } from '@/api/client'
import { createDriverApi } from '@/api/driverApi'
import { CLIENT_FORBIDDEN_FIELDS } from '@/stops/commands'

const STOP_ID = '11111111-1111-4111-8111-111111111111'
const ACTION_ID = '66666666-6666-4666-8666-666666666666'

function capture() {
  const fetchMock = vi.fn(async () =>
    new Response(JSON.stringify({ current: null, next: null, replayed: false, taskId: 't', executionStopId: STOP_ID, status: 'ARRIVED', version: 2, executionId: 'e', caseId: null, acceptedQuantity: 1, rejectedQuantity: 0, revisionId: 'r' }), {
      status: 200,
      headers: { 'Content-Type': 'application/json' },
    }),
  )
  vi.stubGlobal('fetch', fetchMock)
  return fetchMock
}

function sent(fetchMock: ReturnType<typeof vi.fn>) {
  const call = fetchMock.mock.calls[0] as [RequestInfo | URL, RequestInit]
  const headers = (call[1]?.headers ?? {}) as Record<string, string>
  const raw = call[1]?.body
  return {
    headers,
    body: typeof raw === 'string' ? JSON.parse(raw) as Record<string, unknown> : {},
  }
}

describe('stop command security', () => {
  it('keeps driver, tenant, execution, and revision server-derived', async () => {
    const http = new HttpClient({ getToken: () => 'token', isOnline: () => true })
    const api = createDriverApi(http)
    const command = { occurredAt: '2026-10-07T08:00:00.000Z', expectedVersion: 3 }
    const calls = [
      () => api.arriveStop(STOP_ID, command, 'k-arrive'),
      () => api.startStopService(STOP_ID, command, 'k-start'),
      () => api.completeStop(STOP_ID, command, 'k-complete'),
      () => api.confirmStopAction(STOP_ID, ACTION_ID, command, 'k-confirm'),
      () => api.failStopAction(STOP_ID, ACTION_ID, { ...command, reasonCode: 'OTHER' }, 'k-fail'),
      () => api.reportDeliveryDisposition(STOP_ID, ACTION_ID, {
        shipmentId: '33333333-3333-4333-8333-333333333333',
        cargoId: '44444444-4444-4444-8444-444444444444',
        acceptedQuantity: 1,
        rejectedQuantity: 0,
        uom: 'PALLET',
        occurredAt: command.occurredAt,
        evidence: [],
      }, 'k-disposition'),
    ]

    for (const call of calls) {
      const fetchMock = capture()
      await call()
      const request = sent(fetchMock)
      expect(request.headers.Authorization).toBe('Bearer token')
      expect(request.headers['X-Tenant-ID']).toBeUndefined()
      expect(request.headers['X-Driver-ID']).toBeUndefined()
      for (const field of CLIENT_FORBIDDEN_FIELDS) {
        expect(request.body).not.toHaveProperty(field)
      }
      expect(request.body).not.toHaveProperty('idempotencyKey')
    }
  })

  it('does not expose return, redirect, or hold methods', () => {
    const http = new HttpClient({ getToken: () => 'token', isOnline: () => true })
    const api = createDriverApi(http)
    expect(api).not.toHaveProperty('authorizeReturn')
    expect(api).not.toHaveProperty('authorizeRedirect')
    expect(api).not.toHaveProperty('holdDisposition')
  })
})
