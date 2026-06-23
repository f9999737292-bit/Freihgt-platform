/**
 * Lightweight verification for low-code validation_context helper contracts.
 * Run: node scripts/dev/verify_lowcode_validation_context.mjs
 */

const MAX_STRING_LEN = 128

function trimString(value) {
  if (value == null) return undefined
  const text = String(value).trim()
  if (!text) return undefined
  return text.length > MAX_STRING_LEN ? text.slice(0, MAX_STRING_LEN) : text
}

function compactRoute(route) {
  if (!route) return undefined
  const from = trimString(route.from)
  const to = trimString(route.to)
  if (!from && !to) return undefined
  return { ...(from ? { from } : {}), ...(to ? { to } : {}) }
}

function compactDates(dates) {
  if (!dates) return undefined
  const loading_date = trimString(dates.loading_date)
  const delivery_date = trimString(dates.delivery_date)
  if (!loading_date && !delivery_date) return undefined
  return {
    ...(loading_date ? { loading_date } : {}),
    ...(delivery_date ? { delivery_date } : {}),
  }
}

function compactValidationContextForPut(context) {
  if (!context) return undefined
  const out = {}
  for (const key of [
    'entity_type',
    'entity_id',
    'entity_status',
    'role',
    'cargo_type',
    'transport_order_id',
    'carrier_id',
    'driver_id',
    'vehicle_id',
    'period',
    'currency',
  ]) {
    const trimmed = trimString(context[key])
    if (trimmed) out[key] = trimmed
  }
  if (context.amount != null && context.amount !== '') {
    out.amount = context.amount
  }
  const route = compactRoute(context.route)
  if (route) out.route = route
  const dates = compactDates(context.dates)
  if (dates) out.dates = dates
  return Object.keys(out).length ? out : undefined
}

function buildTransportOrderValidationContext(order) {
  if (!order?.id?.trim()) return undefined
  return compactValidationContextForPut({
    entity_type: 'TRANSPORT_ORDER',
    entity_id: order.id.trim(),
    entity_status: order.status?.trim() || undefined,
    cargo_type: order.equipment_type?.trim() || undefined,
    route: compactRoute({
      from: order.origin_location_id?.trim() || undefined,
      to: order.destination_location_id?.trim() || undefined,
    }),
    dates: compactDates({
      loading_date: order.pickup_date?.trim() || order.requested_pickup_date?.trim() || undefined,
      delivery_date: order.delivery_date?.trim() || order.requested_delivery_date?.trim() || undefined,
    }),
  })
}

function buildShipmentValidationContext(shipment, options) {
  if (!shipment?.id?.trim()) return undefined
  return compactValidationContextForPut({
    entity_type: 'SHIPMENT',
    entity_id: shipment.id.trim(),
    entity_status: shipment.status?.trim() || undefined,
    transport_order_id: shipment.transport_order_id?.trim() || undefined,
    carrier_id: shipment.carrier_company_id?.trim() || undefined,
    driver_id: shipment.driver_id?.trim() || undefined,
    vehicle_id: shipment.vehicle_id?.trim() || undefined,
    route: compactRoute(options?.route),
    dates: compactDates({
      loading_date: shipment.planned_pickup_at?.trim() || undefined,
      delivery_date: shipment.planned_delivery_at?.trim() || undefined,
    }),
  })
}

function buildBillingRegisterValidationContext(register) {
  if (!register?.id?.trim()) return undefined
  const periodFrom = register.period_from?.trim()
  const periodTo = register.period_to?.trim()
  const period =
    periodFrom && periodTo ? `${periodFrom}..${periodTo}` : periodFrom || periodTo || undefined
  return compactValidationContextForPut({
    entity_type: 'BILLING_REGISTER',
    entity_id: register.id.trim(),
    entity_status: register.status?.trim() || undefined,
    period,
    amount: register.total_with_vat,
    currency: register.currency_code?.trim() || undefined,
  })
}

function assert(condition, message) {
  if (!condition) {
    throw new Error(message)
  }
}

function assertNoLargeNestedLeakage(context) {
  for (const [key, value] of Object.entries(context)) {
    if (value == null) continue
    if (key === 'route' || key === 'dates') {
      assert(typeof value === 'object' && !Array.isArray(value), `nested ${key} must be plain object`)
      for (const nested of Object.values(value)) {
        assert(typeof nested === 'string', `nested ${key} values must be strings`)
        assert(nested.length <= MAX_STRING_LEN, `nested ${key} value too long`)
      }
      continue
    }
    if (key === 'amount') {
      assert(typeof value === 'string' || typeof value === 'number', 'amount must be scalar')
      continue
    }
    assert(typeof value === 'string', `${key} must be string`)
    assert(value.length <= MAX_STRING_LEN, `${key} too long`)
  }
}

function run() {
  const transport = buildTransportOrderValidationContext({
    id: '2db04b49-665c-469f-bcb1-ffeb1274fedb',
    status: 'READY_FOR_SOURCING',
    equipment_type: 'TENT_20T',
    origin_location_id: '50858945-7115-49b1-bbfc-4d4b9ee897a6',
    destination_location_id: 'e19ca415-0611-4cf3-b119-9cc8867d4346',
    pickup_date: '2026-07-01',
    delivery_date: '2026-07-05',
  })
  assert(transport?.entity_type === 'TRANSPORT_ORDER', 'transport entity_type')
  assert(transport?.entity_status === 'READY_FOR_SOURCING', 'transport status')
  assert(transport?.cargo_type === 'TENT_20T', 'transport cargo_type')
  assert(transport?.route?.from, 'transport route.from')
  assertNoLargeNestedLeakage(transport)

  const shipment = buildShipmentValidationContext(
    {
      id: '14d405e2-0152-4030-b356-eec464a3cc66',
      status: 'IN_TRANSIT',
      transport_order_id: '2db04b49-665c-469f-bcb1-ffeb1274fedb',
      carrier_company_id: '0ef7d4ee-5879-4eb6-b1d0-dd1b0e6f9b43',
      driver_id: 'ad07e667-fa1c-4162-aa45-46afa206a0d4',
      vehicle_id: '94218566-c6c0-4889-8e30-d4b4765a1285',
      planned_pickup_at: '2026-07-02T08:00:00Z',
      planned_delivery_at: '2026-07-06T18:00:00Z',
    },
    { route: { from: 'Склад Москва', to: 'Склад Казань' } },
  )
  assert(shipment?.entity_type === 'SHIPMENT', 'shipment entity_type')
  assert(shipment?.transport_order_id, 'shipment transport_order_id')
  assert(shipment?.route?.from === 'Склад Москва', 'shipment route label')
  assertNoLargeNestedLeakage(shipment)

  const billing = buildBillingRegisterValidationContext({
    id: 'cf7dbc77-395f-42a2-9717-476e4cd93796',
    status: 'DRAFT',
    period_from: '2026-06-01',
    period_to: '2026-06-30',
    total_with_vat: 126000,
    currency_code: 'RUB',
  })
  assert(billing?.entity_type === 'BILLING_REGISTER', 'billing entity_type')
  assert(billing?.period === '2026-06-01..2026-06-30', 'billing period')
  assert(billing?.amount === 126000, 'billing amount')
  assertNoLargeNestedLeakage(billing)

  assert(buildTransportOrderValidationContext(null) === undefined, 'missing order safe fallback')
  assert(buildTransportOrderValidationContext({ id: '  ' }) === undefined, 'blank id safe fallback')

  const stripped = compactValidationContextForPut({
    entity_type: 'TRANSPORT_ORDER',
    entity_id: 'abc',
    entity_status: 'DRAFT',
    secret_payload: { huge: 'should-not-appear' },
  })
  assert(!('secret_payload' in (stripped ?? {})), 'unknown nested keys stripped')

  console.log('verify_lowcode_validation_context: OK')
}

run()
