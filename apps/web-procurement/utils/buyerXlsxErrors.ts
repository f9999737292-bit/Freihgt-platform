import type { BuyerXlsxConflictCode, BuyerXlsxPreviewResponse } from '~/types/buyerXlsx'
import { BUYER_XLSX_CONFLICT_CODES } from '~/types/buyerXlsx'
import { ApiError } from '~/utils/apiClient'

export type BuyerXlsxErrorKind =
  | 'preview_invalid'
  | 'stale_target'
  | 'analysis_expired'
  | 'analysis_already_consumed'
  | 'idempotency_conflict'
  | 'conflict'
  | 'forbidden'
  | 'not_found'
  | 'payload_too_large'
  | 'validation'
  | 'unavailable'

export interface BuyerXlsxClassifiedError {
  kind: BuyerXlsxErrorKind
  status: number
  machineCode: string | null
  retryPreview: boolean
  messageKey: string
}

function readMachineCode(details: Record<string, unknown> | undefined): string | null {
  const value = details?.machine_code
  return typeof value === 'string' && value.trim() ? value : null
}

export function isBuyerXlsxPreviewEnvelope(value: unknown): value is BuyerXlsxPreviewResponse {
  if (!value || typeof value !== 'object') return false
  const body = value as Record<string, unknown>
  return (
    body.mode === 'UPDATE_DRAFT'
    && typeof body.ready_to_commit === 'boolean'
    && typeof body.target_event_id === 'string'
    && Array.isArray(body.errors)
    && Array.isArray(body.warnings)
  )
}

export function classifyBuyerXlsxHttpError(error: unknown): BuyerXlsxClassifiedError {
  if (!(error instanceof ApiError)) {
    return {
      kind: 'unavailable',
      status: 0,
      machineCode: null,
      retryPreview: false,
      messageKey: 'tenders.buyerXlsx.errors.unavailable',
    }
  }

  const machineCode = readMachineCode(error.details)
  if (error.status === 422) {
    return {
      kind: 'preview_invalid',
      status: 422,
      machineCode,
      retryPreview: false,
      messageKey: 'tenders.buyerXlsx.errors.previewInvalid',
    }
  }

  if (error.status === 409) {
    const known = BUYER_XLSX_CONFLICT_CODES.includes(machineCode as BuyerXlsxConflictCode)
      ? (machineCode as BuyerXlsxConflictCode)
      : null
    if (known === 'stale_target') {
      return {
        kind: 'stale_target',
        status: 409,
        machineCode: known,
        retryPreview: true,
        messageKey: 'tenders.buyerXlsx.errors.staleTarget',
      }
    }
    if (known === 'analysis_expired') {
      return {
        kind: 'analysis_expired',
        status: 409,
        machineCode: known,
        retryPreview: true,
        messageKey: 'tenders.buyerXlsx.errors.analysisExpired',
      }
    }
    if (known === 'analysis_already_consumed') {
      return {
        kind: 'analysis_already_consumed',
        status: 409,
        machineCode: known,
        retryPreview: true,
        messageKey: 'tenders.buyerXlsx.errors.analysisConsumed',
      }
    }
    if (known === 'idempotency_conflict') {
      return {
        kind: 'idempotency_conflict',
        status: 409,
        machineCode: known,
        retryPreview: false,
        messageKey: 'tenders.buyerXlsx.errors.idempotencyConflict',
      }
    }
    return {
      kind: 'conflict',
      status: 409,
      machineCode,
      retryPreview: known === 'canonical_hash_mismatch' || known === 'proposal_revalidation_failed',
      messageKey: known
        ? `tenders.buyerXlsx.errors.${known}`
        : 'tenders.buyerXlsx.errors.conflict',
    }
  }

  if (error.status === 403) {
    return {
      kind: 'forbidden',
      status: 403,
      machineCode,
      retryPreview: false,
      messageKey: 'tenders.buyerXlsx.errors.forbidden',
    }
  }
  if (error.status === 404) {
    return {
      kind: 'not_found',
      status: 404,
      machineCode,
      retryPreview: false,
      messageKey: 'tenders.buyerXlsx.errors.notFound',
    }
  }
  if (error.status === 413) {
    return {
      kind: 'payload_too_large',
      status: 413,
      machineCode,
      retryPreview: false,
      messageKey: 'tenders.buyerXlsx.errors.tooLarge',
    }
  }
  if (error.status === 400) {
    return {
      kind: 'validation',
      status: 400,
      machineCode,
      retryPreview: false,
      messageKey: 'tenders.buyerXlsx.errors.validation',
    }
  }

  return {
    kind: 'unavailable',
    status: error.status,
    machineCode,
    retryPreview: false,
    messageKey: 'tenders.buyerXlsx.errors.unavailable',
  }
}

export function canCommitBuyerXlsxPreview(preview: BuyerXlsxPreviewResponse | null): boolean {
  return Boolean(preview?.ready_to_commit && preview.analysis_id)
}
