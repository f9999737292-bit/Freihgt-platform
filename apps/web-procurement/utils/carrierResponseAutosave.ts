import type { CarrierAutosaveStatus } from '~/types/carrierResponse'

export type CarrierAutosaveLabelKey =
  | 'carrierResponse.autosave.empty'
  | 'carrierResponse.autosave.dirty'
  | 'carrierResponse.autosave.validating'
  | 'carrierResponse.autosave.valid'
  | 'carrierResponse.autosave.saving'
  | 'carrierResponse.autosave.saved'
  | 'carrierResponse.autosave.savedAt'
  | 'carrierResponse.autosave.invalid'
  | 'carrierResponse.autosave.conflict'
  | 'carrierResponse.autosave.saveFailed'
  | ''

export type CarrierAutosaveStatusClass = '' | 'save-status--ok' | 'save-status--error' | 'save-status--warn'

export function canShowCarrierSaved(status: CarrierAutosaveStatus): boolean {
  return status !== 'invalid' && status !== 'conflict' && status !== 'save_failed'
}

export function markCarrierDirtyTransition(current: CarrierAutosaveStatus): CarrierAutosaveStatus {
  if (current === 'saving') return current
  return 'dirty'
}

export function markCarrierSavedTransition(
  current: CarrierAutosaveStatus,
  lastSavedAt?: string | null,
): { status: CarrierAutosaveStatus; lastSavedAt: string | null | undefined } {
  if (!canShowCarrierSaved(current)) {
    return { status: current, lastSavedAt }
  }
  return { status: 'saved', lastSavedAt: lastSavedAt ?? undefined }
}

export type CarrierAutosaveHttpErrorKind = 'validation' | 'conflict' | 'auth' | 'forbidden' | 'not_found' | 'other'

export function classifyCarrierAutosaveHttpError(statusCode: number): CarrierAutosaveHttpErrorKind {
  if (statusCode === 422) return 'validation'
  if (statusCode === 409) return 'conflict'
  if (statusCode === 401) return 'auth'
  if (statusCode === 403) return 'forbidden'
  if (statusCode === 404) return 'not_found'
  return 'other'
}

export function autosaveStatusFromCarrierHttpError(statusCode: number): CarrierAutosaveStatus {
  const kind = classifyCarrierAutosaveHttpError(statusCode)
  if (kind === 'validation') return 'invalid'
  if (kind === 'conflict') return 'conflict'
  return 'save_failed'
}

export function resolveCarrierAutosaveLabelKey(status: CarrierAutosaveStatus): CarrierAutosaveLabelKey {
  switch (status) {
    case 'empty':
      return 'carrierResponse.autosave.empty'
    case 'dirty':
      return 'carrierResponse.autosave.dirty'
    case 'validating':
      return 'carrierResponse.autosave.validating'
    case 'valid':
      return 'carrierResponse.autosave.valid'
    case 'saving':
      return 'carrierResponse.autosave.saving'
    case 'saved':
      return 'carrierResponse.autosave.saved'
    case 'invalid':
      return 'carrierResponse.autosave.invalid'
    case 'conflict':
      return 'carrierResponse.autosave.conflict'
    case 'save_failed':
      return 'carrierResponse.autosave.saveFailed'
    default:
      return ''
  }
}

export function resolveCarrierAutosaveStatusClass(status: CarrierAutosaveStatus): CarrierAutosaveStatusClass {
  switch (status) {
    case 'invalid':
    case 'save_failed':
      return 'save-status--error'
    case 'conflict':
      return 'save-status--warn'
    case 'saved':
      return 'save-status--ok'
    default:
      return ''
  }
}

export function isCarrierLeaveWarningState(status: CarrierAutosaveStatus): boolean {
  return status === 'dirty' || status === 'invalid' || status === 'saving' || status === 'save_failed' || status === 'conflict'
}

export function isCarrierUnsavedState(status: CarrierAutosaveStatus): boolean {
  return status !== 'saved' && status !== 'empty'
}

/** DIRTY/INVALID/CONFLICT/SAVE_FAILED must never display as SAVED. */
export function assertSavedOnlyAfterServerAck(
  displayedStatus: CarrierAutosaveStatus,
  serverAcknowledged: boolean,
): boolean {
  if (displayedStatus === 'saved') return serverAcknowledged
  return true
}
