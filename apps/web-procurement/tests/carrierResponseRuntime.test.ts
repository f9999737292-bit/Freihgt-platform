import { describe, expect, it } from 'vitest'
import type { CarrierQuestionnaireDefinition } from '~/types/carrierResponse'
import {
  evaluateCarrierCondition,
  mergeDebouncedAnswerPatches,
  resolveQuestionRequired,
  resolveQuestionVisibility,
  buildRulesByTargetQuestionId,
  buildQuestionMaps,
} from '~/utils/carrierResponseRuntime'

const questionnaire: CarrierQuestionnaireDefinition = {
  event_id: 'e1',
  rfx_version_id: 'v1',
  version_number: 1,
  questionnaire_enabled: true,
  version_status: 'PUBLISHED',
  sections: [
    {
      section: {
        id: 's1',
        section_code: 'HSE',
        title: 'HSE',
        sort_order: 1,
        version: 1,
      },
      questions: [
        {
          id: 'q1',
          section_id: 's1',
          question_code: 'ADR_AVAILABLE',
          question_type: 'YES_NO',
          label: 'ADR?',
          required: false,
          sort_order: 1,
          version: 1,
        },
        {
          id: 'q2',
          section_id: 's1',
          question_code: 'ADR_NUMBER',
          question_type: 'TEXT',
          label: 'ADR number',
          required: false,
          sort_order: 2,
          version: 1,
        },
      ],
    },
  ],
  rules: [
    {
      id: 'r1',
      target_question_id: 'q2',
      rule_code: 'REQ_ADR',
      action: 'REQUIRE',
      condition_json: {
        operator: 'EQUALS',
        source_question_code: 'ADR_AVAILABLE',
        value: true,
      },
      sort_order: 1,
      version: 1,
    },
  ],
}

describe('carrierResponseRuntime', () => {
  it('evaluates EQUALS for yes/no', () => {
    expect(
      evaluateCarrierCondition(
        { operator: 'EQUALS', source_question_code: 'ADR_AVAILABLE', value: true },
        { ADR_AVAILABLE: true },
      ),
    ).toBe(true)
    expect(
      evaluateCarrierCondition(
        { operator: 'EQUALS', source_question_code: 'ADR_AVAILABLE', value: true },
        { ADR_AVAILABLE: false },
      ),
    ).toBe(false)
  })

  it('conditional required when trigger is yes', () => {
    const { questionById } = buildQuestionMaps(questionnaire)
    const adrNumber = questionById.get('q2')!
    const rulesByTarget = buildRulesByTargetQuestionId(questionnaire.rules)
    const ctx = { questionnaire, answersByQuestionCode: { ADR_AVAILABLE: true } }
    expect(resolveQuestionRequired(adrNumber, ctx, rulesByTarget)).toBe(true)
    ctx.answersByQuestionCode.ADR_AVAILABLE = false
    expect(resolveQuestionRequired(adrNumber, ctx, rulesByTarget)).toBe(false)
  })

  it('visibility defaults to visible without hide rules', () => {
    const { questionById } = buildQuestionMaps(questionnaire)
    const q = questionById.get('q1')!
    const rulesByTarget = buildRulesByTargetQuestionId(questionnaire.rules)
    const ctx = { questionnaire, answersByQuestionCode: {} }
    expect(resolveQuestionVisibility(q, ctx, rulesByTarget)).toBe(true)
  })

  it('merges debounced patches without losing earlier edits', () => {
    const merged = mergeDebouncedAnswerPatches([
      { question_id: 'a', section_id: 's', value: 1 },
      { question_id: 'b', section_id: 's', value: 2 },
      { question_id: 'a', section_id: 's', value: 10 },
    ])
    expect(merged).toHaveLength(2)
    expect(merged.find((p) => p.question_id === 'a')?.value).toBe(10)
    expect(merged.find((p) => p.question_id === 'b')?.value).toBe(2)
  })
})
