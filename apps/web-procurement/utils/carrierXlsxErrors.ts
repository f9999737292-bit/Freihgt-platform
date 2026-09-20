import type { CarrierXlsxConflictCode, CarrierXlsxPreviewResponse } from '~/types/carrierXlsx'
import { CARRIER_XLSX_CONFLICT_CODES } from '~/types/carrierXlsx'
import { ApiError } from '~/utils/apiClient'

export type CarrierXlsxErrorKind =
  | 'preview_invalid'
  | 'stale_target'
  | 'analysis_expired'
  | 'analysis_already_consumed'
  | 'idempotency_conflict'
  | 'response_not_editable'
  | 'conflict'
  | 'forbidden'
  | 'not_found'
  | 'payload_too_large'
  | 'validation'
  | 'unavailable'

export interface CarrierXlsxClassifiedError {
  kind: CarrierXlsxErrorKind
  status: number
  machineCode: string | null
  retryPreview: boolean
  disableImport: boolean
  messageKey: string
}

function readMachineCode(details: Record<string, unknown> | undefined): string | null {
  const value = details?.machine_code
  return typeof value === 'string' && value.trim() ? value : null
}

export function isCarrierXlsxPreviewEnvelope(value: unknown): value is CarrierXlsxPreviewResponse {
  if (!value || typeof value !== 'object') return false
  const body = value as Record<string, unknown>
  return (
    body.mode === 'UPDATE_CARRIER_DRAFT'
    && typeof body.ready_to_commit === 'boolean'
    && typeof body.target_event_id === 'string'
    && typeof body.target_response_id === 'string'
    && Array.isArray(body.errors)
    && Array.isArray(body.warnings)
  )
}

export function classifyCarrierXlsxHttpError(error: unknown): CarrierXlsxClassifiedError {
  if (!(error instanceof ApiError)) {
    return {
      kind: 'unavailable',
      status: 0,
      machineCode: null,
      retryPreview: false,
      disableImport: false,
      messageKey: 'carrierTenders.xlsx.errors.unavailable',
    }
  }

  const machineCode = readMachineCode(error.details)
  if (error.status === 422) {
    return {
      kind: 'preview_invalid',
      status: 422,
      machineCode,
      retryPreview: false,
      disableImport: machineCode === 'response_not_editable',
      messageKey: 'carrierTenders.xlsx.errors.previewInvalid',
    }
  }

  if (error.status === 409) {
    const known = CARRIER_XLSX_CONFLICT_CODES.includes(machineCode as CarrierXlsxConflictCode)
      ? (machineCode as CarrierXlsxConflictCode)
      : null
    if (known === 'stale_target') {
      return {
        kind: 'stale_target',
        status: 409,
        machineCode: known,
        retryPreview: true,
        disableImport: false,
        messageKey: 'carrierTenders.xlsx.errors.staleTarget',
      }
    }
    if (known === 'analysis_expired') {
      return {
        kind: 'analysis_expired',
        status: 409,
        machineCode: known,
        retryPreview: true,
        disableImport: false,
        messageKey: 'carrierTenders.xlsx.errors.analysisExpired',
      }
    }
    if (known === 'analysis_already_consumed') {
      return {
        kind: 'analysis_already_consumed',
        status: 409,
        machineCode: known,
        retryPreview: true,
        disableImport: false,
        messageKey: 'carrierTenders.xlsx.errors.analysisConsumed',
      }
    }
    if (known === 'idempotency_conflict') {
      return {
        kind: 'idempotency_conflict',
        status: 409,
        machineCode: known,
        retryPreview: false,
        disableImport: false,
        messageKey: 'carrierTenders.xlsx.errors.idempotencyConflict',
      }
    }
    if (known === 'response_not_editable') {
      return {
        kind: 'response_not_editable',
        status: 409,
        machineCode: known,
        retryPreview: false,
        disableImport: true,
        messageKey: 'carrierTenders.xlsx.errors.responseNotEditable',
      }
    }
    return {
      kind: 'conflict',
      status: 409,
      machineCode,
      retryPreview: known === 'canonical_hash_mismatch' || known === 'proposal_revalidation_failed',
      disableImport: false,
      messageKey: known
        ? `carrierTenders.xlsx.errors.${known}`
        : 'carrierTenders.xlsx.errors.conflict',
    }
  }

  if (error.status === 403) {
    return {
      kind: 'forbidden',
      status: 403,
      machineCode,
      retryPreview: false,
      disableImport: false,
      messageKey: 'carrierTenders.xlsx.errors.forbidden',
    }
  }
  if (error.status === 404) {
    return {
      kind: 'not_found',
      status: 404,
      machineCode,
      retryPreview: false,
      disableImport: false,
      messageKey: 'carrierTenders.xlsx.errors.notFound',
    }
  }
  if (error.status === 413) {
    return {
      kind: 'payload_too_large',
      status: 413,
      machineCode,
      retryPreview: false,
      disableImport: false,
      messageKey: 'carrierTenders.xlsx.errors.tooLarge',
    }
  }
  if (error.status === 400) {
    return {
      kind: 'validation',
      status: 400,
      machineCode,
      retryPreview: false,
      disableImport: false,
      messageKey: 'carrierTenders.xlsx.errors.validation',
    }
  }

  return {
    kind: 'unavailable',
    status: error.status,
    machineCode,
    retryPreview: false,
    disableImport: false,
    messageKey: 'carrierTenders.xlsx.errors.unavailable',
  }
}

export interface CarrierXlsxCommitGuard {
  analysisInvalidated?: boolean
  alreadyCommitted?: boolean
  importDisabled?: boolean
}

export function shouldInvalidateCarrierXlsxAnalysis(kind: CarrierXlsxErrorKind): boolean {
  return kind === 'stale_target' || kind === 'analysis_expired' || kind === 'analysis_already_consumed'
}

export function canCommitCarrierXlsxPreview(
  preview: CarrierXlsxPreviewResponse | null,
  guard: CarrierXlsxCommitGuard = {},
): boolean {
  if (guard.analysisInvalidated || guard.alreadyCommitted || guard.importDisabled) return false
  return Boolean(preview?.ready_to_commit && preview.analysis_id)
}
