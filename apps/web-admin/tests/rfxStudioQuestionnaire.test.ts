import { readFileSync } from 'node:fs'
import { resolve } from 'node:path'
import { describe, expect, it } from 'vitest'
import {
  FRONTEND_OPENAPI_EXCLUDED_V3_0B,
  FRONTEND_OPENAPI_PARITY,
  RFX_QUESTIONNAIRE_API_ROUTES,
} from '../utils/rfxQuestionnaireApiRoutes'
import {
  autosaveStatusFromHttpError,
  canShowSaved,
  classifyAutosaveHttpError,
  isPreviewRenderableQuestionType,
  markDirtyTransition,
  markSavedTransition,
  previewShowsRequiredMark,
  readinessItemClass,
  readinessSummaryBadge,
  resolveAutosaveLabel,
  resolveAutosaveLabelKey,
  resolveAutosaveStatusClass,
  resolvePreviewQuestionTypeLabel,
  resolveReadinessStatusLabel,
} from '../utils/rfxStudioQuestionnaire'
import { AUTOSAVE_STATUSES, OPTION_REORDER_UI } from '../types/rfx-questionnaire'

const openapiPath = resolve(import.meta.dirname, '../../../packages/openapi/rfx-service.yaml')
const openapiYaml = readFileSync(openapiPath, 'utf8')
const composableSource = readFileSync(
  resolve(import.meta.dirname, '../composables/useRfxQuestionnaireApi.ts'),
  'utf8',
)

const mockT = (key: string, params?: Record<string, string>) => {
  if (key === 'rfx.studio.status.savedAt' && params?.time) {
    return `Saved at ${params.time}`
  }
  return key
}

describe('resolveAutosaveLabel', () => {
  it('maps every autosave status to a label contract', () => {
    expect(resolveAutosaveLabel({ status: 'dirty', t: mockT })).toBe('rfx.studio.status.dirty')
    expect(resolveAutosaveLabel({ status: 'saving', t: mockT })).toBe('rfx.studio.status.saving')
    expect(resolveAutosaveLabel({ status: 'saved', t: mockT })).toBe('rfx.studio.status.saved')
    expect(resolveAutosaveLabel({ status: 'invalid', t: mockT })).toBe('rfx.studio.status.invalid')
    expect(resolveAutosaveLabel({ status: 'conflict', t: mockT })).toBe('rfx.studio.status.conflict')
    expect(resolveAutosaveLabel({ status: 'save_failed', t: mockT })).toBe('rfx.studio.status.saveFailed')
    expect(resolveAutosaveLabel({ status: 'idle', t: mockT })).toBe('')
  })

  it('prefers fieldError over generic invalid label', () => {
    expect(
      resolveAutosaveLabel({
        status: 'invalid',
        fieldError: 'Question label is required',
        t: mockT,
      }),
    ).toBe('Question label is required')
  })

  it('formats savedAt with injected formatter', () => {
    expect(
      resolveAutosaveLabel({
        status: 'saved',
        lastSavedAt: '2026-09-05T20:15:00.000Z',
        t: mockT,
        formatSavedTime: () => '20:15',
      }),
    ).toBe('Saved at 20:15')
  })
})

describe('autosave state machine helpers', () => {
  it('covers all declared autosave statuses', () => {
    expect(AUTOSAVE_STATUSES).toEqual([
      'idle',
      'dirty',
      'saving',
      'saved',
      'invalid',
      'conflict',
      'save_failed',
    ])
  })

  it('markDirtyTransition preserves saving and clears path from invalid', () => {
    expect(markDirtyTransition('saving')).toBe('saving')
    expect(markDirtyTransition('invalid')).toBe('dirty')
    expect(markDirtyTransition('saved')).toBe('dirty')
  })

  it('canShowSaved blocks saved indicator for invalid and conflict', () => {
    expect(canShowSaved('saved')).toBe(true)
    expect(canShowSaved('dirty')).toBe(true)
    expect(canShowSaved('invalid')).toBe(false)
    expect(canShowSaved('conflict')).toBe(false)
  })

  it('markSavedTransition never shows saved from invalid or conflict', () => {
    expect(markSavedTransition('invalid', '2026-09-05T20:00:00Z')).toEqual({
      status: 'invalid',
      lastSavedAt: '2026-09-05T20:00:00Z',
    })
    expect(markSavedTransition('conflict', '2026-09-05T20:00:00Z')).toEqual({
      status: 'conflict',
      lastSavedAt: '2026-09-05T20:00:00Z',
    })
    expect(markSavedTransition('dirty', '2026-09-05T20:00:00Z')).toEqual({
      status: 'saved',
      lastSavedAt: '2026-09-05T20:00:00Z',
    })
  })

  it('invalid never shows saved — status class and label remain error state', () => {
    expect(resolveAutosaveStatusClass('invalid')).toBe('save-status--error')
    expect(resolveAutosaveLabelKey('invalid')).toBe('rfx.studio.status.invalid')
    expect(canShowSaved('invalid')).toBe(false)
    const afterSaveAttempt = markSavedTransition('invalid')
    expect(afterSaveAttempt.status).not.toBe('saved')
  })

  it('classifies HTTP errors into autosave terminal states', () => {
    expect(classifyAutosaveHttpError(400)).toBe('validation')
    expect(classifyAutosaveHttpError(409)).toBe('conflict')
    expect(classifyAutosaveHttpError(500)).toBe('other')
    expect(autosaveStatusFromHttpError(400)).toBe('invalid')
    expect(autosaveStatusFromHttpError(409)).toBe('conflict')
    expect(autosaveStatusFromHttpError(503)).toBe('save_failed')
  })

  it('resolveAutosaveStatusClass maps visual tones', () => {
    expect(resolveAutosaveStatusClass('saved')).toBe('save-status--ok')
    expect(resolveAutosaveStatusClass('conflict')).toBe('save-status--warn')
    expect(resolveAutosaveStatusClass('save_failed')).toBe('save-status--error')
    expect(resolveAutosaveStatusClass('dirty')).toBe('')
  })
})

describe('FRONTEND_OPENAPI_PARITY matrix', () => {
  it('lists all studio questionnaire routes used by web-admin', () => {
    expect(FRONTEND_OPENAPI_PARITY.length).toBe(19)
    expect(RFX_QUESTIONNAIRE_API_ROUTES.getStudio.path).toContain('/studio')
    expect(RFX_QUESTIONNAIRE_API_ROUTES.validatePublish.method).toBe('POST')
  })

  it('every matrix route exists in rfx-service OpenAPI', () => {
    for (const route of FRONTEND_OPENAPI_PARITY) {
      const openapiPath = route.path.replace('{id}', '{id}')
      const methodLine = `${route.method.toLowerCase()}:`
      expect(openapiYaml).toContain(`  ${openapiPath}:`)
      const pathBlock = openapiYaml.split(`  ${openapiPath}:`)[1]?.split('\n  /')[0] ?? ''
      expect(pathBlock.toLowerCase()).toContain(methodLine)
    }
  })

  it('excludes v3.0B option reorder from parity and composable', () => {
    expect(OPTION_REORDER_UI).toBe('NOT_AVAILABLE_V3_0B')
    for (const excluded of FRONTEND_OPENAPI_EXCLUDED_V3_0B) {
      expect(openapiYaml).not.toContain(excluded.replace('{id}', '{id}'))
    }
    expect(composableSource).not.toMatch(/options\/reorder/)
  })

  it('composable callers reference matrix paths via rfxEventApiPath', () => {
    expect(composableSource).toContain("rfxEventApiPath")
    expect(composableSource).toContain("basePath('/studio')")
    expect(composableSource).toContain("basePath('/save-draft')")
    expect(composableSource).toContain("basePath('/validate-publish')")
    expect(composableSource).toContain("basePath('/sections')")
    expect(composableSource).toContain("basePath('/questions')")
    expect(composableSource).toContain("basePath('/rules')")
  })

  it('RfxStudioHeader delegates autosave display to shared helpers', async () => {
    const headerSource = readFileSync(
      resolve(import.meta.dirname, '../components/rfx/studio/RfxStudioHeader.vue'),
      'utf8',
    )
    expect(headerSource).toContain('resolveAutosaveLabel')
    expect(headerSource).toContain('resolveAutosaveStatusClass')
    expect(headerSource).not.toMatch(/switch \(props\.autosaveStatus\)/)
  })
})

describe('readiness render helpers', () => {
  it('resolveReadinessStatusLabel falls back to raw status when translation missing', () => {
    expect(resolveReadinessStatusLabel('PASS', mockT)).toBe('PASS')
    expect(resolveReadinessStatusLabel('CUSTOM', (key) => `translated:${key}`)).toBe(
      'translated:rfx.studio.readiness.CUSTOM',
    )
  })

  it('readinessItemClass lowercases status for CSS modifier', () => {
    expect(readinessItemClass('FAIL')).toBe('check--fail')
    expect(readinessItemClass('WARN')).toBe('check--warn')
    expect(readinessItemClass('PASS')).toBe('check--pass')
  })

  it('readinessSummaryBadge reflects ready flag', () => {
    expect(
      readinessSummaryBadge({
        ready: true,
        blocking_fail_count: 0,
        warning_count: 0,
        items: [],
      }),
    ).toEqual({ ready: true, status: 'PASS', tone: 'success' })

    expect(
      readinessSummaryBadge({
        ready: false,
        blocking_fail_count: 2,
        warning_count: 1,
        items: [],
      }),
    ).toEqual({ ready: false, status: 'FAIL', tone: 'danger' })
  })
})

describe('preview render helpers', () => {
  it('resolvePreviewQuestionTypeLabel uses i18n key with fallback', () => {
    expect(resolvePreviewQuestionTypeLabel('TEXT', mockT)).toBe('TEXT')
    expect(resolvePreviewQuestionTypeLabel('TABLE', (key) => `label:${key}`)).toBe(
      'label:rfx.studio.questionTypes.TABLE',
    )
  })

  it('isPreviewRenderableQuestionType matches wave-1 controls', () => {
    expect(isPreviewRenderableQuestionType('TEXT')).toBe(true)
    expect(isPreviewRenderableQuestionType('SINGLE_SELECT')).toBe(true)
    expect(isPreviewRenderableQuestionType('MONEY')).toBe(false)
    expect(isPreviewRenderableQuestionType('TABLE')).toBe(false)
  })

  it('previewShowsRequiredMark mirrors required flag', () => {
    expect(previewShowsRequiredMark(true)).toBe(true)
    expect(previewShowsRequiredMark(false)).toBe(false)
  })

  it('RfxCarrierPreview uses preview helpers', async () => {
    const previewSource = readFileSync(
      resolve(import.meta.dirname, '../components/rfx/studio/RfxCarrierPreview.vue'),
      'utf8',
    )
    expect(previewSource).toContain('isPreviewRenderableQuestionType')
    expect(previewSource).toContain('resolvePreviewQuestionTypeLabel')
  })
})
