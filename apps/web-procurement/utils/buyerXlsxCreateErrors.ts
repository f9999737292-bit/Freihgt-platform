import {
  BUYER_XLSX_CREATE_CONFLICT_CODES,
  BUYER_XLSX_CREATE_MODE,
  type BuyerXlsxCreateConflictCode,
  type BuyerXlsxCreatePreviewResponse,
} from '~/types/buyerXlsxCreate'
import { ApiError } from '~/utils/apiClient'
import type { BuyerXlsxCreateClientErrorKind } from '~/utils/buyerXlsxCreateForm'

export type BuyerXlsxCreateErrorKind =
  | BuyerXlsxCreateClientErrorKind
  | 'preview_invalid'
  | 'analysis_expired'
  | 'analysis_already_consumed'
  | 'idempotency_conflict'
  | 'conflict'
  | 'forbidden'
  | 'not_found'
  | 'payload_too_large'
  | 'validation'
  | 'rate_limited'
  | 'unavailable'

export interface BuyerXlsxCreateClassifiedError {
  kind: BuyerXlsxCreateErrorKind
  status: number
  machineCode: string | null
  retryPreview: boolean
  retryCommit: boolean
  messageKey: string
}

function readMachineCode(details: Record<string, unknown> | undefined): string | null {
  const value = details?.machine_code
  return typeof value === 'string' && value.trim() ? value : null
}

export function isBuyerXlsxCreatePreviewEnvelope(
  value: unknown,
): value is BuyerXlsxCreatePreviewResponse {
  if (!value || typeof value !== 'object') return false
  const body = value as Record<string, unknown>
  const summary = body.normalized_draft_summary
  return (
    body.mode === BUYER_XLSX_CREATE_MODE
    && typeof body.ready_to_commit === 'boolean'
    && typeof body.owner_company_id === 'string'
    && Array.isArray(body.errors)
    && Array.isArray(body.warnings)
    && Boolean(summary)
    && typeof summary === 'object'
  )
}

export function classifyBuyerXlsxCreateClientError(
  kind: BuyerXlsxCreateClientErrorKind,
): BuyerXlsxCreateClassifiedError {
  const messageKey = {
    file_required: 'tenders.buyerXlsxCreate.errors.fileRequired',
    file_extension: 'tenders.buyerXlsxCreate.errors.fileExtension',
    payload_too_large: 'tenders.buyerXlsxCreate.errors.tooLarge',
    metadata_required: 'tenders.buyerXlsxCreate.errors.metadataRequired',
  }[kind]
  return {
    kind,
    status: kind === 'payload_too_large' ? 413 : 400,
    machineCode: null,
    retryPreview: false,
    retryCommit: false,
    messageKey,
  }
}

export function classifyBuyerXlsxCreateHttpError(error: unknown): BuyerXlsxCreateClassifiedError {
  if (!(error instanceof ApiError)) {
    return {
      kind: 'unavailable',
      status: 0,
      machineCode: null,
      retryPreview: false,
      retryCommit: true,
      messageKey: 'tenders.buyerXlsxCreate.errors.unavailable',
    }
  }

  const machineCode = readMachineCode(error.details)
  if (error.status === 422) {
    return {
      kind: 'preview_invalid',
      status: 422,
      machineCode,
      retryPreview: false,
      retryCommit: false,
      messageKey: 'tenders.buyerXlsxCreate.errors.previewInvalid',
    }
  }

  if (error.status === 409) {
    const known = BUYER_XLSX_CREATE_CONFLICT_CODES.includes(machineCode as BuyerXlsxCreateConflictCode)
      ? (machineCode as BuyerXlsxCreateConflictCode)
      : null
    if (known === 'analysis_expired') {
      return {
        kind: 'analysis_expired',
        status: 409,
        machineCode: known,
        retryPreview: true,
        retryCommit: false,
        messageKey: 'tenders.buyerXlsxCreate.errors.analysisExpired',
      }
    }
    if (known === 'analysis_already_consumed') {
      return {
        kind: 'analysis_already_consumed',
        status: 409,
        machineCode: known,
        retryPreview: true,
        retryCommit: false,
        messageKey: 'tenders.buyerXlsxCreate.errors.analysisConsumed',
      }
    }
    if (known === 'idempotency_conflict') {
      return {
        kind: 'idempotency_conflict',
        status: 409,
        machineCode: known,
        retryPreview: false,
        retryCommit: false,
        messageKey: 'tenders.buyerXlsxCreate.errors.idempotencyConflict',
      }
    }
    return {
      kind: 'conflict',
      status: 409,
      machineCode,
      retryPreview: known === 'canonical_hash_mismatch' || known === 'proposal_revalidation_failed',
      retryCommit: false,
      messageKey: known
        ? `tenders.buyerXlsxCreate.errors.${known}`
        : 'tenders.buyerXlsxCreate.errors.conflict',
    }
  }

  if (error.status === 403) {
    return {
      kind: 'forbidden',
      status: 403,
      machineCode,
      retryPreview: false,
      retryCommit: false,
      messageKey: 'tenders.buyerXlsxCreate.errors.forbidden',
    }
  }
  if (error.status === 404) {
    return {
      kind: 'not_found',
      status: 404,
      machineCode,
      retryPreview: false,
      retryCommit: false,
      messageKey: 'tenders.buyerXlsxCreate.errors.notFound',
    }
  }
  if (error.status === 413) {
    return {
      kind: 'payload_too_large',
      status: 413,
      machineCode,
      retryPreview: false,
      retryCommit: false,
      messageKey: 'tenders.buyerXlsxCreate.errors.tooLarge',
    }
  }
  if (error.status === 400) {
    return {
      kind: 'validation',
      status: 400,
      machineCode,
      retryPreview: false,
      retryCommit: false,
      messageKey: 'tenders.buyerXlsxCreate.errors.validation',
    }
  }
  if (error.status === 429) {
    return {
      kind: 'rate_limited',
      status: 429,
      machineCode,
      retryPreview: false,
      retryCommit: true,
      messageKey: 'tenders.buyerXlsxCreate.errors.rateLimited',
    }
  }

  return {
    kind: 'unavailable',
    status: error.status,
    machineCode,
    retryPreview: false,
    retryCommit: error.status === 0 || error.status >= 500,
    messageKey: 'tenders.buyerXlsxCreate.errors.unavailable',
  }
}

export interface BuyerXlsxCreateCommitGuard {
  analysisInvalidated?: boolean
  alreadyCommitted?: boolean
}

export function shouldInvalidateBuyerXlsxCreateAnalysis(kind: BuyerXlsxCreateErrorKind): boolean {
  return kind === 'analysis_expired' || kind === 'analysis_already_consumed'
}

export function canCommitBuyerXlsxCreatePreview(
  preview: BuyerXlsxCreatePreviewResponse | null,
  guard: BuyerXlsxCreateCommitGuard = {},
): boolean {
  if (guard.analysisInvalidated || guard.alreadyCommitted) return false
  return Boolean(preview?.ready_to_commit && preview.analysis_id)
}
