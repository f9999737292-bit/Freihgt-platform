import type { CreateLateSubmissionRequestBody, LateSubmissionRequest } from '~/types/lateSubmission'

export const LATE_SUBMISSION_IDEMPOTENCY_KEY_MAX_LENGTH = 128

export const LATE_SUBMISSION_TERMINAL_STATUSES = ['REJECTED', 'EXPIRED', 'CONSUMED'] as const

export type LateSubmissionCreateAttempt = {
  key: string
  body: CreateLateSubmissionRequestBody
}

type LateSubmissionAttemptRef = Pick<LateSubmissionRequest, 'id' | 'status'>

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

export function isLateSubmissionTerminalStatus(status?: string | null): boolean {
  return (LATE_SUBMISSION_TERMINAL_STATUSES as readonly string[]).includes(
    String(status || '').toUpperCase(),
  )
}

export function lateCreateAttemptPhase(request?: LateSubmissionAttemptRef | null): string {
  const id = String(request?.id || '').trim()
  const status = String(request?.status || '').toUpperCase()
  if (id && isLateSubmissionTerminalStatus(status)) {
    return `after:${id}`
  }
  if (id && (status === 'REQUESTED' || status === 'APPROVED')) {
    return `active:${id}`
  }
  return 'open'
}

function newIdempotencyToken(): string {
  if (typeof crypto !== 'undefined' && typeof crypto.randomUUID === 'function') {
    return crypto.randomUUID()
  }
  return `k${Date.now().toString(36)}${Math.random().toString(36).slice(2, 12)}`
}

function newIdempotencyKey(prefix: string): string {
  return assertLateSubmissionIdempotencyKey(`${prefix}${newIdempotencyToken()}`)
}

function snapshotCreateBody(body: CreateLateSubmissionRequestBody): CreateLateSubmissionRequestBody {
  return {
    reason_code: body.reason_code,
    reason_text: body.reason_text,
    requested_until: body.requested_until,
  }
}

export function createLateSubmissionIdempotencyStore() {
  const keys = new Map<string, string>()
  const createAttempts = new Map<string, LateSubmissionCreateAttempt>()

  function remember(scope: string, prefix: string): string {
    const existing = keys.get(scope)
    if (existing) return existing
    const next = newIdempotencyKey(prefix)
    keys.set(scope, next)
    return next
  }

  function createScope(
    eventId: string,
    carrierCompanyId: string,
    latest?: LateSubmissionAttemptRef | null,
  ): string {
    return `create:${eventId}:${carrierCompanyId}:${lateCreateAttemptPhase(latest)}`
  }

  function keyForCreate(
    eventId: string,
    carrierCompanyId: string,
    latest?: LateSubmissionAttemptRef | null,
  ): string {
    return remember(createScope(eventId, carrierCompanyId, latest), 'late-create:')
  }

  function bindCreateAttempt(
    eventId: string,
    carrierCompanyId: string,
    latest: LateSubmissionAttemptRef | null | undefined,
    body: CreateLateSubmissionRequestBody,
  ): LateSubmissionCreateAttempt {
    const scope = createScope(eventId, carrierCompanyId, latest)
    const existing = createAttempts.get(scope)
    if (existing) return existing
    const next: LateSubmissionCreateAttempt = {
      key: remember(scope, 'late-create:'),
      body: snapshotCreateBody(body),
    }
    createAttempts.set(scope, next)
    return next
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
    createAttempts.clear()
  }

  return {
    keyForCreate,
    bindCreateAttempt,
    keyForApprove,
    keyForReject,
    keyForSubmit,
    reset,
  }
}
