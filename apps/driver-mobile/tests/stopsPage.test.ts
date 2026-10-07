import { mount, flushPromises } from '@vue/test-utils'
import { createPinia, setActivePinia } from 'pinia'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { createMemoryHistory, createRouter } from 'vue-router'
import { i18n } from '@/i18n'
import ShipmentsPage from '@/pages/ShipmentsPage.vue'
import StopsPage from '@/pages/StopsPage.vue'
import { useAuthStore } from '@/stores/auth'
import { useNetworkStore } from '@/stores/network'
import type { DriverCurrentNextStopsResponse, DriverStopTask } from '@/types/stops'

const STOP_ID = '11111111-1111-4111-8111-111111111111'
const TASK_ID = 'aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa'
const NEXT_STOP_ID = '22222222-2222-4222-8222-222222222222'
const SHIPMENT_ID = '33333333-3333-4333-8333-333333333333'
const CARGO_ID = '44444444-4444-4444-8444-444444444444'
const LOCATION_ID = '55555555-5555-4555-8555-555555555555'
const ACTION_ID = '66666666-6666-4666-8666-666666666666'
const NEXT_LOCATION_ID = '99999999-9999-4999-8999-999999999999'
const NEXT_SHIPMENT_ID = '88888888-8888-4888-8888-888888888888'
const ACTION_A = 'a1111111-1111-4111-8111-111111111111'
const ACTION_B = 'b2222222-2222-4222-8222-222222222222'
const SHIPMENT_S1 = 'c1111111-1111-4111-8111-111111111111'
const SHIPMENT_S2 = 'd2222222-2222-4222-8222-222222222222'
const CARGO_C1 = 'e1111111-1111-4111-8111-111111111111'
const CARGO_C2 = 'f2222222-2222-4222-8222-222222222222'
const NIL_UUID = '00000000-0000-0000-0000-000000000000'

function task(overrides: Partial<DriverStopTask> = {}): DriverStopTask {
  return {
    taskId: TASK_ID,
    executionId: '77777777-7777-4777-8777-777777777777',
    executionStopId: STOP_ID,
    shipmentId: SHIPMENT_ID,
    ordinal: 2,
    locationId: LOCATION_ID,
    plannedArrival: '2026-10-07T08:00:00Z',
    status: 'PLANNED',
    version: 3,
    position: 'CURRENT',
    actionSummary: {
      actions: [
        { actionId: ACTION_ID, actionType: 'DELIVERY', shipmentId: SHIPMENT_ID, cargoId: CARGO_ID, ordinal: 1 },
      ],
      counts: { DELIVERY: 1 },
    },
    ...overrides,
  }
}

function multiShipmentStop(stopShipmentId: string | null = null): DriverStopTask {
  return task({
    shipmentId: stopShipmentId,
    status: 'SERVICE_STARTED',
    version: 5,
    actionSummary: {
      actions: [
        { actionId: ACTION_A, actionType: 'DELIVERY', shipmentId: SHIPMENT_S1, cargoId: CARGO_C1, ordinal: 1 },
        { actionId: ACTION_B, actionType: 'DELIVERY', shipmentId: SHIPMENT_S2, cargoId: CARGO_C2, ordinal: 2 },
      ],
      counts: { DELIVERY: 2 },
    },
  })
}

function dispositionResult(acceptedQuantity: number, rejectedQuantity: number) {
  return {
    executionId: '77777777-7777-4777-8777-777777777777',
    caseId: rejectedQuantity > 0 ? 'case' : null,
    status: 'OPEN',
    acceptedQuantity,
    rejectedQuantity,
    revisionId: 'rev',
    replayed: false,
  }
}

function stopsResponse(
  current: DriverStopTask | null,
  next: DriverStopTask | null = task({
    taskId: 'bbbbbbbb-bbbb-4bbb-8bbb-bbbbbbbbbbbb',
    executionStopId: NEXT_STOP_ID,
    shipmentId: NEXT_SHIPMENT_ID,
    locationId: NEXT_LOCATION_ID,
    ordinal: 3,
    plannedArrival: '2026-10-07T12:00:00Z',
    status: 'PLANNED',
    position: 'NEXT',
    version: 3,
  }),
): DriverCurrentNextStopsResponse {
  return { current, next }
}

function json(body: unknown, status = 200) {
  return new Response(JSON.stringify(body), {
    status,
    headers: { 'Content-Type': 'application/json' },
  })
}

function commandBody(status: string, version: number) {
  return {
    taskId: TASK_ID,
    executionStopId: STOP_ID,
    status,
    version,
    replayed: false,
  }
}

type FetchHandler = (url: string, init?: RequestInit) => Response | Promise<Response>

let activeFetch = vi.fn()

function installFetch(handler: FetchHandler) {
  activeFetch = vi.fn(async (input: RequestInfo | URL, init?: RequestInit) => {
    const url = typeof input === 'string' ? input : input instanceof URL ? input.toString() : input.url
    return handler(url, init)
  })
  vi.stubGlobal('fetch', activeFetch)
  return activeFetch
}

function sentRequests() {
  return requests(activeFetch)
}

function requests(fetchMock: ReturnType<typeof vi.fn>) {
  return fetchMock.mock.calls.map((call) => {
    const [input, init] = call as [RequestInfo | URL, RequestInit | undefined]
    const url = typeof input === 'string' ? input : input instanceof URL ? input.toString() : input.url
    const headers = (init?.headers ?? {}) as Record<string, string>
    const raw = init?.body
    return {
      url,
      method: init?.method ?? 'GET',
      headers,
      body: typeof raw === 'string' ? JSON.parse(raw) as Record<string, unknown> : undefined,
    }
  })
}

async function mountStops(handler: FetchHandler) {
  const fetchMock = installFetch(handler)
  const pinia = createPinia()
  setActivePinia(pinia)
  const auth = useAuthStore()
  auth.token = 'token'
  auth.user = {
    id: 'user-1',
    tenant_id: 'tenant-1',
    email: 'driver@example.com',
    full_name: 'Driver',
    status: 'ACTIVE',
  }
  const network = useNetworkStore()
  network.online = true
  const router = createRouter({
    history: createMemoryHistory(),
    routes: [
      { path: '/stops', component: StopsPage },
      { path: '/login', component: { template: '<div>login</div>' } },
      { path: '/shipments', component: { template: '<div>shipments</div>' } },
    ],
  })
  await router.push('/stops')
  await router.isReady()
  const wrapper = mount(StopsPage, { global: { plugins: [pinia, router, i18n] } })
  await flushPromises()
  return { wrapper, auth, network, router, fetchMock }
}

beforeEach(() => {
  sessionStorage.clear()
})

afterEach(() => {
  vi.unstubAllGlobals()
  vi.restoreAllMocks()
})

describe('CURRENT_STOP_RENDER', () => {
  it('shows the current stop facts from the server read model', async () => {
    const { wrapper } = await mountStops(() => json(stopsResponse(task())))
    const current = wrapper.get('[data-testid="current-stop"]')
    expect(current.text()).toContain('Текущая остановка')
    expect(current.text()).toContain('2')
    expect(current.text()).toContain('2026-10-07T08:00:00Z')
    expect(current.text()).toContain('PLANNED')
    expect(current.text()).toContain(LOCATION_ID)
    expect(current.text()).toContain(SHIPMENT_ID)
    expect(current.text()).toContain('DELIVERY')
    expect(current.text()).not.toContain(TASK_ID)
  })
})

describe('NEXT_STOP_RENDER', () => {
  it('shows the next stop summary without commands', async () => {
    const { wrapper } = await mountStops(() => json(stopsResponse(task())))
    const next = wrapper.get('[data-testid="next-stop"]')
    expect(next.text()).toContain('Следующая остановка')
    expect(next.text()).toContain('3')
    expect(next.text()).toContain('2026-10-07T12:00:00Z')
    expect(next.text()).toContain('PLANNED')
    expect(next.text()).toContain(NEXT_LOCATION_ID)
    expect(next.text()).toContain(NEXT_SHIPMENT_ID)
    expect(next.find('[data-testid="arrive"]').exists()).toBe(false)
    expect(next.find('[data-testid="complete"]').exists()).toBe(false)
  })
})

describe('EMPTY_STATE', () => {
  it('shows that no current stop is assigned', async () => {
    const { wrapper } = await mountStops(() => json({ current: null, next: null }))
    expect(wrapper.get('[data-testid="empty-current"]').text()).toBe('Нет назначенной текущей остановки')
    expect(wrapper.find('[data-testid="arrive"]').exists()).toBe(false)
  })
})

describe('ARRIVE', () => {
  it('posts arrive for the execution stop and reloads', async () => {
    let reads = 0
    const { wrapper } = await mountStops(() => {
      reads += 1
      if (reads === 1) return json(stopsResponse(task()))
      return json(stopsResponse(task({ status: 'ARRIVED', version: 4 })))
    })
    await wrapper.get('[data-testid="arrive"]').trigger('click')
    await flushPromises()
    const posted = sentRequests().filter((call) => call.method === 'POST')
    expect(posted).toHaveLength(1)
    expect(posted[0]?.url).toContain(`/api/v1/driver/me/stops/${STOP_ID}/arrive`)
    expect(posted[0]?.url).not.toContain(TASK_ID)
    expect(posted[0]?.url).not.toContain('/events')
    expect(posted[0]?.headers['Idempotency-Key']).toBeTruthy()
    expect(posted[0]?.body).toMatchObject({ expectedVersion: 3 })
    expect(posted[0]?.body).not.toHaveProperty('reasonCode')
    expect(wrapper.get('[data-testid="current-status"]').text()).toContain('ARRIVED')
  })
})

describe('START_SERVICE', () => {
  it('posts start-service from the arrived stop', async () => {
    let reads = 0
    const { wrapper } = await mountStops(() => {
      reads += 1
      if (reads === 1) return json(stopsResponse(task({ status: 'ARRIVED', version: 4 })))
      return json(stopsResponse(task({ status: 'SERVICE_STARTED', version: 5 })))
    })
    await wrapper.get('[data-testid="start-service"]').trigger('click')
    await flushPromises()
    const posted = sentRequests().filter((call) => call.method === 'POST')
    expect(posted[0]?.url).toContain(`/start-service`)
    expect(posted[0]?.body).toMatchObject({ expectedVersion: 4 })
    expect(wrapper.get('[data-testid="current-status"]').text()).toContain('SERVICE_STARTED')
  })
})

describe('COMPLETE', () => {
  it('posts complete and reloads the server status', async () => {
    let reads = 0
    const { wrapper } = await mountStops(() => {
      reads += 1
      if (reads === 1) return json(stopsResponse(task({ status: 'SERVICE_STARTED', version: 5 })))
      return json(stopsResponse(task({ status: 'COMPLETED', version: 6 })))
    })
    await wrapper.get('[data-testid="complete"]').trigger('click')
    await flushPromises()
    const posted = sentRequests().filter((call) => call.method === 'POST')
    expect(posted[0]?.url).toContain('/complete')
    expect(posted[0]?.url).not.toContain('/events')
    expect(posted[0]?.body).toMatchObject({ expectedVersion: 5 })
    expect(wrapper.get('[data-testid="current-status"]').text()).toContain('COMPLETED')
  })

  it('keeps the stop when complete returns ACTION_PENDING', async () => {
    const { wrapper } = await mountStops((_url, init) => {
      if (init?.method === 'POST') {
        return json({ error: { code: 'CONFLICT', message: 'ACTION_PENDING' } }, 409)
      }
      return json(stopsResponse(task({ status: 'SERVICE_STARTED', version: 5 })))
    })
    await wrapper.get('[data-testid="complete"]').trigger('click')
    await flushPromises()
    expect(wrapper.get('[data-testid="notice"]').text()).toContain('незавершённые действия')
    expect(wrapper.get('[data-testid="current-status"]').text()).toContain('SERVICE_STARTED')
    expect(sentRequests().every((call) => !call.url.includes('/events'))).toBe(true)
  })
})

describe('CONFIRM_ACTION', () => {
  it('confirms the delivery action and reloads', async () => {
    const { wrapper } = await mountStops((_url, init) => {
      if (init?.method === 'POST') return json(commandBody('SERVICE_STARTED', 6))
      return json(stopsResponse(task({ status: 'SERVICE_STARTED', version: 5 })))
    })
    await wrapper.get(`[data-testid="confirm-${ACTION_ID}"]`).trigger('click')
    await flushPromises()
    const posted = sentRequests().find((call) => call.method === 'POST')
    expect(posted?.url).toContain(`/actions/${ACTION_ID}/confirm`)
    expect(posted?.headers['Idempotency-Key']).toBeTruthy()
    expect(posted?.body).toMatchObject({ expectedVersion: 5 })
  })
})

describe('FAIL_ACTION', () => {
  it('fails the action with a bounded reason and no comment', async () => {
    const { wrapper } = await mountStops((_url, init) => {
      if (init?.method === 'POST') return json(commandBody('SERVICE_STARTED', 6))
      return json(stopsResponse(task({ status: 'SERVICE_STARTED', version: 5 })))
    })
    await wrapper.get(`[data-testid="fail-reason-${ACTION_ID}"]`).setValue('TRAFFIC')
    await wrapper.get(`[data-testid="fail-${ACTION_ID}"]`).trigger('click')
    await flushPromises()
    const posted = sentRequests().find((call) => call.method === 'POST')
    expect(posted?.url).toContain(`/actions/${ACTION_ID}/fail`)
    expect(posted?.body).toEqual({
      occurredAt: posted?.body?.occurredAt,
      expectedVersion: 5,
      reasonCode: 'TRAFFIC',
    })
    expect(posted?.body).not.toHaveProperty('reasonComment')
    expect(wrapper.find(`[data-testid="disposition-comment-${ACTION_ID}"]`).exists()).toBe(true)
    expect(wrapper.text()).not.toContain('Authorize Return')
  })
})

describe('FULL_ACCEPTANCE', () => {
  it('records acceptance without a rejection reason', async () => {
    const { wrapper } = await mountStops((_url, init) => {
      if (init?.method === 'POST') {
        return json({
          executionId: '77777777-7777-4777-8777-777777777777',
          caseId: null,
          status: 'OPEN',
          acceptedQuantity: 2,
          rejectedQuantity: 0,
          revisionId: 'rev',
          replayed: false,
        })
      }
      return json(stopsResponse(task({ status: 'SERVICE_STARTED', version: 5 })))
    })
    await wrapper.get(`[data-testid="accepted-${ACTION_ID}"]`).setValue('2')
    await wrapper.get(`[data-testid="disposition-form-${ACTION_ID}"]`).trigger('submit')
    await flushPromises()
    const posted = sentRequests().find((call) => call.method === 'POST')
    expect(posted?.url).toContain('/delivery-disposition')
    expect(posted?.body).toMatchObject({
      shipmentId: SHIPMENT_ID,
      cargoId: CARGO_ID,
      acceptedQuantity: 2,
      rejectedQuantity: 0,
      uom: 'PALLET',
      evidence: [],
    })
    expect(posted?.body).not.toHaveProperty('reasonCode')
    expect(posted?.headers['Idempotency-Key']).toBeTruthy()
    expect(wrapper.get('[data-testid="notice"]').text()).toBe('Результат доставки зарегистрирован')
    expect(wrapper.find('[data-testid="authorize-return"]').exists()).toBe(false)
    expect(wrapper.find('[data-testid="authorize-redirect"]').exists()).toBe(false)
  })
})

describe('PARTIAL_REJECTION', () => {
  it('requires a reason and tells the driver that dispatch decides', async () => {
    const { wrapper } = await mountStops((_url, init) => {
      if (init?.method === 'POST') {
        return json({
          executionId: '77777777-7777-4777-8777-777777777777',
          caseId: 'case',
          status: 'OPEN',
          acceptedQuantity: 1,
          rejectedQuantity: 1,
          revisionId: 'rev',
          replayed: false,
        })
      }
      return json(stopsResponse(task({ status: 'SERVICE_STARTED', version: 5 })))
    })
    await wrapper.get(`[data-testid="accepted-${ACTION_ID}"]`).setValue('1')
    await wrapper.get(`[data-testid="rejected-${ACTION_ID}"]`).setValue('1')
    await wrapper.get(`[data-testid="disposition-form-${ACTION_ID}"]`).trigger('submit')
    await flushPromises()
    expect(sentRequests().some((call) => call.method === 'POST')).toBe(false)
    await wrapper.get(`[data-testid="disposition-reason-${ACTION_ID}"]`).setValue('DAMAGE')
    await wrapper.get(`[data-testid="disposition-form-${ACTION_ID}"]`).trigger('submit')
    await flushPromises()
    const posted = sentRequests().find((call) => call.method === 'POST')
    expect(posted?.body).toMatchObject({
      acceptedQuantity: 1,
      rejectedQuantity: 1,
      reasonCode: 'DAMAGE',
      uom: 'PALLET',
    })
    expect(wrapper.get('[data-testid="notice"]').text()).toBe(
      'Отклонение зарегистрировано. Решение о возврате или перенаправлении принимает диспетчер.',
    )
  })
})

describe('FULL_REJECTION', () => {
  it('records a full rejection for dispatcher decision', async () => {
    const { wrapper } = await mountStops((_url, init) => {
      if (init?.method === 'POST') {
        return json({
          executionId: '77777777-7777-4777-8777-777777777777',
          caseId: 'case',
          status: 'OPEN',
          acceptedQuantity: 0,
          rejectedQuantity: 3,
          revisionId: 'rev',
          replayed: false,
        })
      }
      return json(stopsResponse(task({ status: 'SERVICE_STARTED', version: 5 })))
    })
    await wrapper.get(`[data-testid="rejected-${ACTION_ID}"]`).setValue('3')
    await wrapper.get(`[data-testid="disposition-reason-${ACTION_ID}"]`).setValue('SHORTAGE')
    await wrapper.get(`[data-testid="disposition-form-${ACTION_ID}"]`).trigger('submit')
    await flushPromises()
    const posted = sentRequests().find((call) => call.method === 'POST')
    expect(posted?.body).toMatchObject({ acceptedQuantity: 0, rejectedQuantity: 3, reasonCode: 'SHORTAGE', uom: 'PALLET' })
    expect(wrapper.get('[data-testid="notice"]').text()).toContain('Отклонение зарегистрировано')
  })
})

describe('OTHER_COMMENT_VALIDATION', () => {
  it('blocks other without a comment and sends the comment once provided', async () => {
    const { wrapper } = await mountStops((_url, init) => {
      if (init?.method === 'POST') {
        return json({
          executionId: '77777777-7777-4777-8777-777777777777',
          caseId: 'case',
          status: 'OPEN',
          acceptedQuantity: 0,
          rejectedQuantity: 1,
          revisionId: 'rev',
          replayed: false,
        })
      }
      return json(stopsResponse(task({ status: 'SERVICE_STARTED', version: 5 })))
    })
    await wrapper.get(`[data-testid="rejected-${ACTION_ID}"]`).setValue('1')
    await wrapper.get(`[data-testid="disposition-reason-${ACTION_ID}"]`).setValue('OTHER')
    await wrapper.get(`[data-testid="disposition-form-${ACTION_ID}"]`).trigger('submit')
    await flushPromises()
    expect(sentRequests().some((call) => call.method === 'POST')).toBe(false)
    expect(wrapper.get('[data-testid="notice"]').text()).toContain('комментарий')
    await wrapper.get(`[data-testid="disposition-comment-${ACTION_ID}"]`).setValue('seal broken')
    await wrapper.get(`[data-testid="disposition-form-${ACTION_ID}"]`).trigger('submit')
    await flushPromises()
    const posted = sentRequests().find((call) => call.method === 'POST')
    expect(posted?.body).toMatchObject({ reasonCode: 'OTHER', reasonComment: 'seal broken', uom: 'PALLET' })
  })
})

describe('DOUBLE_SUBMIT_BLOCKED', () => {
  it('sends one in-flight arrive request', async () => {
    let release: (response: Response) => void = () => undefined
    const gate = new Promise<Response>((resolve) => {
      release = resolve
    })
    const { wrapper } = await mountStops((_url, init) => {
      if (init?.method === 'POST') return gate
      return json(stopsResponse(task()))
    })
    try {
      const button = wrapper.get('[data-testid="arrive"]').element as HTMLButtonElement
      button.click()
      button.click()
      const posted = sentRequests().filter((call) => call.method === 'POST')
      expect(posted).toHaveLength(1)
    } finally {
      release(json(commandBody('ARRIVED', 4)))
      await flushPromises()
    }
  })
})

describe('OFFLINE_NO_SEND', () => {
  it('does not send arrive while offline and does not report success', async () => {
    const { wrapper, network } = await mountStops(() => json(stopsResponse(task())))
    const before = activeFetch.mock.calls.length
    network.online = false
    await wrapper.get('[data-testid="arrive"]').trigger('click')
    await flushPromises()
    expect(activeFetch.mock.calls.length).toBe(before)
    expect(wrapper.get('[data-testid="notice"]').text()).toContain('Запрос не отправлен')
    expect(wrapper.get('[data-testid="current-status"]').text()).toContain('PLANNED')
  })
})

describe('UNKNOWN_RESPONSE_KEY_REUSED', () => {
  it('retries arrive with the same idempotency key', async () => {
    const keys: string[] = []
    const { wrapper } = await mountStops((_url, init) => {
      if (init?.method === 'POST') {
        keys.push(((init.headers ?? {}) as Record<string, string>)['Idempotency-Key'] ?? '')
        throw new TypeError('network down')
      }
      return json(stopsResponse(task()))
    })
    await wrapper.get('[data-testid="arrive"]').trigger('click')
    await flushPromises()
    expect(wrapper.get('[data-testid="notice"]').text()).toContain('неизвестен')
    await wrapper.get('[data-testid="retry"]').trigger('click')
    await flushPromises()
    expect(keys).toHaveLength(2)
    expect(keys[0]).toBeTruthy()
    expect(keys[1]).toBe(keys[0])
    expect(wrapper.get('[data-testid="current-status"]').text()).toContain('PLANNED')
  })
})

describe('VERSION_CONFLICT_REFRESH', () => {
  it('reloads stops and shows the safe conflict message', async () => {
    let reads = 0
    const { wrapper } = await mountStops((_url, init) => {
      if (init?.method === 'POST') {
        return json({ error: { code: 'CONFLICT', message: 'VERSION_CONFLICT', details: { reason: 'VERSION_CONFLICT' } } }, 409)
      }
      reads += 1
      const arrival = reads === 1 ? '2026-10-07T08:00:00Z' : '2026-10-07T11:00:00Z'
      return json(stopsResponse(task({ plannedArrival: arrival, version: reads === 1 ? 3 : 9 })))
    })
    await wrapper.get('[data-testid="arrive"]').trigger('click')
    await flushPromises()
    expect(wrapper.get('[data-testid="notice"]').text()).toBe('Состояние маршрута изменилось. Данные обновлены.')
    expect(wrapper.text()).toContain('2026-10-07T11:00:00Z')
    expect(reads).toBe(2)
  })
})

describe('401_REDIRECT_LOGIN', () => {
  it('logs out and opens the login route', async () => {
    const { wrapper, auth, router } = await mountStops(() =>
      json({ error: { code: 'UNAUTHORIZED', message: 'bad token' } }, 401),
    )
    expect(router.currentRoute.value.path).toBe('/login')
    expect(auth.token).toBeNull()
    expect(wrapper.text()).not.toContain('stack')
  })
})

describe('stops navigation', () => {
  it('opens stops from the shipments screen', async () => {
    installFetch(() => json({ items: [], total: 0 }))
    const pinia = createPinia()
    setActivePinia(pinia)
    const auth = useAuthStore()
    auth.token = 'token'
    auth.user = {
      id: 'user-1',
      tenant_id: 'tenant-1',
      email: 'driver@example.com',
      full_name: 'Driver',
      status: 'ACTIVE',
    }
    useNetworkStore().online = true
    const router = createRouter({
      history: createMemoryHistory(),
      routes: [
        { path: '/shipments', component: ShipmentsPage },
        { path: '/stops', component: { template: '<div>stops</div>' } },
        { path: '/login', component: { template: '<div>login</div>' } },
      ],
    })
    await router.push('/shipments')
    await router.isReady()
    const wrapper = mount(ShipmentsPage, { global: { plugins: [pinia, router, i18n] } })
    await flushPromises()
    await wrapper.get('[data-testid="nav-stops"]').trigger('click')
    await flushPromises()
    expect(router.currentRoute.value.path).toBe('/stops')
  })
})

describe('MULTI_SHIPMENT_STOP', () => {
  function assertActionIdentity(body: Record<string, unknown> | undefined, shipmentId: string, cargoId: string, otherShipmentId: string) {
    expect(body?.shipmentId).toBe(shipmentId)
    expect(body?.cargoId).toBe(cargoId)
    expect(body?.shipmentId).not.toBe(otherShipmentId)
    expect(body?.shipmentId).not.toBe(NIL_UUID)
    expect(body?.shipmentId).not.toBe('')
    expect(body?.shipmentId).not.toBeNull()
    expect(body).not.toHaveProperty('executionId')
    expect(body).not.toHaveProperty('revisionId')
    expect(body).not.toHaveProperty('actorId')
    expect(body).not.toHaveProperty('actorKind')
    expect(body).not.toHaveProperty('tenantId')
    expect(body).not.toHaveProperty('driverId')
    expect(body).not.toHaveProperty('operatingTenantId')
  }

  it('renders both delivery forms when the stop shipment id is null', async () => {
    const { wrapper } = await mountStops(() => json(stopsResponse(multiShipmentStop(), null)))
    const current = wrapper.get('[data-testid="current-stop"]')
    expect(current.text()).toContain('SERVICE_STARTED')
    expect(current.text()).toContain('—')
    expect(current.text()).toContain(SHIPMENT_S1)
    expect(current.text()).toContain(SHIPMENT_S2)
    expect(wrapper.find(`[data-testid="disposition-form-${ACTION_A}"]`).exists()).toBe(true)
    expect(wrapper.find(`[data-testid="disposition-form-${ACTION_B}"]`).exists()).toBe(true)
    expect(wrapper.find('[data-testid="authorize-return"]').exists()).toBe(false)
    expect(wrapper.find('[data-testid="authorize-redirect"]').exists()).toBe(false)
    expect(current.text()).not.toContain(NIL_UUID)
  })

  it('accepts action A with its own shipment and cargo', async () => {
    const { wrapper } = await mountStops((_url, init) => {
      if (init?.method === 'POST') return json(dispositionResult(2, 0))
      return json(stopsResponse(multiShipmentStop(), null))
    })
    await wrapper.get(`[data-testid="accepted-${ACTION_A}"]`).setValue('2')
    await wrapper.get(`[data-testid="disposition-form-${ACTION_A}"]`).trigger('submit')
    await flushPromises()
    const posted = sentRequests().find((call) => call.method === 'POST')
    expect(posted?.url).toContain(`/actions/${ACTION_A}/delivery-disposition`)
    expect(posted?.url).not.toContain(ACTION_B)
    assertActionIdentity(posted?.body, SHIPMENT_S1, CARGO_C1, SHIPMENT_S2)
    expect(posted?.body).toMatchObject({ acceptedQuantity: 2, rejectedQuantity: 0, uom: 'PALLET', evidence: [] })
    expect(posted?.body).not.toHaveProperty('reasonCode')
    expect(posted?.headers['X-Tenant-ID']).toBeUndefined()
    expect(posted?.headers['X-Driver-ID']).toBeUndefined()
    expect(wrapper.get('[data-testid="notice"]').text()).toBe('Результат доставки зарегистрирован')
  })

  it('requires a reason for a partial rejection of action A', async () => {
    const { wrapper } = await mountStops((_url, init) => {
      if (init?.method === 'POST') return json(dispositionResult(1, 1))
      return json(stopsResponse(multiShipmentStop(), null))
    })
    await wrapper.get(`[data-testid="accepted-${ACTION_A}"]`).setValue('1')
    await wrapper.get(`[data-testid="rejected-${ACTION_A}"]`).setValue('1')
    await wrapper.get(`[data-testid="disposition-form-${ACTION_A}"]`).trigger('submit')
    await flushPromises()
    expect(sentRequests().some((call) => call.method === 'POST')).toBe(false)
    await wrapper.get(`[data-testid="disposition-reason-${ACTION_A}"]`).setValue('DAMAGE')
    await wrapper.get(`[data-testid="disposition-form-${ACTION_A}"]`).trigger('submit')
    await flushPromises()
    const posted = sentRequests().find((call) => call.method === 'POST')
    expect(posted?.url).toContain(`/actions/${ACTION_A}/delivery-disposition`)
    assertActionIdentity(posted?.body, SHIPMENT_S1, CARGO_C1, SHIPMENT_S2)
    expect(posted?.body).toMatchObject({ acceptedQuantity: 1, rejectedQuantity: 1, reasonCode: 'DAMAGE', uom: 'PALLET' })
  })

  it('rejects action B with its own shipment and cargo', async () => {
    const { wrapper } = await mountStops((_url, init) => {
      if (init?.method === 'POST') return json(dispositionResult(0, 3))
      return json(stopsResponse(multiShipmentStop(), null))
    })
    await wrapper.get(`[data-testid="rejected-${ACTION_B}"]`).setValue('3')
    await wrapper.get(`[data-testid="disposition-reason-${ACTION_B}"]`).setValue('SHORTAGE')
    await wrapper.get(`[data-testid="disposition-form-${ACTION_B}"]`).trigger('submit')
    await flushPromises()
    const posted = sentRequests().find((call) => call.method === 'POST')
    expect(posted?.url).toContain(`/actions/${ACTION_B}/delivery-disposition`)
    expect(posted?.url).not.toContain(ACTION_A)
    assertActionIdentity(posted?.body, SHIPMENT_S2, CARGO_C2, SHIPMENT_S1)
    expect(posted?.body).toMatchObject({ acceptedQuantity: 0, rejectedQuantity: 3, reasonCode: 'SHORTAGE', uom: 'PALLET' })
  })

  it('still requires a comment when the multi-shipment reason is other', async () => {
    const { wrapper } = await mountStops((_url, init) => {
      if (init?.method === 'POST') return json(dispositionResult(0, 1))
      return json(stopsResponse(multiShipmentStop(), null))
    })
    await wrapper.get(`[data-testid="rejected-${ACTION_A}"]`).setValue('1')
    await wrapper.get(`[data-testid="disposition-reason-${ACTION_A}"]`).setValue('OTHER')
    await wrapper.get(`[data-testid="disposition-form-${ACTION_A}"]`).trigger('submit')
    await flushPromises()
    expect(sentRequests().some((call) => call.method === 'POST')).toBe(false)
    expect(wrapper.get('[data-testid="notice"]').text()).toContain('комментарий')
    await wrapper.get(`[data-testid="disposition-comment-${ACTION_A}"]`).setValue('seal broken')
    await wrapper.get(`[data-testid="disposition-form-${ACTION_A}"]`).trigger('submit')
    await flushPromises()
    const posted = sentRequests().find((call) => call.method === 'POST')
    assertActionIdentity(posted?.body, SHIPMENT_S1, CARGO_C1, SHIPMENT_S2)
    expect(posted?.body).toMatchObject({ reasonCode: 'OTHER', reasonComment: 'seal broken' })
  })

  it('reuses the stored action identity when the response is unknown', async () => {
    const keys: string[] = []
    const bodies: Record<string, unknown>[] = []
    let posts = 0
    const { wrapper } = await mountStops((_url, init) => {
      if (init?.method === 'POST') {
        posts += 1
        keys.push(((init.headers ?? {}) as Record<string, string>)['Idempotency-Key'] ?? '')
        bodies.push(JSON.parse(String(init.body)) as Record<string, unknown>)
        throw new TypeError('network down')
      }
      const stop = multiShipmentStop(posts > 0 ? SHIPMENT_S2 : null)
      return json(stopsResponse(stop, null))
    })
    await wrapper.get(`[data-testid="accepted-${ACTION_A}"]`).setValue('2')
    await wrapper.get(`[data-testid="disposition-form-${ACTION_A}"]`).trigger('submit')
    await flushPromises()
    expect(wrapper.get('[data-testid="notice"]').text()).toContain('неизвестен')
    await wrapper.get('[data-testid="retry"]').trigger('click')
    await flushPromises()
    expect(keys).toHaveLength(2)
    expect(keys[1]).toBe(keys[0])
    expect(bodies).toHaveLength(2)
    expect(bodies[1]).toEqual(bodies[0])
    assertActionIdentity(bodies[0], SHIPMENT_S1, CARGO_C1, SHIPMENT_S2)
    assertActionIdentity(bodies[1], SHIPMENT_S1, CARGO_C1, SHIPMENT_S2)
  })

  it('does not send a zero shipment id', async () => {
    const stop = multiShipmentStop()
    stop.actionSummary.actions[0].shipmentId = NIL_UUID
    const { wrapper } = await mountStops(() => json(stopsResponse(stop, null)))
    await wrapper.get(`[data-testid="accepted-${ACTION_A}"]`).setValue('1')
    await wrapper.get(`[data-testid="disposition-form-${ACTION_A}"]`).trigger('submit')
    await flushPromises()
    expect(sentRequests().some((call) => call.method === 'POST')).toBe(false)
    expect(wrapper.get('[data-testid="notice"]').text()).toContain('действия')
  })
})
