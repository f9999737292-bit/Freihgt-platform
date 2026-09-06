import type {
  AutosaveStatus,
  RfxPublishReadinessResult,
  RfxQuestionType,
  RfxReadinessStatus,
} from '~/types/rfx-questionnaire'
import { isWave1QuestionType } from '~/types/rfx-questionnaire'

export type AutosaveLabelKey =
  | 'rfx.studio.status.dirty'
  | 'rfx.studio.status.saving'
  | 'rfx.studio.status.saved'
  | 'rfx.studio.status.savedAt'
  | 'rfx.studio.status.invalid'
  | 'rfx.studio.status.conflict'
  | 'rfx.studio.status.saveFailed'
  | ''

export type AutosaveStatusClass = '' | 'save-status--ok' | 'save-status--error' | 'save-status--warn'

export interface ResolveAutosaveLabelInput {
  status: AutosaveStatus
  lastSavedAt?: string | null
  fieldError?: string | null
  t: (key: string, params?: Record<string, string>) => string
  formatSavedTime?: (iso: string) => string
}

/** Pure autosave label resolver — mirrors RfxStudioHeader display contract. */
export function resolveAutosaveLabel(input: ResolveAutosaveLabelInput): string {
  const { status, lastSavedAt, fieldError, t } = input
  const formatTime = input.formatSavedTime ?? ((iso: string) => new Date(iso).toLocaleTimeString())

  switch (status) {
    case 'dirty':
      return t('rfx.studio.status.dirty')
    case 'saving':
      return t('rfx.studio.status.saving')
    case 'saved':
      if (lastSavedAt) {
        return t('rfx.studio.status.savedAt', { time: formatTime(lastSavedAt) })
      }
      return t('rfx.studio.status.saved')
    case 'invalid':
      return fieldError || t('rfx.studio.status.invalid')
    case 'conflict':
      return t('rfx.studio.status.conflict')
    case 'save_failed':
      return t('rfx.studio.status.saveFailed')
    default:
      return ''
  }
}

export function resolveAutosaveLabelKey(status: AutosaveStatus): AutosaveLabelKey {
  switch (status) {
    case 'dirty':
      return 'rfx.studio.status.dirty'
    case 'saving':
      return 'rfx.studio.status.saving'
    case 'saved':
      return 'rfx.studio.status.saved'
    case 'invalid':
      return 'rfx.studio.status.invalid'
    case 'conflict':
      return 'rfx.studio.status.conflict'
    case 'save_failed':
      return 'rfx.studio.status.saveFailed'
    default:
      return ''
  }
}

export function resolveAutosaveStatusClass(status: AutosaveStatus): AutosaveStatusClass {
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

/** Saved indicator must never appear while invalid or in conflict. */
export function canShowSaved(status: AutosaveStatus): boolean {
  return status !== 'invalid' && status !== 'conflict'
}

export function markDirtyTransition(current: AutosaveStatus): AutosaveStatus {
  if (current === 'saving') return current
  return 'dirty'
}

export function markSavedTransition(
  current: AutosaveStatus,
  lastSavedAt?: string | null,
): { status: AutosaveStatus; lastSavedAt: string | null | undefined } {
  if (!canShowSaved(current)) {
    return { status: current, lastSavedAt }
  }
  return { status: 'saved', lastSavedAt: lastSavedAt ?? undefined }
}

export type AutosaveHttpErrorKind = 'validation' | 'conflict' | 'other'

export function classifyAutosaveHttpError(statusCode: number): AutosaveHttpErrorKind {
  if (statusCode === 400) return 'validation'
  if (statusCode === 409) return 'conflict'
  return 'other'
}

export function autosaveStatusFromHttpError(statusCode: number): AutosaveStatus {
  const kind = classifyAutosaveHttpError(statusCode)
  if (kind === 'validation') return 'invalid'
  if (kind === 'conflict') return 'conflict'
  return 'save_failed'
}

export function resolveReadinessStatusLabel(
  status: RfxReadinessStatus | string,
  t: (key: string) => string,
): string {
  const key = `rfx.studio.readiness.${status}`
  const translated = t(key)
  return translated === key ? status : translated
}

export function readinessItemClass(status: RfxReadinessStatus | string): string {
  return `check--${status.toLowerCase()}`
}

export function readinessSummaryBadge(
  result: RfxPublishReadinessResult,
): { status: 'PASS' | 'FAIL'; tone: 'success' | 'danger'; ready: boolean } {
  return {
    ready: result.ready,
    status: result.ready ? 'PASS' : 'FAIL',
    tone: result.ready ? 'success' : 'danger',
  }
}

export function resolvePreviewQuestionTypeLabel(type: string, t: (key: string) => string): string {
  const key = `rfx.studio.questionTypes.${type}`
  const translated = t(key)
  return translated === key ? type : translated
}

/** Wave-1 types render interactive preview controls; others show type label only. */
export function isPreviewRenderableQuestionType(type: RfxQuestionType): boolean {
  return isWave1QuestionType(type)
}

export function previewShowsRequiredMark(required: boolean): boolean {
  return required
}
