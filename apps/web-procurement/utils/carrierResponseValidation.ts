import type {
  CarrierAutosaveStatus,
  CarrierGlobalErrorItem,
  CarrierQuestion,
  CarrierQuestionnaireDefinition,
  CarrierSectionState,
  CarrierSectionSummary,
  CarrierSectionWithQuestions,
  CarrierValidationErrorItem,
} from '~/types/carrierResponse'
import {
  answersByCodeFromLocalValues,
  buildQuestionMaps,
  buildRulesByTargetQuestionId,
  resolveQuestionRequired,
  resolveQuestionVisibility,
  type CarrierRuleRuntimeContext,
} from '~/utils/carrierResponseRuntime'

export function parseCarrierValidation422Body(body: unknown): CarrierValidationErrorItem[] {
  if (!body || typeof body !== 'object') return []
  const record = body as { code?: string; errors?: CarrierValidationErrorItem[] }
  if (record.code !== 'VALIDATION_FAILED' || !Array.isArray(record.errors)) return []
  return record.errors
}

export function groupErrorsByQuestionId(
  errors: CarrierValidationErrorItem[],
): Map<string, CarrierValidationErrorItem[]> {
  const map = new Map<string, CarrierValidationErrorItem[]>()
  for (const item of errors) {
    if (!item.question_id) continue
    const list = map.get(item.question_id) ?? []
    list.push(item)
    map.set(item.question_id, list)
  }
  return map
}

export function localizeValidationMessage(
  item: CarrierValidationErrorItem,
  t: (key: string, params?: Record<string, unknown>) => string,
): string {
  const suffix = item.message_key.replace(/^rfx\.carrier\.validation\./, '')
  const key = `carrierResponse.validation.${suffix}`
  const translated = t(key, item.params ?? {})
  return translated === key ? item.message_key : translated
}

export function validationMessageKey(item: CarrierValidationErrorItem): string {
  return item.message_key.replace(/^rfx\.carrier\.validation\./, '')
}

export function buildGlobalErrorSummary(
  errors: CarrierValidationErrorItem[],
  questionnaire: CarrierQuestionnaireDefinition,
  localValues: Map<string, unknown>,
  t: (key: string, params?: Record<string, unknown>) => string,
): CarrierGlobalErrorItem[] {
  const { questionById } = buildQuestionMaps(questionnaire)
  const sectionById = new Map<string, CarrierSectionWithQuestions['section']>()
  for (const swq of questionnaire.sections) {
    sectionById.set(swq.section.id, swq.section)
  }
  return errors.map((item) => {
    const question = item.question_id ? questionById.get(item.question_id) : undefined
    const section = item.section_id ? sectionById.get(item.section_id) : undefined
    return {
      sectionId: item.section_id ?? question?.section_id ?? '',
      sectionTitle: section?.title ?? '',
      questionId: item.question_id ?? '',
      questionCode: question?.question_code ?? '',
      questionLabel: question?.label ?? '',
      messageKey: item.message_key,
      params: item.params,
      rule: item.rule,
      localValue: item.question_id ? localValues.get(item.question_id) : undefined,
    }
  })
}

function isQuestionAnswered(question: CarrierQuestion, localValues: Map<string, unknown>): boolean {
  const value = localValues.get(question.id)
  if (value === null || value === undefined) return false
  if (typeof value === 'string') return value.trim() !== ''
  if (Array.isArray(value)) return value.length > 0
  return true
}

export function computeSectionSummaries(input: {
  questionnaire: CarrierQuestionnaireDefinition
  localValues: Map<string, unknown>
  fieldErrors: Map<string, CarrierValidationErrorItem[]>
  warningCountBySection?: Map<string, number>
}): CarrierSectionSummary[] {
  const { questionnaire, localValues, fieldErrors } = input
  const answersByQuestionCode = answersByCodeFromLocalValues(questionnaire, localValues)
  const ctx: CarrierRuleRuntimeContext = { questionnaire, answersByQuestionCode }
  const rulesByTarget = buildRulesByTargetQuestionId(questionnaire.rules)
  const warnings = input.warningCountBySection ?? new Map<string, number>()

  return questionnaire.sections.map((swq) => {
    let errorCount = 0
    let incompleteCount = 0
    const warningCount = warnings.get(swq.section.id) ?? 0

    for (const question of swq.questions) {
      const visible = resolveQuestionVisibility(question, ctx, rulesByTarget)
      if (!visible) continue
      const qErrors = fieldErrors.get(question.id)
      if (qErrors?.length) {
        errorCount += qErrors.length
        continue
      }
      const required = resolveQuestionRequired(question, ctx, rulesByTarget)
      if (required && !isQuestionAnswered(question, localValues)) {
        incompleteCount += 1
      }
    }

    let state: CarrierSectionState = 'COMPLETE'
    if (errorCount > 0) state = 'ERROR'
    else if (incompleteCount > 0) state = 'INCOMPLETE'
    else if (warningCount > 0) state = 'WARNING'

    return {
      sectionId: swq.section.id,
      sectionCode: swq.section.section_code,
      title: swq.section.title,
      state,
      errorCount,
      warningCount,
      incompleteCount,
    }
  })
}

export function countBlockingErrors(fieldErrors: Map<string, CarrierValidationErrorItem[]>): number {
  let total = 0
  for (const items of fieldErrors.values()) total += items.length
  return total
}

export function shouldBlockSubmit(
  autosaveStatus: CarrierAutosaveStatus,
  blockingErrorCount: number,
  serverValid: boolean | null,
  isSubmitted: boolean,
): boolean {
  if (isSubmitted) return true
  if (blockingErrorCount > 0) return true
  if (serverValid === false) return true
  if (autosaveStatus === 'dirty' || autosaveStatus === 'invalid' || autosaveStatus === 'conflict') return true
  return false
}

export function formatLocalValueForDisplay(value: unknown): string {
  if (value === null || value === undefined) return '—'
  if (typeof value === 'boolean') return value ? 'true' : 'false'
  if (Array.isArray(value)) return value.join(', ')
  return String(value)
}
