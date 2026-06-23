import type { BillingRegister } from '~/types/billing'
import type { LowCodeEntityType, PreviewRuleContext } from '~/types/lowCode'
import type { Shipment } from '~/types/shipment'
import type { TransportOrder } from '~/types/transportOrder'

const MAX_STRING_LEN = 128

export interface LowCodeValidationContextRoute {
  from?: string
  to?: string
}

export interface LowCodeValidationContextDates {
  loading_date?: string
  delivery_date?: string
}

/** Compact entity snapshot sent as PUT validation_context (backend uses entity_status + role in v0.1). */
export interface LowCodeValidationContext extends PreviewRuleContext {
  entity_type?: string
  entity_id?: string
  cargo_type?: string
  route?: LowCodeValidationContextRoute
  dates?: LowCodeValidationContextDates
  transport_order_id?: string
  carrier_id?: string
  driver_id?: string
  vehicle_id?: string
  period?: string
  amount?: string | number
  currency?: string
}

export interface ShipmentValidationContextOptions {
  route?: LowCodeValidationContextRoute
}

function trimString(value: unknown): string | undefined {
  if (value == null) return undefined
  const text = String(value).trim()
  if (!text) return undefined
  return text.length > MAX_STRING_LEN ? text.slice(0, MAX_STRING_LEN) : text
}

function compactRoute(route?: LowCodeValidationContextRoute): LowCodeValidationContextRoute | undefined {
  if (!route) return undefined
  const from = trimString(route.from)
  const to = trimString(route.to)
  if (!from && !to) return undefined
  return { ...(from ? { from } : {}), ...(to ? { to } : {}) }
}

function compactDates(dates?: LowCodeValidationContextDates): LowCodeValidationContextDates | undefined {
  if (!dates) return undefined
  const loading_date = trimString(dates.loading_date)
  const delivery_date = trimString(dates.delivery_date)
  if (!loading_date && !delivery_date) return undefined
  return {
    ...(loading_date ? { loading_date } : {}),
    ...(delivery_date ? { delivery_date } : {}),
  }
}

function assignTrimmed(target: LowCodeValidationContext, key: keyof LowCodeValidationContext, value: unknown) {
  const trimmed = trimString(value)
  if (trimmed) {
    target[key] = trimmed as never
  }
}

/** Removes empty values and caps string length; keeps only route/dates nested objects. */
export function compactValidationContextForPut(
  context?: LowCodeValidationContext | PreviewRuleContext | null,
): LowCodeValidationContext | undefined {
  if (!context) return undefined

  const out: LowCodeValidationContext = {}
  assignTrimmed(out, 'entity_type', context.entity_type)
  assignTrimmed(out, 'entity_id', context.entity_id)
  assignTrimmed(out, 'entity_status', context.entity_status)
  assignTrimmed(out, 'role', context.role)
  assignTrimmed(out, 'cargo_type', context.cargo_type)
  assignTrimmed(out, 'transport_order_id', context.transport_order_id)
  assignTrimmed(out, 'carrier_id', context.carrier_id)
  assignTrimmed(out, 'driver_id', context.driver_id)
  assignTrimmed(out, 'vehicle_id', context.vehicle_id)
  assignTrimmed(out, 'period', context.period)
  assignTrimmed(out, 'currency', context.currency)

  if ('amount' in context && context.amount != null && context.amount !== '') {
    const amount =
      typeof context.amount === 'number' && Number.isFinite(context.amount)
        ? context.amount
        : trimString(context.amount)
    if (amount != null && amount !== '') {
      out.amount = amount
    }
  }

  const route = compactRoute('route' in context ? context.route : undefined)
  if (route) out.route = route

  const dates = compactDates('dates' in context ? context.dates : undefined)
  if (dates) out.dates = dates

  return out.entity_type ||
    out.entity_id ||
    out.entity_status ||
    out.role ||
    out.cargo_type ||
    out.route ||
    out.dates ||
    out.transport_order_id ||
    out.carrier_id ||
    out.driver_id ||
    out.vehicle_id ||
    out.period ||
    out.amount != null ||
    out.currency
    ? out
    : undefined
}

export function mergeLowCodeValidationContext(
  ...contexts: Array<LowCodeValidationContext | PreviewRuleContext | undefined | null>
): LowCodeValidationContext | undefined {
  const merged: LowCodeValidationContext = {}

  for (const context of contexts) {
    if (!context) continue
    if (context.entity_type?.trim()) merged.entity_type = context.entity_type.trim()
    if (context.entity_id?.trim()) merged.entity_id = context.entity_id.trim()
    if (context.entity_status?.trim()) merged.entity_status = context.entity_status.trim()
    if (context.role?.trim()) merged.role = context.role.trim()
    if ('cargo_type' in context && context.cargo_type?.trim()) {
      merged.cargo_type = context.cargo_type.trim()
    }
    if ('transport_order_id' in context && context.transport_order_id?.trim()) {
      merged.transport_order_id = context.transport_order_id.trim()
    }
    if ('carrier_id' in context && context.carrier_id?.trim()) {
      merged.carrier_id = context.carrier_id.trim()
    }
    if ('driver_id' in context && context.driver_id?.trim()) {
      merged.driver_id = context.driver_id.trim()
    }
    if ('vehicle_id' in context && context.vehicle_id?.trim()) {
      merged.vehicle_id = context.vehicle_id.trim()
    }
    if ('period' in context && context.period?.trim()) {
      merged.period = context.period.trim()
    }
    if ('currency' in context && context.currency?.trim()) {
      merged.currency = context.currency.trim()
    }
    if ('amount' in context && context.amount != null && context.amount !== '') {
      merged.amount = context.amount
    }
    if ('route' in context && context.route) {
      merged.route = { ...merged.route, ...context.route }
    }
    if ('dates' in context && context.dates) {
      merged.dates = { ...merged.dates, ...context.dates }
    }
  }

  return compactValidationContextForPut(merged)
}

export function buildTransportOrderValidationContext(
  order?: TransportOrder | null,
): LowCodeValidationContext | undefined {
  if (!order?.id?.trim()) return undefined

  const loadingDate = order.pickup_date || order.requested_pickup_date
  const deliveryDate = order.delivery_date || order.requested_delivery_date

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
      loading_date: loadingDate?.trim() || undefined,
      delivery_date: deliveryDate?.trim() || undefined,
    }),
  })
}

export function buildShipmentValidationContext(
  shipment?: Shipment | null,
  options?: ShipmentValidationContextOptions,
): LowCodeValidationContext | undefined {
  if (!shipment?.id?.trim()) return undefined

  const routeFromOptions = compactRoute(options?.route)
  const routeFromEntity = compactRoute({
    from: shipment.origin_location_id?.trim() || undefined,
    to: shipment.destination_location_id?.trim() || undefined,
  })

  return compactValidationContextForPut({
    entity_type: 'SHIPMENT',
    entity_id: shipment.id.trim(),
    entity_status: shipment.status?.trim() || undefined,
    transport_order_id: shipment.transport_order_id?.trim() || undefined,
    carrier_id: shipment.carrier_company_id?.trim() || undefined,
    driver_id: shipment.driver_id?.trim() || undefined,
    vehicle_id: shipment.vehicle_id?.trim() || undefined,
    route: routeFromOptions ?? routeFromEntity,
    dates: compactDates({
      loading_date: shipment.planned_pickup_at?.trim() || shipment.actual_pickup_at?.trim() || undefined,
      delivery_date: shipment.planned_delivery_at?.trim() || shipment.actual_delivery_at?.trim() || undefined,
    }),
  })
}

export function buildBillingRegisterValidationContext(
  register?: BillingRegister | null,
): LowCodeValidationContext | undefined {
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
    amount: register.total_with_vat ?? register.total_without_vat,
    currency: register.currency_code?.trim() || undefined,
  })
}

export function buildLowCodeValidationContext(
  entityType: LowCodeEntityType | string,
  entity: unknown,
  options?: ShipmentValidationContextOptions,
): LowCodeValidationContext | undefined {
  const normalized = String(entityType).trim().toUpperCase()
  switch (normalized) {
    case 'TRANSPORT_ORDER':
      return buildTransportOrderValidationContext(entity as TransportOrder)
    case 'SHIPMENT':
      return buildShipmentValidationContext(entity as Shipment, options)
    case 'BILLING_REGISTER':
      return buildBillingRegisterValidationContext(entity as BillingRegister)
    default:
      return undefined
  }
}

/** True when context contains only safe scalar/route/dates fields (no unexpected nested blobs). */
export function isSafeValidationContextShape(context: LowCodeValidationContext): boolean {
  for (const [key, value] of Object.entries(context)) {
    if (value == null) continue
    if (key === 'route' || key === 'dates') {
      if (typeof value !== 'object' || Array.isArray(value)) return false
      for (const nested of Object.values(value as Record<string, unknown>)) {
        if (nested != null && typeof nested !== 'string') return false
      }
      continue
    }
    if (key === 'amount') {
      if (typeof value !== 'string' && typeof value !== 'number') return false
      continue
    }
    if (typeof value !== 'string') return false
  }
  return true
}
