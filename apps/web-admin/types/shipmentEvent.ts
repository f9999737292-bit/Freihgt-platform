export interface ShipmentEventActor {
  type: string
  id?: string | null
  name?: string | null
}

export interface ShipmentTimelineEvent {
  id: string
  shipmentId: string
  shipmentNumber: string
  type: string
  category: string
  source: string
  severity: string
  titleCode: string
  descriptionCode?: string | null
  occurredAt: string
  recordedAt?: string | null
  actor?: ShipmentEventActor | null
  metadata?: Record<string, unknown> | null
  derived: boolean
  correlationId?: string | null
  sourceEventId?: string | null
}

export interface ShipmentEventsDataFreshness {
  shipmentLoaded: boolean
  shipmentEventsLoaded: boolean
  documentsLoaded: boolean
  billingLoaded: boolean
  technicalEventsLoaded: boolean
  partial: boolean
  warnings: string[]
}

export interface ShipmentEventsTimelinePage {
  items: ShipmentTimelineEvent[]
  page: number
  limit: number
  total: number
  hasNext: boolean
}

export interface ShipmentEventFilterOption {
  value: string
  label: string
}

export interface ShipmentEventsFiltersResponse {
  types: ShipmentEventFilterOption[]
  categories: ShipmentEventFilterOption[]
  sources: ShipmentEventFilterOption[]
  severities: ShipmentEventFilterOption[]
}

export interface ShipmentEventsResponse {
  shipment: {
    id: string
    number: string
    status: string
  }
  generatedAt: string
  dataFreshness: ShipmentEventsDataFreshness
  timeline: ShipmentEventsTimelinePage
  filters: ShipmentEventsFiltersResponse
}

export interface ShipmentEventQueryFilters {
  type?: string
  category?: string
  source?: string
  severity?: string
  date_from?: string
  date_to?: string
  derived?: string
  order?: 'asc' | 'desc'
  page?: number
  limit?: number
}

export const SHIPMENT_EVENT_CATEGORIES = [
  'SHIPMENT',
  'OPERATION',
  'DOCUMENT',
  'SLA',
  'BILLING',
  'TECHNICAL',
  'GEOLOCATION',
  'SYSTEM',
] as const

export const SHIPMENT_EVENT_SEVERITIES = ['INFO', 'WARNING', 'CRITICAL'] as const

export const SHIPMENT_EVENT_SOURCES = [
  'SHIPMENT_STATE',
  'SLA_CALCULATOR',
  'DOCUMENT_STATE',
  'BILLING_STATE',
] as const

export function titleCodeToI18nKey(code: string): string {
  const match = code.match(/^shipment\.timeline\.([A-Z0-9_]+)\.title$/)
  if (!match) return code
  return `shipmentEvents.types.${match[1]}.title`
}

export function descriptionCodeToI18nKey(code: string): string {
  const match = code.match(/^shipment\.timeline\.([A-Z0-9_]+)\.description$/)
  if (!match) return code
  return `shipmentEvents.types.${match[1]}.description`
}

export function formatEventDateTime(value?: string | null): string {
  if (!value) return '—'
  const date = new Date(value)
  if (Number.isNaN(date.getTime())) return value
  return date.toLocaleString()
}
