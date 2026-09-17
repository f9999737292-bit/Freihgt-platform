import type {
  MultiSelectAggregation,
  NormalizationType,
  PutScoreModelInput,
  ScoreBindingInput,
  ScoreCriterionInput,
  ScoreCriterionRecord,
  ScoreModelEditorState,
  ScoreModelReadinessError,
  ScoreModelReadinessResult,
  ScoringBindableQuestion,
  ScoringQuestionType,
} from '~/types/rfx-score-model'

export function parseJsonField<T extends Record<string, unknown>>(value: unknown): T {
  if (typeof value === 'string') {
    try {
      return JSON.parse(value) as T
    } catch {
      return {} as T
    }
  }
  if (value && typeof value === 'object') return value as T
  return {} as T
}

export function defaultNormalizationForQuestionType(type: string): Record<string, unknown> {
  switch (type) {
    case 'YES_NO':
      return { type: 'BOOLEAN_MAP', true_score: 100, false_score: 0 }
    case 'NUMBER':
    case 'PERCENT':
      return { type: 'NUMBER_LINEAR', min: 0, max: 100 }
    case 'SINGLE_SELECT':
      return { type: 'OPTION_MAP', option_scores: {}, default_score: 0 }
    case 'MULTI_SELECT':
      return { type: 'MULTI_SELECT', aggregation: 'SUM_CAPPED', option_scores: {}, cap: 100 }
    default:
      return { type: 'NUMBER_LINEAR', min: 0, max: 100 }
  }
}

export function isScoringCompatibleQuestionType(type: string): type is ScoringQuestionType {
  return ['NUMBER', 'PERCENT', 'YES_NO', 'SINGLE_SELECT', 'MULTI_SELECT'].includes(type)
}

export function filterBindableQuestions(
  questions: ScoringBindableQuestion[],
): ScoringBindableQuestion[] {
  return questions.filter((q) => isScoringCompatibleQuestionType(q.question_type))
}

export function totalWeight(criteria: ScoreCriterionInput[]): number {
  return criteria.reduce((sum, c) => sum + (Number(c.weight) || 0), 0)
}

export function editorStateLabel(state: ScoreModelEditorState, t: (key: string) => string): string {
  const key = `rfx.studio.scoring.states.${state}`
  const translated = t(key)
  return translated === key ? state : translated
}

export function readinessErrorMessage(
  err: ScoreModelReadinessError,
  t: (key: string) => string,
): string {
  const key = `rfx.studio.scoring.readiness.${err.code}`
  const translated = t(key)
  if (translated !== key) return translated
  return err.message || err.code
}

export function deriveEditorState(params: {
  loading: boolean
  loadFailed: boolean
  published: boolean
  saving: boolean
  saveFailed: boolean
  validating: boolean
  publishing: boolean
  dirty: boolean
  readiness: ScoreModelReadinessResult | null
}): ScoreModelEditorState {
  if (params.loading) return 'LOADING'
  if (params.loadFailed) return 'LOAD_FAILED'
  if (params.published) return 'PUBLISHED'
  if (params.publishing) return 'PUBLISHING'
  if (params.validating) return 'VALIDATING'
  if (params.saving) return 'SAVING'
  if (params.saveFailed) return 'SAVE_FAILED'
  if (params.readiness?.ready) return params.dirty ? 'DRAFT_DIRTY' : 'READY'
  if (params.readiness && !params.readiness.ready) return params.dirty ? 'DRAFT_DIRTY' : 'NOT_READY'
  return params.dirty ? 'DRAFT_DIRTY' : 'DRAFT_CLEAN'
}

export function viewToDraftInput(
  criteria: ScoreCriterionRecord[],
  bindings: Array<{ criterion_code: string; question_code: string; knockout_rule_json?: unknown }>,
): PutScoreModelInput {
  return {
    criteria: criteria.map((c, index) =>
      toApiCriterionInput(
        {
          criterion_code: c.criterion_code,
          name: c.name,
          weight: c.weight,
          normalization_json: parseJsonField(c.normalization_json),
          sort_order: c.sort_order ?? index + 1,
          persisted_id: c.id,
          client_render_key: c.id,
        },
        index,
      ),
    ),
    bindings: bindings.map((b) => ({
      criterion_code: b.criterion_code,
      question_code: b.question_code,
      knockout_rule_json: b.knockout_rule_json
        ? parseJsonField(b.knockout_rule_json as Record<string, unknown>)
        : null,
    })),
  }
}

export function createClientRenderKey(): string {
  if (typeof crypto !== 'undefined' && typeof crypto.randomUUID === 'function') {
    return crypto.randomUUID()
  }
  return `client-${Date.now()}-${Math.random().toString(16).slice(2)}`
}

export function criterionRenderKey(criterion: ScoreCriterionInput): string {
  if (criterion.persisted_id) return `persisted:${criterion.persisted_id}`
  if (criterion.client_render_key) return `client:${criterion.client_render_key}`
  return `code:${criterion.criterion_code}`
}

export function toApiCriterionInput(criterion: ScoreCriterionInput, index: number) {
  return {
    criterion_code: criterion.criterion_code,
    name: criterion.name,
    weight: criterion.weight,
    normalization_json: criterion.normalization_json,
    sort_order: criterion.sort_order ?? index + 1,
  }
}

export function mergeCriterionClientKeys(
  previous: ScoreCriterionInput[],
  records: ScoreCriterionRecord[],
): ScoreCriterionInput[] {
  const priorByCode = new Map(previous.map((criterion) => [criterion.criterion_code, criterion]))
  return records.map((record, index) => {
    const prior = priorByCode.get(record.criterion_code)
    return {
      criterion_code: record.criterion_code,
      name: record.name,
      weight: record.weight,
      normalization_json: parseJsonField(record.normalization_json),
      sort_order: record.sort_order ?? index + 1,
      persisted_id: record.id,
      client_render_key: prior?.client_render_key ?? record.id ?? createClientRenderKey(),
    }
  })
}

export function newCriterion(index: number): ScoreCriterionInput {
  return {
    criterion_code: `CRITERION_${index}`,
    name: '',
    weight: 0,
    normalization_json: { type: 'BOOLEAN_MAP', true_score: 100, false_score: 0 },
    sort_order: index,
    client_render_key: createClientRenderKey(),
  }
}

export function normalizationTypeOf(norm: Record<string, unknown>): NormalizationType | null {
  const type = String(norm.type ?? '')
  if (['NUMBER_LINEAR', 'BOOLEAN_MAP', 'OPTION_MAP', 'MULTI_SELECT'].includes(type)) {
    return type as NormalizationType
  }
  return null
}

export function multiSelectAggregation(norm: Record<string, unknown>): MultiSelectAggregation {
  const agg = String(norm.aggregation ?? 'SUM_CAPPED')
  if (agg === 'MAX' || agg === 'AVERAGE') return agg
  return 'SUM_CAPPED'
}

export function bindingForCriterion(
  bindings: ScoreBindingInput[],
  criterionCode: string,
): ScoreBindingInput | undefined {
  return bindings.find((b) => b.criterion_code === criterionCode)
}
