import type {
  CarrierConditionalExpression,
  CarrierQuestion,
  CarrierQuestionRule,
  CarrierQuestionnaireDefinition,
  CarrierRuleAction,
} from '~/types/carrierResponse'

export interface CarrierRuleRuntimeContext {
  questionnaire: CarrierQuestionnaireDefinition
  answersByQuestionCode: Record<string, unknown>
}

function isAnswerEmpty(value: unknown): boolean {
  if (value === null || value === undefined) return true
  if (typeof value === 'string') return value.trim() === ''
  if (Array.isArray(value)) return value.length === 0
  return false
}

function compareScalar(left: unknown, right: unknown, operator: string): boolean {
  if (operator === 'EQUALS') {
    if (typeof left === 'boolean' || typeof right === 'boolean') {
      return Boolean(left) === Boolean(right)
    }
    if (typeof left === 'number' && typeof right === 'number') return left === right
    return String(left) === String(right)
  }
  if (operator === 'NOT_EQUALS') {
    return !compareScalar(left, right, 'EQUALS')
  }
  if (operator === 'GREATER_THAN') {
    return Number(left) > Number(right)
  }
  if (operator === 'LESS_THAN') {
    return Number(left) < Number(right)
  }
  if (operator === 'IN') {
    if (!Array.isArray(right)) return false
    return right.some((item) => compareScalar(left, item, 'EQUALS'))
  }
  if (operator === 'NOT_IN') {
    if (!Array.isArray(right)) return true
    return !right.some((item) => compareScalar(left, item, 'EQUALS'))
  }
  return false
}

export function evaluateCarrierCondition(
  expr: CarrierConditionalExpression | null | undefined,
  answersByQuestionCode: Record<string, unknown>,
): boolean {
  if (!expr) return false
  const op = String(expr.operator || '').trim()
  switch (op) {
    case 'AND':
      return (expr.children ?? []).every((child) => evaluateCarrierCondition(child, answersByQuestionCode))
    case 'OR':
      return (expr.children ?? []).some((child) => evaluateCarrierCondition(child, answersByQuestionCode))
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
      return compareScalar(
        answersByQuestionCode[expr.source_question_code ?? ''],
        expr.value,
        op,
      )
    default:
      return false
  }
}

export function buildQuestionMaps(questionnaire: CarrierQuestionnaireDefinition) {
  const questionById = new Map<string, CarrierQuestion>()
  const questionByCode = new Map<string, CarrierQuestion>()
  for (const swq of questionnaire.sections) {
    for (const question of swq.questions) {
      questionById.set(question.id, question)
      questionByCode.set(question.question_code, question)
    }
  }
  return { questionById, questionByCode }
}

export function buildRulesByTargetQuestionId(rules: CarrierQuestionRule[]): Map<string, CarrierQuestionRule[]> {
  const map = new Map<string, CarrierQuestionRule[]>()
  for (const rule of rules) {
    const targetId = rule.target_question_id
    if (!targetId) continue
    const list = map.get(targetId) ?? []
    list.push(rule)
    map.set(targetId, list)
  }
  return map
}

export function resolveQuestionVisibility(
  question: CarrierQuestion,
  ctx: CarrierRuleRuntimeContext,
  rulesByTarget: Map<string, CarrierQuestionRule[]>,
): boolean {
  const rules = rulesByTarget.get(question.id) ?? []
  let visible = true
  for (const rule of rules) {
    const condition = rule.condition_json as CarrierConditionalExpression | undefined
    const matches = evaluateCarrierCondition(condition, ctx.answersByQuestionCode)
    if (rule.action === 'SHOW') {
      visible = matches
    } else if (rule.action === 'HIDE' && matches) {
      visible = false
    }
  }
  return visible
}

export function resolveQuestionRequired(
  question: CarrierQuestion,
  ctx: CarrierRuleRuntimeContext,
  rulesByTarget: Map<string, CarrierQuestionRule[]>,
): boolean {
  if (question.required) return true
  const rules = rulesByTarget.get(question.id) ?? []
  for (const rule of rules) {
    if (rule.action !== 'REQUIRE') continue
    const condition = rule.condition_json as CarrierConditionalExpression | undefined
    if (evaluateCarrierCondition(condition, ctx.answersByQuestionCode)) {
      return true
    }
  }
  return false
}

export function answersByCodeFromLocalValues(
  questionnaire: CarrierQuestionnaireDefinition,
  localValues: Map<string, unknown>,
): Record<string, unknown> {
  const { questionById } = buildQuestionMaps(questionnaire)
  const out: Record<string, unknown> = {}
  for (const [questionId, value] of localValues.entries()) {
    const question = questionById.get(questionId)
    if (question) out[question.question_code] = value
  }
  return out
}

export function isHiddenQuestionForSave(
  question: CarrierQuestion,
  ctx: CarrierRuleRuntimeContext,
  rulesByTarget: Map<string, CarrierQuestionRule[]>,
): boolean {
  return !resolveQuestionVisibility(question, ctx, rulesByTarget)
}

export function mergeDebouncedAnswerPatches<T extends { question_id: string }>(patches: T[]): T[] {
  const merged = new Map<string, T>()
  for (const patch of patches) {
    merged.set(patch.question_id, { ...merged.get(patch.question_id), ...patch })
  }
  return Array.from(merged.values())
}

export function collectRuleActionsForQuestion(
  questionId: string,
  rules: CarrierQuestionRule[],
): CarrierRuleAction[] {
  return rules.filter((rule) => rule.target_question_id === questionId).map((rule) => rule.action)
}
