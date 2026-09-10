import { ApiError } from '~/composables/useApi'

export type RfxHttpUiState =
  | 'bad_request'
  | 'unauthorized'
  | 'forbidden'
  | 'not_found'
  | 'conflict'
  | 'validation'
  | 'unknown'

export function classifyRfxHttpError(error: unknown): RfxHttpUiState {
  if (!(error instanceof ApiError)) return 'unknown'
  if (error.status === 400) return 'bad_request'
  if (error.status === 401) return 'unauthorized'
  if (error.status === 403) return 'forbidden'
  if (error.status === 404) return 'not_found'
  if (error.status === 409) return 'conflict'
  if (error.status === 422) return 'validation'
  return 'unknown'
}

export function resolveRfxErrorMessageKey(state: RfxHttpUiState): string {
  switch (state) {
    case 'bad_request':
      return 'rfx.errors.badRequest'
    case 'unauthorized':
      return 'rfx.errors.unauthorized'
    case 'forbidden':
      return 'rfx.errors.forbidden'
    case 'not_found':
      return 'rfx.errors.notFound'
    case 'conflict':
      return 'rfx.errors.conflict'
    case 'validation':
      return 'rfx.errors.validation'
    default:
      return 'common.error'
  }
}

export function resolveRfxConflictDetailKey(code?: string): string | null {
  if (!code) return null
  const map: Record<string, string> = {
    VERSION_CONFLICT: 'rfx.errors.versionConflict',
    STATUS_CONFLICT: 'rfx.errors.statusConflict',
    IDEMPOTENCY_CONFLICT: 'rfx.errors.idempotencyConflict',
    DRAFT_ALREADY_EXISTS: 'rfx.errors.draftAlreadyExists',
    STALE_DIFF: 'rfx.errors.staleDiff',
    IMPACT_ANALYSIS_CONSUMED: 'rfx.errors.impactAnalysisConsumed',
    IMPACT_ANALYSIS_EXPIRED: 'rfx.errors.impactAnalysisExpired',
    TEMPLATE_ARCHIVED: 'rfx.errors.templateArchived',
    TEMPLATE_DRAFT_ONLY: 'rfx.errors.templateDraftOnly',
  }
  return map[code] ?? null
}

export function formatRfxApiError(error: unknown, t: (key: string) => string): string {
  if (error instanceof ApiError) {
    const detailKey = resolveRfxConflictDetailKey(error.code)
    if (detailKey) return t(detailKey)
  }
  const state = classifyRfxHttpError(error)
  return t(resolveRfxErrorMessageKey(state))
}
