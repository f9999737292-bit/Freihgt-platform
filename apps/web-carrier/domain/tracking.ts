/** Customer-safe carrier tracking, ETA, and slot facts. Provider correlation ids are not part of this contract. */

export const HISTORY_PAGE_SIZE = 50

export const FORBIDDEN_BUSINESS_FACT_KEYS = [
  'plannedPickupAt',
  'plannedDeliveryAt',
  'actualPickupAt',
  'actualDeliveryAt',
  'shipmentStatus',
  'pickupEtaStatus',
  'deliveryEtaStatus',
  'pickupEstimatedArrivalAt',
  'deliveryEstimatedArrivalAt',
  'pickupEtaFreshness',
  'deliveryEtaFreshness',
  'pickupEtaQuality',
  'deliveryEtaQuality',
  'windowStart',
  'windowEnd',
  'windowStatus',
  'slotStatus',
  'bookedAt',
  'confirmedAt',
  'etaRelation',
] as const

export interface QualityFact {
  status?: string
}

export interface LastKnownPosition {
  latitude?: number
  longitude?: number
  recordedAt?: string
  ageSeconds?: number
}

export interface TrackingSummary {
  shipmentId?: string
  trackingStatus?: string
  freshness?: { status?: string; ageSeconds?: number }
  quality?: QualityFact
  lastKnownPosition?: LastKnownPosition | null
  lastRecordedAt?: string
  lastReceivedAt?: string
  speedKph?: number
  headingDegrees?: number
  deliveryDelaySeconds?: number
}

export interface LocationFact {
  shipmentId?: string
  latitude?: number
  longitude?: number
  recordedAt?: string
  receivedAt?: string
  sourceType?: string
  speedKph?: number
  headingDegrees?: number
  accuracyMeters?: number
  quality?: QualityFact
}

export interface ETATarget {
  status?: string
  freshnessStatus?: string
  qualityStatus?: string
  arrivalProjection?: string
  estimatedArrivalAt?: string
  sourceType?: string
  sourceObservedAt?: string
  receivedAt?: string
  ageSeconds?: number
  deliveryLagSeconds?: number
  plannedArrivalAt?: string
  projectedDeviationSeconds?: number
  qualityReasons?: string[]
}

export interface ETASummary {
  shipmentId?: string
  pickup?: ETATarget | null
  delivery?: ETATarget | null
}

export interface ETAHistoryItem {
  shipmentId?: string
  targetType?: string
  estimatedArrivalAt?: string
  sourceType?: string
  sourceObservedAt?: string
  receivedAt?: string
  qualityStatus?: string
  qualityReasons?: string[]
}

export interface SlotTarget {
  windowStatus?: string
  slotStatus?: string
  windowStart?: string
  windowEnd?: string
  timezone?: string
  qualityStatus?: string
  arrivalProjection?: string
  etaRelation?: string
  sourceType?: string
  bookedAt?: string
  confirmedAt?: string
  qualityReasons?: string[]
}

export interface SlotSummary {
  shipmentId?: string
  pickup?: SlotTarget | null
  delivery?: SlotTarget | null
}

export interface SlotHistoryItem {
  shipmentId?: string
  slotType?: string
  windowStart?: string
  windowEnd?: string
  slotStatus?: string
  sourceType?: string
  sourceObservedAt?: string
  receivedAt?: string
  qualityStatus?: string
  timezone?: string
  qualityReasons?: string[]
}

export interface HistoryPage<T> {
  items?: T[]
  total?: number
  limit?: number
  offset?: number
}

export interface HistoryQuery {
  from?: string
  to?: string
  limit?: number
  offset?: number
  targetType?: 'pickup' | 'delivery'
  slotType?: 'pickup' | 'delivery'
}

export const CARRIER_TRACKING_SUFFIXES = [
  '/tracking',
  '/tracking/locations',
  '/eta',
  '/eta/history',
  '/slots',
  '/slots/history',
] as const

export type CarrierTrackingSuffix = (typeof CARRIER_TRACKING_SUFFIXES)[number]

export function displayValue(value: string | number | null | undefined): string {
  if (value === null || value === undefined) return '—'
  const text = String(value).trim()
  return text === '' ? '—' : text
}

export function isNotConfigured(status: string | undefined): boolean {
  return (status ?? '').trim().toUpperCase() === 'NOT_CONFIGURED'
}

export function carrierReadSpec(
  shipmentId: string,
  suffix: CarrierTrackingSuffix,
  companyId: string,
  history?: HistoryQuery,
): { path: string; query: Record<string, string> } {
  const id = shipmentId.trim()
  const company = companyId.trim()
  if (!id || !company) throw new Error('shipment company scope required')
  if (history) rejectBusinessFacts(history)
  const path = `/api/v1/carrier/shipments/${encodeURIComponent(id)}${suffix}`
  const query: Record<string, string> = {
    carrier_company_id: company,
    ...historyQuery(suffix, history),
  }
  rejectBusinessFacts(query)
  return { path, query }
}

function historyQuery(suffix: CarrierTrackingSuffix, history?: HistoryQuery): Record<string, string> {
  if (suffix !== '/tracking/locations' && suffix !== '/eta/history' && suffix !== '/slots/history') {
    return {}
  }
  const limit = history?.limit && history.limit > 0 ? Math.min(history.limit, HISTORY_PAGE_SIZE) : HISTORY_PAGE_SIZE
  const offset = history?.offset && history.offset > 0 ? history.offset : 0
  const query: Record<string, string> = {
    limit: String(limit),
    offset: String(offset),
  }
  const from = history?.from?.trim()
  const to = history?.to?.trim()
  if (from) query.from = from
  if (to) query.to = to
  if (suffix === '/eta/history') {
    if (history?.targetType !== 'pickup' && history?.targetType !== 'delivery') {
      throw new Error('eta target rejected')
    }
    query.targetType = history.targetType
  }
  if (suffix === '/slots/history') {
    if (history?.slotType !== 'pickup' && history?.slotType !== 'delivery') {
      throw new Error('slot type rejected')
    }
    query.slotType = history.slotType
  }
  return query
}

function rejectBusinessFacts(value: object) {
  for (const key of FORBIDDEN_BUSINESS_FACT_KEYS) {
    if (Object.prototype.hasOwnProperty.call(value, key)) {
      throw new Error('business fact query rejected')
    }
  }
}
