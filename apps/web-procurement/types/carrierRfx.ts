import type { RfxEvent } from '~/types/rfx'

export const CARRIER_RESPONSE_FILTERS = [
  'OPEN_FOR_RESPONSE',
  'RESPONDED',
  'NOT_RESPONDED',
  'CLOSED',
] as const

export type CarrierResponseFilter = (typeof CARRIER_RESPONSE_FILTERS)[number]

export const CARRIER_OWN_RESPONSE_STATUSES = [
  'NOT_STARTED',
  'DRAFT',
  'SUBMITTED',
] as const

export type CarrierOwnResponseStatus = (typeof CARRIER_OWN_RESPONSE_STATUSES)[number]

export interface CarrierInvitedTender extends RfxEvent {
  participant_status: string
  own_response_status: CarrierOwnResponseStatus | string
  own_response_id?: string | null
  lot_count: number
  participant_company_id: string
}

export interface CarrierRfxResponse {
  id: string
  tenant_id?: string
  rfx_event_id: string
  participant_company_id: string
  status: string
  submitted_at?: string | null
  created_at?: string
  updated_at?: string
  version?: number
  offer_lines?: Array<{
    id?: string
    rfx_lot_id?: string | null
    amount: number
    currency_code: string
    comment?: string | null
  }>
}

export function isDeadlineExpired(deadline?: string | null, now = Date.now()): boolean {
  if (!deadline) return false
  const ts = new Date(deadline).getTime()
  return !Number.isNaN(ts) && ts <= now
}

export function formatDeadlineRemaining(deadline?: string | null, now = Date.now()): string | null {
  if (!deadline) return null
  const ts = new Date(deadline).getTime()
  if (Number.isNaN(ts)) return null
  const diffMs = ts - now
  if (diffMs <= 0) return '0'
  const hours = Math.floor(diffMs / 3600000)
  const minutes = Math.floor((diffMs % 3600000) / 60000)
  if (hours >= 48) {
    const days = Math.floor(hours / 24)
    return `${days}d`
  }
  if (hours > 0) return `${hours}h ${minutes}m`
  return `${minutes}m`
}

export function canEditCommercial(status: string): boolean {
  return status === 'DRAFT'
}

export function canCreateResponse(
  eventStatus: string,
  ownResponseStatus: string,
  deadline?: string | null,
): boolean {
  if (ownResponseStatus !== 'NOT_STARTED') return false
  if (!['PUBLISHED', 'RESPONSES_OPEN'].includes(eventStatus)) return false
  return !isDeadlineExpired(deadline)
}

export function canSubmitResponse(
  ownResponseStatus: string,
  eventStatus: string,
  deadline?: string | null,
): boolean {
  if (ownResponseStatus !== 'DRAFT') return false
  if (!['PUBLISHED', 'RESPONSES_OPEN'].includes(eventStatus)) return false
  return !isDeadlineExpired(deadline)
}
