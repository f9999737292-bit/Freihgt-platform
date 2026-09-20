import { isDeadlineExpired } from '~/types/carrierRfx'
import type { LateSubmissionRequest } from '~/types/lateSubmission'

export const LATE_SUBMISSION_CARRIER_ROLES = [
  'CARRIER_ADMIN',
  'CARRIER_DISPATCHER',
] as const

export const LATE_SUBMISSION_BUYER_READ_ROLES = [
  'PLATFORM_ADMIN',
  'PROCUREMENT_MANAGER',
  'SHIPPER_ADMIN',
  'SHIPPER_LOGIST',
  'FORWARDER_MANAGER',
] as const

export const LATE_SUBMISSION_BUYER_MANAGE_ROLES = [
  'PLATFORM_ADMIN',
  'PROCUREMENT_MANAGER',
  'SHIPPER_ADMIN',
  'FORWARDER_MANAGER',
] as const

export function hasLateSubmissionCarrierRole(roles: readonly string[]): boolean {
  return roles.some((role) => (LATE_SUBMISSION_CARRIER_ROLES as readonly string[]).includes(role))
}

export function hasLateSubmissionBuyerReadRole(roles: readonly string[]): boolean {
  return roles.some((role) => (LATE_SUBMISSION_BUYER_READ_ROLES as readonly string[]).includes(role))
}

export function hasLateSubmissionBuyerManageRole(roles: readonly string[]): boolean {
  return roles.some((role) => (LATE_SUBMISSION_BUYER_MANAGE_ROLES as readonly string[]).includes(role))
}

export function isApprovedWindowActive(
  from?: string | null,
  until?: string | null,
  now = Date.now(),
): boolean {
  if (!from || !until) return false
  const fromTs = new Date(from).getTime()
  const untilTs = new Date(until).getTime()
  if (Number.isNaN(fromTs) || Number.isNaN(untilTs)) return false
  return now >= fromTs && now < untilTs
}

export function latestOwnLateRequest(
  items: readonly LateSubmissionRequest[] | undefined,
): LateSubmissionRequest | null {
  if (!items?.length) return null
  return [...items].sort((a, b) => {
    const aTime = new Date(a.updated_at || a.created_at || 0).getTime()
    const bTime = new Date(b.updated_at || b.created_at || 0).getTime()
    return bTime - aTime
  })[0] ?? null
}

export function canShowCarrierLateRequestPanel(input: {
  lateSubmissionEnabled: boolean
  roles: readonly string[]
  deadline?: string | null
}): boolean {
  return input.lateSubmissionEnabled
    && hasLateSubmissionCarrierRole(input.roles)
    && isDeadlineExpired(input.deadline)
}

export function canCreateLateSubmissionRequest(input: {
  lateSubmissionEnabled: boolean
  roles: readonly string[]
  deadline?: string | null
  request?: LateSubmissionRequest | null
  now?: number
}): boolean {
  if (!canShowCarrierLateRequestPanel(input)) return false
  const status = String(input.request?.status || '').toUpperCase()
  return status !== 'REQUESTED' && status !== 'APPROVED'
}

export function canLateSubmitQuestionnaire(input: {
  lateSubmissionEnabled: boolean
  roles: readonly string[]
  deadline?: string | null
  responseStatus?: string | null
  request?: LateSubmissionRequest | null
  now?: number
}): boolean {
  if (!input.lateSubmissionEnabled || !hasLateSubmissionCarrierRole(input.roles)) return false
  if (!isDeadlineExpired(input.deadline, input.now)) return false
  if (String(input.responseStatus || '').toUpperCase() !== 'DRAFT') return false
  if (String(input.request?.status || '').toUpperCase() !== 'APPROVED') return false
  return isApprovedWindowActive(
    input.request?.approved_valid_from,
    input.request?.approved_valid_until,
    input.now,
  )
}

export function lateSubmitBlockReason(input: {
  deadline?: string | null
  responseStatus?: string | null
  request?: LateSubmissionRequest | null
  now?: number
}): 'deadline' | 'draft' | 'permission' | 'window_not_started' | 'window_expired' | null {
  if (!isDeadlineExpired(input.deadline, input.now)) return null
  if (String(input.responseStatus || '').toUpperCase() !== 'DRAFT') return 'draft'
  const status = String(input.request?.status || '').toUpperCase()
  if (status === 'EXPIRED') return 'window_expired'
  if (status !== 'APPROVED') return 'permission'
  const from = input.request?.approved_valid_from
  const until = input.request?.approved_valid_until
  const now = input.now ?? Date.now()
  if (from) {
    const fromTs = new Date(from).getTime()
    if (!Number.isNaN(fromTs) && now < fromTs) return 'window_not_started'
  }
  if (until) {
    const untilTs = new Date(until).getTime()
    if (!Number.isNaN(untilTs) && now >= untilTs) return 'window_expired'
  }
  if (!isApprovedWindowActive(from, until, now)) return 'permission'
  return null
}

export function canShowBuyerLateQueue(input: {
  lateSubmissionEnabled: boolean
  roles: readonly string[]
}): boolean {
  return input.lateSubmissionEnabled && hasLateSubmissionBuyerReadRole(input.roles)
}

export function canEnableQuestionnaireSubmitButton(input: {
  deadlineExpired: boolean
  lateSubmitAllowed: boolean
}): boolean {
  return !input.deadlineExpired || input.lateSubmitAllowed
}

export function canDecideLateSubmission(input: {
  lateSubmissionEnabled: boolean
  roles: readonly string[]
  request?: LateSubmissionRequest | null
}): boolean {
  return input.lateSubmissionEnabled
    && hasLateSubmissionBuyerManageRole(input.roles)
    && String(input.request?.status || '').toUpperCase() === 'REQUESTED'
}

export function lateSubmissionHumanJwtOperations(eventId = '{id}', requestId = '{request_id}') {
  return {
    create: `/api/v1/rfx-events/${eventId}/late-submission-requests`,
    mine: `/api/v1/rfx-events/${eventId}/late-submission-requests/mine`,
    buyerQueue: `/api/v1/rfx-events/${eventId}/late-submission-requests`,
    approve: `/api/v1/rfx-events/${eventId}/late-submission-requests/${requestId}/approve`,
    reject: `/api/v1/rfx-events/${eventId}/late-submission-requests/${requestId}/reject`,
    questionnaireSubmit: `/api/v1/rfx-events/${eventId}/carrier-response/submit`,
  }
}
