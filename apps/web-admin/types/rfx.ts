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

export const RFX_STATUSES = ['DRAFT', 'PUBLISHED', 'CANCELLED'] as const

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

export const BID_STATUSES = ['DRAFT', 'SUBMITTED', 'ACCEPTED', 'REJECTED'] as const

export const PARTICIPANT_TYPES = ['CARRIER', 'SHIPPER', 'FORWARDER', 'LSP'] as const

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

export interface FreightRequest {
  id: string
  tenant_id: string
  freight_request_number: string
  request_type: string
  shipper_company_id: string
  transport_order_id?: string | null
  status: string
  response_deadline?: string | null
  currency_code?: string | null
  created_at?: string
  updated_at?: string
  version?: number
}

export interface CreateFreightRequestFromOrderPayload {
  tenant_id: string
  transport_order_id: string
  freight_request_number: string
  request_type: string
  shipper_company_id: string
  response_deadline?: string
  currency_code?: string
}

export interface ListFreightRequestsFilters {
  request_type?: string
  status?: string
  shipper_company_id?: string
  search?: string
  limit?: number
  offset?: number
}

export interface BidItem {
  id?: string
  description?: string | null
  base_amount: number
  fuel_surcharge?: number
  toll_amount?: number
  extra_charges?: number
  amount_without_vat?: number
  vat_rate?: number | null
  vat_amount?: number
  amount_with_vat?: number
  comment?: string | null
}

export interface Bid {
  id: string
  tenant_id: string
  freight_request_id: string
  carrier_company_id: string
  bid_number: string
  status: string
  total_amount?: number
  currency_code?: string | null
  vat_rate?: number | null
  vat_amount?: number
  total_amount_with_vat?: number
  valid_until?: string | null
  submitted_at?: string | null
  items?: BidItem[]
  created_at?: string
  updated_at?: string
}

export interface CreateBidPayload {
  tenant_id: string
  carrier_company_id: string
  bid_number: string
  currency_code?: string
  vat_rate?: number
  valid_until?: string
  items: Array<{
    description?: string
    base_amount: number
    fuel_surcharge?: number
    toll_amount?: number
    extra_charges?: number
    vat_rate?: number
    comment?: string
  }>
}

export interface RfxFormErrors {
  rfx_number?: string
  rfx_type?: string
  category?: string
  title?: string
  owner_company_id?: string
  response_deadline?: string
  valid_to?: string
}

export interface FreightRequestFormErrors {
  transport_order_id?: string
  freight_request_number?: string
  request_type?: string
  shipper_company_id?: string
  response_deadline?: string
}

export interface BidFormErrors {
  carrier_company_id?: string
  bid_number?: string
  base_amount?: string
  vat_rate?: string
  currency_code?: string
  valid_until?: string
}

export interface RfxCreateForm {
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

export interface FreightRequestCreateForm {
  transport_order_id: string
  freight_request_number: string
  request_type: string
  shipper_company_id: string
  response_deadline: string
  currency_code: string
}

export interface BidCreateForm {
  carrier_company_id: string
  bid_number: string
  currency_code: string
  vat_rate: string
  valid_until: string
  base_amount: string
  fuel_surcharge: string
  toll_amount: string
  extra_charges: string
  comment: string
}

export function emptyCreateRfxForm(): RfxCreateForm {
  const deadline = defaultDeadlineLocal()
  const today = new Date().toISOString().slice(0, 10)
  const nextMonth = new Date(Date.now() + 30 * 86400000).toISOString().slice(0, 10)
  return {
    rfx_number: `RFX-${Date.now().toString().slice(-6)}`,
    rfx_type: 'RFQ',
    category: 'FREIGHT',
    title: '',
    description: '',
    owner_company_id: '',
    currency_code: 'RUB',
    valid_from: today,
    valid_to: nextMonth,
    response_deadline: deadline,
  }
}

export function emptyCreateFreightRequestForm(
  transportOrderId = '',
  shipperCompanyId = '',
): FreightRequestCreateForm {
  return {
    transport_order_id: transportOrderId,
    freight_request_number: `FR-${Date.now().toString().slice(-6)}`,
    request_type: 'MINI_TENDER',
    shipper_company_id: shipperCompanyId,
    response_deadline: defaultDeadlineLocal(),
    currency_code: 'RUB',
  }
}

export function emptyCreateBidForm(): BidCreateForm {
  return {
    carrier_company_id: '',
    bid_number: `BID-TEST-${Date.now().toString().slice(-6)}`,
    currency_code: 'RUB',
    vat_rate: '20',
    valid_until: defaultDeadlineLocal(),
    base_amount: '100000',
    fuel_surcharge: '5000',
    toll_amount: '3000',
    extra_charges: '0',
    comment: 'Цена за один рейс',
  }
}

function defaultDeadlineLocal(): string {
  const d = new Date()
  d.setDate(d.getDate() + 14)
  d.setHours(18, 0, 0, 0)
  const pad = (n: number) => String(n).padStart(2, '0')
  return `${d.getFullYear()}-${pad(d.getMonth() + 1)}-${pad(d.getDate())}T${pad(d.getHours())}:${pad(d.getMinutes())}`
}

function defaultDeadlineIso(): string {
  return new Date(defaultDeadlineLocal()).toISOString()
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

export function validateCreateRfxForm(payload: RfxCreateForm): RfxFormErrors {
  const errors: RfxFormErrors = {}
  if (!payload.rfx_number.trim()) errors.rfx_number = 'required'
  if (!payload.rfx_type.trim()) errors.rfx_type = 'required'
  if (!payload.category.trim()) errors.category = 'required'
  if (!payload.title.trim()) errors.title = 'required'
  if (!payload.owner_company_id.trim()) errors.owner_company_id = 'required'
  if (!payload.response_deadline?.trim()) errors.response_deadline = 'required'
  if (payload.valid_from && payload.valid_to && payload.valid_to < payload.valid_from) {
    errors.valid_to = 'range'
  }
  return errors
}

export function validateFreightRequestForm(payload: FreightRequestCreateForm): FreightRequestFormErrors {
  const errors: FreightRequestFormErrors = {}
  if (!payload.transport_order_id.trim()) errors.transport_order_id = 'required'
  if (!payload.freight_request_number.trim()) errors.freight_request_number = 'required'
  if (!payload.request_type.trim()) errors.request_type = 'required'
  if (!payload.shipper_company_id.trim()) errors.shipper_company_id = 'required'
  if (!payload.response_deadline?.trim()) errors.response_deadline = 'required'
  return errors
}

export function validateBidForm(form: BidCreateForm): BidFormErrors {
  const errors: BidFormErrors = {}
  if (!form.carrier_company_id.trim()) errors.carrier_company_id = 'required'
  if (!form.bid_number.trim()) errors.bid_number = 'required'
  if (!form.currency_code.trim()) errors.currency_code = 'required'
  if (!form.valid_until.trim()) errors.valid_until = 'required'
  const baseAmount = Number(form.base_amount)
  const vatRate = Number(form.vat_rate)
  if (Number.isNaN(baseAmount) || baseAmount < 0) errors.base_amount = 'negative'
  if (Number.isNaN(vatRate) || vatRate < 0) errors.vat_rate = 'negative'
  return errors
}

export function parseBidFormNumbers(form: BidCreateForm) {
  return {
    base_amount: Number(form.base_amount) || 0,
    fuel_surcharge: Number(form.fuel_surcharge) || 0,
    toll_amount: Number(form.toll_amount) || 0,
    extra_charges: Number(form.extra_charges) || 0,
    vat_rate: Number(form.vat_rate) || 0,
  }
}

export function hasFormErrors<T extends object>(errors: T): boolean {
  return Object.keys(errors).length > 0
}

export function formatRfxDate(value?: string | null): string {
  if (!value) return '—'
  const date = new Date(value)
  if (Number.isNaN(date.getTime())) return value
  return date.toLocaleString()
}

export function formatMoney(value?: number | null, currency?: string | null): string {
  if (value === undefined || value === null) return '—'
  const formatted = new Intl.NumberFormat('ru-RU', {
    minimumFractionDigits: 0,
    maximumFractionDigits: 2,
  }).format(value)
  return currency ? `${formatted} ${currency}` : formatted
}
