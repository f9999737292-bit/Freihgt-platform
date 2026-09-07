import { describe, expect, it } from 'vitest'
import {
  deriveEditorState,
  filterBindableQuestions,
  isScoringCompatibleQuestionType,
  readinessErrorMessage,
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
    expect(totalWeight([{ criterion_code: 'A', name: 'A', weight: 40, normalization_json: {} }, { criterion_code: 'B', name: 'B', weight: 60, normalization_json: {} }])).toBe(100)
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
    const t = (key: string) => (key.endsWith('CRITERION_CODE_DUPLICATE') ? 'Duplicate criterion code' : key)
    expect(readinessErrorMessage({ code: 'CRITERION_CODE_DUPLICATE', message: 'dup' }, t)).toBe('Duplicate criterion code')
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
