import type { DriverApiError } from '@/types/api'
import type {
  DriverDeliveryDispositionRequest,
  DriverDispositionReasonCode,
  DriverFailReasonCode,
  DriverStopCommandRequest,
  DriverStopFailRequest,
  DriverStopStatus,
  DriverStopTask,
} from '@/types/stops'
import { DISPOSITION_REASON_CODES, FAIL_REASON_CODES } from '@/types/stops'
import { createOperationId, isValidOperationId } from '@/utils/idempotency'

export type StopOperationKind = 'arrive' | 'start-service' | 'complete' | 'confirm' | 'fail' | 'disposition'

export const CLIENT_FORBIDDEN_FIELDS = [
  'actorId',
  'actorKind',
  'executionId',
  'revisionId',
  'operatingTenantId',
] as const

const CONFLICT_CODES = ['VERSION_CONFLICT', 'STALE_REVISION', 'TERMINAL_ACTION', 'STOP_NOT_CURRENT'] as const
const NIL_UUID = '00000000-0000-0000-0000-000000000000'

function usableIdentity(value: string | null | undefined): value is string {
  return typeof value === 'string' && value.trim() !== '' && value !== NIL_UUID
}

export type StopFailureKind =
  | 'unauthorized'
  | 'forbidden'
  | 'missing'
  | 'conflict'
  | 'action_pending'
  | 'validation'
  | 'server'

export interface DispositionDraft {
  accepted: number
  rejected: number
  reasonCode: string
  reasonComment: string
}

export type DispositionIssue = 'quantity' | 'reason' | 'comment' | 'shipment'

interface StoredCommand<T> {
  key: string
  body: T
}

function storageKey(kind: StopOperationKind, targetId: string): string {
  return `driver-mobile-stop-op:${kind}:${targetId}`
}

export function stopCommandId(stop: DriverStopTask): string {
  return stop.executionStopId
}

export function stopCommandVisibility(status: DriverStopStatus) {
  return {
    arrive: status === 'PLANNED',
    startService: status === 'ARRIVED',
    complete: status === 'SERVICE_STARTED',
    actions: status === 'SERVICE_STARTED',
  }
}

export function isFailReason(value: string): value is DriverFailReasonCode {
  return (FAIL_REASON_CODES as readonly string[]).includes(value)
}

export function isDispositionReason(value: string): value is DriverDispositionReasonCode {
  return (DISPOSITION_REASON_CODES as readonly string[]).includes(value)
}

export function dispositionIssue(
  shipmentId: string | null | undefined,
  cargoId: string | null | undefined,
  draft: DispositionDraft,
): DispositionIssue | null {
  if (!usableIdentity(shipmentId) || !usableIdentity(cargoId)) return 'shipment'
  if (!Number.isInteger(draft.accepted) || !Number.isInteger(draft.rejected)) return 'quantity'
  if (draft.accepted < 0 || draft.rejected < 0 || draft.accepted + draft.rejected <= 0) return 'quantity'
  if (draft.rejected > 0 && !isDispositionReason(draft.reasonCode)) return 'reason'
  if (draft.rejected > 0 && draft.reasonCode === 'OTHER' && draft.reasonComment.trim() === '') return 'comment'
  return null
}

export function buildStopCommand(version: number, occurredAt: string): DriverStopCommandRequest {
  return { occurredAt, expectedVersion: version }
}

export function buildFailCommand(
  version: number,
  occurredAt: string,
  reasonCode: DriverFailReasonCode,
): DriverStopFailRequest {
  return { occurredAt, expectedVersion: version, reasonCode }
}

export function buildDispositionBody(
  shipmentId: string,
  cargoId: string,
  draft: DispositionDraft,
  occurredAt: string,
): DriverDeliveryDispositionRequest | null {
  if (dispositionIssue(shipmentId, cargoId, draft)) return null
  const body: DriverDeliveryDispositionRequest = {
    shipmentId,
    cargoId,
    acceptedQuantity: draft.accepted,
    rejectedQuantity: draft.rejected,
    uom: 'PALLET',
    occurredAt,
    evidence: [],
  }
  if (draft.rejected > 0 && isDispositionReason(draft.reasonCode)) {
    body.reasonCode = draft.reasonCode
    const comment = draft.reasonComment.trim()
    if (comment) body.reasonComment = comment
  }
  return body
}

export function dispositionIsRejection(rejectedQuantity: number): boolean {
  return rejectedQuantity > 0
}

export function classifyStopFailure(error: DriverApiError | undefined): StopFailureKind {
  if (!error) return 'server'
  if (error.status === 401) return 'unauthorized'
  if (error.status === 403) return 'forbidden'
  if (error.status === 404) return 'missing'
  const detailsReason = typeof error.details.reason === 'string' ? error.details.reason : ''
  const token = `${error.code} ${error.message} ${detailsReason}`
  if (error.status === 409 && token.includes('ACTION_PENDING')) return 'action_pending'
  if (error.status === 409 && CONFLICT_CODES.some((code) => token.includes(code))) return 'conflict'
  if (error.status === 409) return 'conflict'
  if (error.status === 400 || error.status === 422) return 'validation'
  if (error.status >= 500) return 'server'
  return 'server'
}

export function safeValidationMessage(error: DriverApiError, fallback: string): string {
  const message = error.message.trim()
  if (!message || message.includes('\n') || message.length > 300 || /at\s+\S+\s+\(/.test(message)) {
    return fallback
  }
  return message
}

export function loadStoredCommand<T>(kind: StopOperationKind, targetId: string): StoredCommand<T> | null {
  try {
    const raw = sessionStorage.getItem(storageKey(kind, targetId))
    if (!raw) return null
    const parsed = JSON.parse(raw) as StoredCommand<T>
    if (!parsed?.key || !isValidOperationId(parsed.key) || parsed.body === undefined || parsed.body === null) {
      return null
    }
    return parsed
  } catch {
    return null
  }
}

export function clearStoredCommand(kind: StopOperationKind, targetId: string): void {
  sessionStorage.removeItem(storageKey(kind, targetId))
}

export function beginCommand<T>(kind: StopOperationKind, targetId: string, build: () => T): StoredCommand<T> {
  const existing = loadStoredCommand<T>(kind, targetId)
  if (existing) return existing
  const created = { key: createOperationId(kind, targetId), body: build() }
  sessionStorage.setItem(storageKey(kind, targetId), JSON.stringify(created))
  return created
}
