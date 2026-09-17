import { describe, expect, it } from 'vitest'
import {
  criterionRenderKey,
  deriveEditorState,
  filterBindableQuestions,
  isScoringCompatibleQuestionType,
  mergeCriterionClientKeys,
  newCriterion,
  readinessErrorMessage,
  toApiCriterionInput,
  totalWeight,
} from '../utils/rfxStudioScoring'

describe('rfxStudioScoring helpers', () => {
  it('filters scoring-compatible question types only', () => {
    const questions = filterBindableQuestions([
      { id: '1', question_code: 'A', label: 'A', question_type: 'TEXT' },
      { id: '2', question_code: 'B', label: 'B', question_type: 'YES_NO' },
    ])
    expect(questions).toHaveLength(1)
    expect(questions[0].question_code).toBe('B')
  })

  it('computes total weight', () => {
    expect(
      totalWeight([
        { criterion_code: 'A', name: 'A', weight: 40, normalization_json: {} },
        { criterion_code: 'B', name: 'B', weight: 60, normalization_json: {} },
      ]),
    ).toBe(100)
  })

  it('derives READY when server readiness is true and draft clean', () => {
    expect(
      deriveEditorState({
        loading: false,
        loadFailed: false,
        published: false,
        saving: false,
        saveFailed: false,
        validating: false,
        publishing: false,
        dirty: false,
        readiness: { ready: true },
      }),
    ).toBe('READY')
  })

  it('maps readiness error codes to i18n keys', () => {
    const t = (key: string) =>
      key.endsWith('CRITERION_CODE_DUPLICATE') ? 'Duplicate criterion code' : key
    expect(readinessErrorMessage({ code: 'CRITERION_CODE_DUPLICATE', message: 'dup' }, t)).toBe(
      'Duplicate criterion code',
    )
  })

  it('YES_NO is scoring compatible', () => {
    expect(isScoringCompatibleQuestionType('YES_NO')).toBe(true)
    expect(isScoringCompatibleQuestionType('TEXT')).toBe(false)
  })
})

describe('studioNav scoring step', () => {
  it('includes scoring in allowed steps source contract', () => {
    const { readFileSync } = require('node:fs')
    const { resolve } = require('node:path')
    const source = readFileSync(resolve(__dirname, '../components/rfx/studio/studioNav.ts'), 'utf8')
    expect(source).toContain("'scoring'")
    expect(source).toContain("t('rfx.studio.steps.scoring')")
  })
})

describe('criterion stable render keys', () => {
  it('assigns distinct client keys to new criteria', () => {
    const first = newCriterion(1)
    const second = newCriterion(2)
    expect(first.client_render_key).toBeTruthy()
    expect(second.client_render_key).toBeTruthy()
    expect(first.client_render_key).not.toBe(second.client_render_key)
    expect(criterionRenderKey(first)).not.toBe(criterionRenderKey(second))
  })

  it('keeps key stable when editable fields change', () => {
    const criterion = newCriterion(1)
    const keyBefore = criterionRenderKey(criterion)
    criterion.name = 'Capacity'
    criterion.weight = 60
    criterion.criterion_code = 'CAPACITY'
    expect(criterionRenderKey(criterion)).toBe(keyBefore)
  })

  it('does not reuse deleted criterion key for a new add', () => {
    const first = newCriterion(1)
    const second = newCriterion(2)
    const deletedKey = criterionRenderKey(first)
    const replacement = newCriterion(3)
    expect(criterionRenderKey(replacement)).not.toBe(deletedKey)
    expect(criterionRenderKey(second)).not.toBe(deletedKey)
  })

  it('preserves client render key for matched criterion during server resync', () => {
    const localHse = { ...newCriterion(1), criterion_code: 'HSE', name: 'HSE', weight: 40 }
    const synced = mergeCriterionClientKeys(
      [localHse],
      [
        {
          criterion_code: 'HSE',
          name: 'HSE',
          weight: 40,
          normalization_json: {},
          id: 'server-hse-id',
        },
      ],
    )
    expect(synced).toHaveLength(1)
    expect(synced[0].persisted_id).toBe('server-hse-id')
    expect(synced[0].client_render_key).toBe(localHse.client_render_key)
    expect(criterionRenderKey(synced[0])).toBe(`persisted:server-hse-id`)
  })

  it('strips client keys from API payload', () => {
    const payload = toApiCriterionInput(newCriterion(1), 0)
    expect(payload).not.toHaveProperty('client_render_key')
    expect(payload).not.toHaveProperty('persisted_id')
    expect(payload.criterion_code).toBe('CRITERION_1')
  })
})

describe('RfxScoringWorkspace readiness markers', () => {
  it('exposes distinct workspace shell and model-ready test ids', () => {
    const { readFileSync } = require('node:fs')
    const { resolve } = require('node:path')
    const source = readFileSync(
      resolve(__dirname, '../components/rfx/studio/RfxScoringWorkspace.vue'),
      'utf8',
    )
    expect(source).toContain('data-testid="rfx-scoring-workspace"')
    expect(source).toContain('data-testid="scoring-model-ready"')
    expect(source).toContain('data-testid="scoring-state-load-failed"')
    expect(source).toContain('data-testid="scoring-add-criterion"')
    expect(source).toContain('criterionRenderKey(criterion)')
    expect(source).not.toMatch(/:key="index"/)
  })
})
