import { readFileSync } from 'node:fs'
import { resolve } from 'node:path'
import { describe, expect, it } from 'vitest'
import type { RfxQuestion, RfxQuestionRule, RfxSectionWithQuestions } from '../types/rfx-questionnaire'
import {
  computePreviewCompletionPercent,
  computePreviewSectionSummaries,
  evaluatePreviewCondition,
  resolvePreviewQuestionRequired,
  resolvePreviewQuestionVisibility,
  validatePreviewAnswers,
} from '../utils/rfxPreviewSandbox'

const sectionId = 'sec-hse'
const qAdrAvailable = 'q-adr-available'
const qAdrNumber = 'q-adr-number'
const qAdrExpiry = 'q-adr-expiry'
const qFleetCount = 'q-fleet-count'
const qUnsupported = 'q-money'

function question(partial: Partial<RfxQuestion> & Pick<RfxQuestion, 'id' | 'question_code' | 'question_type' | 'label'>): RfxQuestion {
  return {
    section_id: sectionId,
    required: false,
    sort_order: 0,
    ...partial,
  } as RfxQuestion
}

const sections: RfxSectionWithQuestions[] = [
  {
    section: { id: sectionId, section_code: 'HSE', title: 'HSE', sort_order: 0 },
    questions: [
      question({ id: qAdrAvailable, question_code: 'ADR_AVAILABLE', question_type: 'YES_NO', label: 'ADR available?' }),
      question({ id: qAdrNumber, question_code: 'ADR_NUMBER', question_type: 'TEXT', label: 'ADR number' }),
      question({ id: qAdrExpiry, question_code: 'ADR_EXPIRY', question_type: 'DATE', label: 'ADR expiry' }),
      question({
        id: qFleetCount,
        question_code: 'FLEET_COUNT',
        question_type: 'NUMBER',
        label: 'Fleet count',
        validation_rule_json: { min_value: 0 },
      }),
      question({ id: qUnsupported, question_code: 'PRICE', question_type: 'MONEY', label: 'Price', required: true }),
    ],
  },
]

const rules: RfxQuestionRule[] = [
  {
    id: 'rule-1',
    target_question_id: qAdrNumber,
    action: 'REQUIRE',
    condition_json: { operator: 'EQUALS', source_question_code: 'ADR_AVAILABLE', value: true },
  },
  {
    id: 'rule-2',
    target_question_id: qAdrExpiry,
    action: 'REQUIRE',
    condition_json: { operator: 'EQUALS', source_question_code: 'ADR_AVAILABLE', value: true },
  },
]

function local(values: Record<string, unknown>) {
  return new Map(Object.entries(values))
}

describe('rfxPreviewSandbox rule engine', () => {
  it('evaluates EQUALS for YES_NO answers', () => {
    expect(
      evaluatePreviewCondition(
        { operator: 'EQUALS', source_question_code: 'ADR_AVAILABLE', value: true },
        { ADR_AVAILABLE: true },
      ),
    ).toBe(true)
    expect(
      evaluatePreviewCondition(
        { operator: 'EQUALS', source_question_code: 'ADR_AVAILABLE', value: true },
        { ADR_AVAILABLE: false },
      ),
    ).toBe(false)
  })

  it('applies conditional requiredness when ADR_AVAILABLE is YES', () => {
    const answers = { ADR_AVAILABLE: true }
    const adrNumber = sections[0].questions[1]
    expect(resolvePreviewQuestionRequired(adrNumber, sections, rules, answers)).toBe(true)
  })

  it('clears conditional requiredness when condition is false', () => {
    const answers = { ADR_AVAILABLE: false }
    const adrNumber = sections[0].questions[1]
    expect(resolvePreviewQuestionRequired(adrNumber, sections, rules, answers)).toBe(false)
    const errors = validatePreviewAnswers({
      sections,
      rules,
      localValues: local({ [qAdrAvailable]: false, [qFleetCount]: 10 }),
    })
    expect(errors.some((e) => e.questionCode === 'ADR_NUMBER')).toBe(false)
    expect(errors.some((e) => e.questionCode === 'ADR_EXPIRY')).toBe(false)
  })

  it('validates number min/max locally', () => {
    const errors = validatePreviewAnswers({
      sections,
      rules,
      localValues: local({ [qAdrAvailable]: false, [qFleetCount]: -1 }),
    })
    expect(errors.some((e) => e.questionCode === 'FLEET_COUNT' && e.messageKey === 'min_value')).toBe(true)
  })

  it('blocks simulated submit when conditionally required fields are empty', () => {
    const errors = validatePreviewAnswers({
      sections,
      rules,
      localValues: local({ [qAdrAvailable]: true, [qFleetCount]: 10 }),
    })
    const requiredCodes = errors
      .filter((e) => e.messageKey === 'required' && e.questionCode.startsWith('ADR'))
      .map((e) => e.questionCode)
      .sort()
    expect(requiredCodes).toEqual(['ADR_EXPIRY', 'ADR_NUMBER'])
  })

  it('passes validation when all visible required fields are filled', () => {
    const errors = validatePreviewAnswers({
      sections,
      rules,
      localValues: local({
        [qAdrAvailable]: true,
        [qAdrNumber]: 'ABC-123',
        [qAdrExpiry]: '2026-12-31',
        [qFleetCount]: 10,
      }),
    })
    const blocking = errors.filter((e) => e.questionCode !== 'PRICE')
    expect(blocking).toEqual([])
  })

  it('flags unsupported required types explicitly', () => {
    const errors = validatePreviewAnswers({ sections, rules, localValues: local({}) })
    expect(errors.some((e) => e.questionCode === 'PRICE' && e.messageKey === 'unsupportedRequired')).toBe(true)
  })

  it('computes section summaries and simulated completion percent', () => {
    const values = local({ [qAdrAvailable]: true, [qAdrNumber]: 'x', [qFleetCount]: 5 })
    const errors = validatePreviewAnswers({ sections, rules, localValues: values })
    const summaries = computePreviewSectionSummaries({ sections, rules, localValues: values, fieldErrors: errors })
    expect(summaries[0].errorCount).toBeGreaterThan(0)
    const percent = computePreviewCompletionPercent({ sections, rules, localValues: values })
    expect(percent).toBeGreaterThan(0)
    expect(percent).toBeLessThan(100)
  })

  it('renders all seven wave-1 question types without silent TEXT fallback', () => {
    const types = ['TEXT', 'LONG_TEXT', 'NUMBER', 'YES_NO', 'SINGLE_SELECT', 'MULTI_SELECT', 'DATE'] as const
    for (const type of types) {
      const q = question({
        id: `q-${type}`,
        question_code: type,
        question_type: type,
        label: type,
        options: type.includes('SELECT')
          ? [{ id: 'o1', option_code: 'A', label: 'A', sort_order: 0 }]
          : undefined,
      })
      expect(resolvePreviewQuestionVisibility(q, sections, [], {})).toBe(true)
    }
  })
})

describe('preview sandbox API isolation (source scan)', () => {
  const root = resolve(import.meta.dirname, '..')
  const scannedFiles = [
    'components/rfx/studio/RfxCarrierPreviewSandbox.vue',
    'pages/rfx/[id]/studio/preview.vue',
    'utils/rfxPreviewSandbox.ts',
  ]

  for (const rel of scannedFiles) {
    it(`${rel} must not reference carrier-response write routes`, () => {
      const source = readFileSync(resolve(root, rel), 'utf8')
      expect(source).not.toMatch(/\/carrier-response\/start/)
      expect(source).not.toMatch(/\/carrier-response\/answers/)
      expect(source).not.toMatch(/\/carrier-response\/validate/)
      expect(source).not.toMatch(/\/carrier-response\/submit/)
    })
  }

  it('web-admin composables must not define carrier-response client', () => {
    const composablePath = resolve(root, 'composables/useRfxQuestionnaireApi.ts')
    const source = readFileSync(composablePath, 'utf8')
    expect(source).not.toMatch(/carrier-response/)
  })
})
