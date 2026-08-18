export const RFX_STATUSES = [
  'DRAFT',
  'PUBLISHED',
  'INVITATION_SENT',
  'QUESTIONS_OPEN',
  'RESPONSES_OPEN',
  'RESPONSES_CLOSED',
  'EVALUATION_IN_PROGRESS',
  'SHORTLISTED',
  'AWARDED',
  'PARTIALLY_AWARDED',
  'CANCELLED',
  'ARCHIVED',
] as const

export type RfxStatus = (typeof RFX_STATUSES)[number]

export const RFX_TYPES = [
  'RFI',
  'RFQ',
  'RFP',
  'RFG',
  'RFT',
  'SPOT_RFQ',
  'MINI_TENDER',
  'LANE_TENDER',
  'CONTRACT_TENDER',
  'SEASONAL_TENDER',
  'PROJECT_TENDER',
  'REVERSE_AUCTION',
] as const

export type RfxType = (typeof RFX_TYPES)[number]

export const RFX_CATEGORIES = [
  'FREIGHT',
  'WAREHOUSING',
  'CUSTOMS',
  'INSURANCE',
  'PACKAGING',
  'FUEL',
  'VEHICLE_SERVICE',
  'GOODS',
  'GENERAL_SERVICE',
] as const

export const PARTICIPANT_TYPES = ['CARRIER', 'SHIPPER', 'FORWARDER', 'LSP'] as const

export const SPOT_RFX_TYPES = ['SPOT_RFQ', 'MINI_TENDER'] as const

export interface RfxEvent {
  id: string
  tenant_id: string
  rfx_number: string
  rfx_type: string
  category: string
  title: string
  description?: string | null
  owner_company_id: string
  status: string
  currency_code?: string | null
  valid_from?: string | null
  valid_to?: string | null
  response_deadline?: string | null
  created_at?: string
  updated_at?: string
  version?: number
}

export interface CreateRfxEventPayload {
  tenant_id: string
  rfx_number: string
  rfx_type: string
  category: string
  title: string
  description?: string
  owner_company_id: string
  currency_code?: string
  valid_from?: string
  valid_to?: string
  response_deadline?: string
}

export interface UpdateRfxEventPayload {
  title?: string
  description?: string
  response_deadline?: string
  currency_code?: string
}

export interface ListRfxEventsFilters {
  rfx_type?: string
  category?: string
  status?: string
  owner_company_id?: string
  search?: string
  limit?: number
  offset?: number
}

export interface RfxParticipant {
  id: string
  tenant_id: string
  rfx_event_id: string
  company_id: string
  participant_type: string
  status: string
  invited_at?: string | null
}

export interface AddRfxParticipantPayload {
  tenant_id: string
  company_id: string
  participant_type: string
}

export interface RfxLot {
  id: string
  tenant_id: string
  rfx_event_id: string
  lot_number: string
  name: string
  description?: string | null
  category?: string | null
  estimated_value?: number | null
  currency_code?: string | null
  status: string
}

export interface CreateRfxLotPayload {
  tenant_id: string
  lot_number: string
  name: string
  description?: string
  category?: string
  estimated_value?: number
  currency_code?: string
}

export interface RfxLane {
  id: string
  tenant_id: string
  rfx_lot_id: string
  origin_location_id?: string | null
  destination_location_id?: string | null
  transport_mode: string
  equipment_type?: string | null
  estimated_volume?: number | null
  volume_unit?: string | null
  required_service_level?: string | null
}

export interface CreateRfxLanePayload {
  tenant_id: string
  origin_location_id: string
  destination_location_id: string
  transport_mode: string
  equipment_type?: string
  estimated_volume?: number
  volume_unit?: string
  required_service_level?: string
}

export interface TenderWizardForm {
  rfx_number: string
  rfx_type: string
  category: string
  title: string
  description: string
  owner_company_id: string
  currency_code: string
  valid_from: string
  valid_to: string
  response_deadline: string
}

export function isLongFormRfxType(rfxType: string): boolean {
  return !SPOT_RFX_TYPES.includes(rfxType as (typeof SPOT_RFX_TYPES)[number])
}

export function emptyTenderWizardForm(ownerCompanyId = ''): TenderWizardForm {
  const deadline = defaultDeadlineLocal()
  const today = new Date().toISOString().slice(0, 10)
  const nextMonth = new Date(Date.now() + 30 * 86400000).toISOString().slice(0, 10)
  return {
    rfx_number: `TND-${Date.now().toString().slice(-6)}`,
    rfx_type: 'LANE_TENDER',
    category: 'FREIGHT',
    title: '',
    description: '',
    owner_company_id: ownerCompanyId,
    currency_code: 'RUB',
    valid_from: today,
    valid_to: nextMonth,
    response_deadline: deadline,
  }
}

function defaultDeadlineLocal(): string {
  const d = new Date()
  d.setDate(d.getDate() + 14)
  d.setHours(18, 0, 0, 0)
  const pad = (n: number) => String(n).padStart(2, '0')
  return `${d.getFullYear()}-${pad(d.getMonth() + 1)}-${pad(d.getDate())}T${pad(d.getHours())}:${pad(d.getMinutes())}`
}

export function toDatetimeLocal(value?: string | null): string {
  if (!value) return ''
  const d = new Date(value)
  if (Number.isNaN(d.getTime())) return value
  const pad = (n: number) => String(n).padStart(2, '0')
  return `${d.getFullYear()}-${pad(d.getMonth() + 1)}-${pad(d.getDate())}T${pad(d.getHours())}:${pad(d.getMinutes())}`
}

export function toRFC3339(value: string): string {
  if (!value.trim()) return ''
  if (value.includes('T') && !value.endsWith('Z') && !value.includes('+')) {
    return new Date(value).toISOString()
  }
  return value
}

export function formatRfxDate(value?: string | null): string {
  if (!value) return '—'
  const date = new Date(value)
  if (Number.isNaN(date.getTime())) return value
  return date.toLocaleString()
}

export function isEditableStatus(status: string): boolean {
  return status === 'DRAFT'
}

export function canCancelStatus(status: string): boolean {
  return ['DRAFT', 'PUBLISHED', 'INVITATION_SENT', 'QUESTIONS_OPEN', 'RESPONSES_OPEN'].includes(status)
}

export function canPublishStatus(status: string): boolean {
  return status === 'DRAFT'
}

export const FREIGHT_REQUEST_TYPES = [
  'SPOT',
  'MINI_TENDER',
  'LANE_TENDER',
  'CONTRACT_TENDER',
  'SEASONAL_TENDER',
  'PROJECT_TENDER',
] as const

export const FREIGHT_REQUEST_STATUSES = [
  'DRAFT',
  'PUBLISHED',
  'RESPONSES_OPEN',
  'AWARDED',
] as const

export interface FreightRequest {
  id: string
  tenant_id: string
  freight_request_number: string
  request_type: string
  shipper_company_id: string
  status: string
  response_deadline?: string | null
  currency_code?: string | null
  transport_order_id?: string | null
  created_at?: string
  updated_at?: string
  version?: number
}

export interface ListFreightRequestsFilters {
  request_type?: string
  status?: string
  shipper_company_id?: string
  limit?: number
  offset?: number
}
