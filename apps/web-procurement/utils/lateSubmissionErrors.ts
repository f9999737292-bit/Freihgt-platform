import { ApiError } from '~/utils/apiClient'

export function classifyLateSubmissionHttpError(error: unknown): string {
  if (!(error instanceof ApiError)) return 'unavailable'
  if (error.status === 401) return 'unauthorized'
  if (error.status === 403) return 'forbidden'
  if (error.status === 404) return 'notFound'
  if (error.status === 409) {
    const field = String(error.details?.field || '')
    if (field === 'Idempotency-Key') return 'idempotencyConflict'
    if (field === 'expected_version' || field === 'status') return 'conflict'
    return 'conflict'
  }
  if (error.status === 422) {
    const field = String(error.details?.field || '')
    if (field === 'approved_valid_from') return 'windowNotStarted'
    if (field === 'approved_valid_until') return 'windowExpired'
    if (field === 'late_submission_request' || field === 'late_submission_status') return 'permissionRequired'
    return 'unprocessable'
  }
  if (error.status === 400) {
    const field = String(error.details?.field || '')
    if (field === 'response_deadline') return 'beforeDeadline'
    if (field === 'reason_code') return 'reasonCode'
    if (field === 'reason_text') return 'reasonText'
    if (field === 'requested_until') return 'requestedUntil'
    if (field === 'Idempotency-Key') return 'idempotencyRequired'
    return 'validation'
  }
  if (error.status === 0) return 'unavailable'
  return 'unavailable'
}
