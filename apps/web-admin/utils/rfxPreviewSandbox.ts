import type {
  RfxConditionalExpression,
  RfxQuestion,
  RfxQuestionRule,
  RfxQuestionType,
  RfxSectionWithQuestions,
} from '~/types/rfx-questionnaire'
import { isWave1QuestionType } from '~/types/rfx-questionnaire'

export interface PreviewSandboxValidationError {
  sectionId: string
  questionId: string
  questionCode: string
  rule: string
  messageKey: string
  params?: Record<string, unknown>
}

export interface PreviewSectionSummary {
  sectionId: string
  sectionCode: string
  title: string
  errorCount: number
  incompleteCount: number
}

function isAnswerEmpty(value: unknown): boolean {
  if (value === null || value === undefined) return true
  if (typeof value === 'string') return value.trim() === ''
  if (Array.isArray(value)) return value.length === 0
  return false
}

function compareScalar(left: unknown, right: unknown, operator: string): boolean {
  if (operator === 'EQUALS') {
    if (typeof left === 'boolean' || typeof right === 'boolean') return Boolean(left) === Boolean(right)
    if (typeof left === 'number' && typeof right === 'number') return left === right
    return String(left) === String(right)
  }
  if (operator === 'NOT_EQUALS') return !compareScalar(left, right, 'EQUALS')
  if (operator === 'GREATER_THAN') return Number(left) > Number(right)
  if (operator === 'LESS_THAN') return Number(left) < Number(right)
  if (operator === 'IN') {
    return Array.isArray(right) && right.some((item) => compareScalar(left, item, 'EQUALS'))
  }
  if (operator === 'NOT_IN') {
    return !Array.isArray(right) || !right.some((item) => compareScalar(left, item, 'EQUALS'))
  }
  return false
}

export function evaluatePreviewCondition(
  expr: RfxConditionalExpression | null | undefined,
  answersByQuestionCode: Record<string, unknown>,
): boolean {
  if (!expr) return false
  const op = String(expr.operator || '').trim()
  switch (op) {
    case 'AND':
      return (expr.children ?? []).every((child) => evaluatePreviewCondition(child, answersByQuestionCode))
    case 'OR':
      return (expr.children ?? []).some((child) => evaluatePreviewCondition(child, answersByQuestionCode))
    case 'IS_EMPTY':
      return isAnswerEmpty(answersByQuestionCode[expr.source_question_code ?? ''])
    case 'IS_NOT_EMPTY':
      return !isAnswerEmpty(answersByQuestionCode[expr.source_question_code ?? ''])
    case 'EQUALS':
    case 'NOT_EQUALS':
    case 'GREATER_THAN':
    case 'LESS_THAN':
    case 'IN':
    case 'NOT_IN':
      return compareScalar(answersByQuestionCode[expr.source_question_code ?? ''], expr.value, op)
    default:
      return false
  }
}

export function buildPreviewQuestionMaps(sections: RfxSectionWithQuestions[]) {
  const questionById = new Map<string, RfxQuestion>()
  const questionByCode = new Map<string, RfxQuestion>()
  for (const swq of sections) {
    for (const question of swq.questions) {
      questionById.set(question.id, question)
      questionByCode.set(question.question_code, question)
    }
  }
  return { questionById, questionByCode }
}

export function buildPreviewRulesByTarget(rules: RfxQuestionRule[]): Map<string, RfxQuestionRule[]> {
  const map = new Map<string, RfxQuestionRule[]>()
  for (const rule of rules) {
    const targetId = rule.target_question_id
    if (!targetId) continue
    const list = map.get(targetId) ?? []
    list.push(rule)
    map.set(targetId, list)
  }
  return map
}

export function answersByCodeFromLocal(
  sections: RfxSectionWithQuestions[],
  localValues: Map<string, unknown>,
): Record<string, unknown> {
  const { questionById } = buildPreviewQuestionMaps(sections)
  const out: Record<string, unknown> = {}
  for (const [questionId, value] of localValues.entries()) {
    const q = questionById.get(questionId)
    if (q) out[q.question_code] = value
  }
  return out
}

export function resolvePreviewQuestionVisibility(
  question: RfxQuestion,
  sections: RfxSectionWithQuestions[],
  rules: RfxQuestionRule[],
  answersByQuestionCode: Record<string, unknown>,
): boolean {
  const rulesByTarget = buildPreviewRulesByTarget(rules)
  const qRules = rulesByTarget.get(question.id) ?? []
  let visible = true
  for (const rule of qRules) {
    const matches = evaluatePreviewCondition(rule.condition_json as RfxConditionalExpression, answersByQuestionCode)
    if (rule.action === 'SHOW') visible = matches
    else if (rule.action === 'HIDE' && matches) visible = false
  }
  return visible
}

export function resolvePreviewQuestionRequired(
  question: RfxQuestion,
  sections: RfxSectionWithQuestions[],
  rules: RfxQuestionRule[],
  answersByQuestionCode: Record<string, unknown>,
): boolean {
  if (question.required) return true
  const rulesByTarget = buildPreviewRulesByTarget(rules)
  for (const rule of rulesByTarget.get(question.id) ?? []) {
    if (rule.action !== 'REQUIRE') continue
    if (evaluatePreviewCondition(rule.condition_json as RfxConditionalExpression, answersByQuestionCode)) {
      return true
    }
  }
  return false
}

function validateFieldValue(question: RfxQuestion, value: unknown): PreviewSandboxValidationError[] {
  if (isAnswerEmpty(value)) return []
  const errors: PreviewSandboxValidationError[] = []
  const base = {
    sectionId: question.section_id,
    questionId: question.id,
    questionCode: question.question_code,
  }

  if (!isWave1QuestionType(question.question_type)) {
    if (question.required) {
      errors.push({ ...base, rule: 'unsupported_type', messageKey: 'unsupportedRequired' })
    }
    return errors
  }

  switch (question.question_type) {
    case 'NUMBER': {
      const num = Number(value)
      if (Number.isNaN(num)) {
        errors.push({ ...base, rule: 'type', messageKey: 'invalid_type' })
        break
      }
      const rules = question.validation_rule_json ?? {}
      const min = rules.min_value as number | undefined
      const max = rules.max_value as number | undefined
      if (min !== undefined && num < min) {
        errors.push({ ...base, rule: 'min', messageKey: 'min_value', params: { min } })
      }
      if (max !== undefined && num > max) {
        errors.push({ ...base, rule: 'max', messageKey: 'max_value', params: { max } })
      }
      break
    }
    case 'SINGLE_SELECT': {
      const code = String(value)
      const ok = (question.options ?? []).some((o) => o.option_code === code)
      if (!ok) errors.push({ ...base, rule: 'enum', messageKey: 'invalid_option' })
      break
    }
    case 'MULTI_SELECT': {
      if (!Array.isArray(value)) {
        errors.push({ ...base, rule: 'type', messageKey: 'invalid_type' })
        break
      }
      for (const item of value) {
        if (!(question.options ?? []).some((o) => o.option_code === item)) {
          errors.push({ ...base, rule: 'enum', messageKey: 'invalid_option', params: { value: item } })
        }
      }
      break
    }
    case 'DATE': {
      const s = String(value).trim()
      if (!/^\d{4}-\d{2}-\d{2}$/.test(s)) {
        errors.push({ ...base, rule: 'date', messageKey: 'date_format' })
      }
      break
    }
    default:
      break
  }
  return errors
}

export function validatePreviewAnswers(input: {
  sections: RfxSectionWithQuestions[]
  rules: RfxQuestionRule[]
  localValues: Map<string, unknown>
}): PreviewSandboxValidationError[] {
  const { sections, rules, localValues } = input
  const answersByQuestionCode = answersByCodeFromLocal(sections, localValues)
  const errors: PreviewSandboxValidationError[] = []

  for (const swq of sections) {
    for (const question of swq.questions) {
      if (!resolvePreviewQuestionVisibility(question, sections, rules, answersByQuestionCode)) continue
      const value = localValues.get(question.id)
      errors.push(...validateFieldValue(question, value))
      const required = resolvePreviewQuestionRequired(question, sections, rules, answersByQuestionCode)
      if (required && isAnswerEmpty(value)) {
        errors.push({
          sectionId: swq.section.id,
          questionId: question.id,
          questionCode: question.question_code,
          rule: 'required',
          messageKey: 'required',
        })
      }
      if (!isWave1QuestionType(question.question_type) && required) {
        errors.push({
          sectionId: swq.section.id,
          questionId: question.id,
          questionCode: question.question_code,
          rule: 'unsupported_type',
          messageKey: 'unsupportedRequired',
        })
      }
    }
  }
  return errors
}

export function computePreviewSectionSummaries(input: {
  sections: RfxSectionWithQuestions[]
  rules: RfxQuestionRule[]
  localValues: Map<string, unknown>
  fieldErrors: PreviewSandboxValidationError[]
}): PreviewSectionSummary[] {
  const answersByQuestionCode = answersByCodeFromLocal(input.sections, input.localValues)
  return input.sections.map((swq) => {
    let errorCount = 0
    let incompleteCount = 0
    for (const question of swq.questions) {
      if (!resolvePreviewQuestionVisibility(question, input.sections, input.rules, answersByQuestionCode)) continue
      const qErrors = input.fieldErrors.filter((e) => e.questionId === question.id)
      if (qErrors.length) {
        errorCount += qErrors.length
        continue
      }
      const required = resolvePreviewQuestionRequired(question, input.sections, input.rules, answersByQuestionCode)
      if (required && isAnswerEmpty(input.localValues.get(question.id))) incompleteCount += 1
    }
    return {
      sectionId: swq.section.id,
      sectionCode: swq.section.section_code,
      title: swq.section.title,
      errorCount,
      incompleteCount,
    }
  })
}

export function computePreviewCompletionPercent(input: {
  sections: RfxSectionWithQuestions[]
  rules: RfxQuestionRule[]
  localValues: Map<string, unknown>
}): number {
  const answersByQuestionCode = answersByCodeFromLocal(input.sections, input.localValues)
  const requiredIds: string[] = []
  for (const swq of input.sections) {
    for (const question of swq.questions) {
      if (!resolvePreviewQuestionVisibility(question, input.sections, input.rules, answersByQuestionCode)) continue
      if (!isWave1QuestionType(question.question_type)) continue
      if (resolvePreviewQuestionRequired(question, input.sections, input.rules, answersByQuestionCode)) {
        requiredIds.push(question.id)
      }
    }
  }
  if (requiredIds.length === 0) return 100
  let answered = 0
  for (const id of requiredIds) {
    if (!isAnswerEmpty(input.localValues.get(id))) answered += 1
  }
  return Math.round((answered / requiredIds.length) * 100)
}

export function isPreviewUnsupportedType(type: RfxQuestionType): boolean {
  return !isWave1QuestionType(type)
}
