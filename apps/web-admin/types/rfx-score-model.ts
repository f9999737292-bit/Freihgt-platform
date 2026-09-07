/** RFx v3.0D score model types aligned with backend DTOs (snake_case). */

export const SCORE_MODEL_STATUSES = ['DRAFT', 'PUBLISHED'] as const
export type ScoreModelStatus = (typeof SCORE_MODEL_STATUSES)[number]

export const SCORING_QUESTION_TYPES = ['NUMBER', 'PERCENT', 'YES_NO', 'SINGLE_SELECT', 'MULTI_SELECT'] as const
export type ScoringQuestionType = (typeof SCORING_QUESTION_TYPES)[number]

export const NORMALIZATION_TYPES = ['NUMBER_LINEAR', 'BOOLEAN_MAP', 'OPTION_MAP', 'MULTI_SELECT'] as const
export type NormalizationType = (typeof NORMALIZATION_TYPES)[number]

export const MULTI_SELECT_AGGREGATIONS = ['SUM_CAPPED', 'MAX', 'AVERAGE'] as const
export type MultiSelectAggregation = (typeof MULTI_SELECT_AGGREGATIONS)[number]

export const KNOCKOUT_TYPES = ['BOOLEAN_EQUALS', 'OPTION_EQUALS', 'NUMBER_THRESHOLD'] as const
export type KnockoutType = (typeof KNOCKOUT_TYPES)[number]

export const SCORE_MODEL_EDITOR_STATES = [
  'LOADING',
  'DRAFT_CLEAN',
  'DRAFT_DIRTY',
  'SAVING',
  'SAVE_FAILED',
  'VALIDATING',
  'NOT_READY',
  'READY',
  'PUBLISHING',
  'PUBLISHED',
  'LOAD_FAILED',
] as const
export type ScoreModelEditorState = (typeof SCORE_MODEL_EDITOR_STATES)[number]

export interface ScoreModelMeta {
  id: string
  rfx_version_id: string
  model_version: number
  status: ScoreModelStatus
  model_type: string
  published_at?: string | null
}

export interface ScoreCriterionRecord {
  id?: string
  criterion_code: string
  name: string
  weight: number
  normalization_json: Record<string, unknown> | string
  sort_order?: number
}

export interface ScoreBindingRecord {
  id?: string
  criterion_id?: string
  question_id?: string
  binding_type?: string
  scoring_rule_json?: Record<string, unknown> | string | null
  knockout_rule_json?: Record<string, unknown> | string | null
}

export interface ScoreModelReadinessError {
  code: string
  field?: string
  message: string
  params?: Record<string, unknown>
}

export interface ScoreModelReadinessResult {
  ready: boolean
  errors?: ScoreModelReadinessError[]
}

export interface ScoreModelView {
  model: ScoreModelMeta
  criteria: ScoreCriterionRecord[]
  bindings: ScoreBindingRecord[]
  readiness?: ScoreModelReadinessResult
}

export interface ScoreCriterionInput {
  criterion_code: string
  name: string
  weight: number
  normalization_json: Record<string, unknown>
  sort_order?: number
}

export interface ScoreBindingInput {
  criterion_code: string
  question_code: string
  scoring_rule_json?: Record<string, unknown> | null
  knockout_rule_json?: Record<string, unknown> | null
}

export interface PutScoreModelInput {
  criteria: ScoreCriterionInput[]
  bindings: ScoreBindingInput[]
}

export interface ScoringQuestionOption {
  id: string
  option_code: string
  label: string
}

export interface ScoringBindableQuestion {
  id: string
  question_code: string
  label: string
  question_type: string
  options?: ScoringQuestionOption[]
}
