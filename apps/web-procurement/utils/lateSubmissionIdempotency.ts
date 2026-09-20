export const LATE_SUBMISSION_IDEMPOTENCY_KEY_MAX_LENGTH = 128

export function assertLateSubmissionIdempotencyKey(key: string): string {
  const trimmed = key.trim()
  if (!trimmed) {
    throw new Error('Idempotency-Key is required')
  }
  if (trimmed.length > LATE_SUBMISSION_IDEMPOTENCY_KEY_MAX_LENGTH) {
    throw new Error('Idempotency-Key exceeds 128 characters')
  }
  return trimmed
}

export function createLateSubmissionIdempotencyStore() {
  const keys = new Map<string, string>()

  function remember(scope: string, prefix: string): string {
    const existing = keys.get(scope)
    if (existing) return existing
    const next = assertLateSubmissionIdempotencyKey(`${prefix}${scope}`)
    keys.set(scope, next)
    return next
  }

  function keyForCreate(eventId: string, carrierCompanyId: string): string {
    return remember(`${eventId}:${carrierCompanyId}`, 'late-create:')
  }

  function keyForApprove(requestId: string, version: number): string {
    return remember(`${requestId}:${version}:approve`, 'late-approve:')
  }

  function keyForReject(requestId: string, version: number): string {
    return remember(`${requestId}:${version}:reject`, 'late-reject:')
  }

  function keyForSubmit(responseId: string, saveVersion: number): string {
    return remember(`${responseId}:${saveVersion}:submit`, 'late-submit:')
  }

  function reset(): void {
    keys.clear()
  }

  return {
    keyForCreate,
    keyForApprove,
    keyForReject,
    keyForSubmit,
    reset,
  }
}
