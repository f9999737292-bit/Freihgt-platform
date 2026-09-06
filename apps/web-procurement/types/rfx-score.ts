/** RFx v3.0D response score types (buyer evaluation). */

export const QUALIFICATION_STATUSES = [
  'QUALIFIED',
  'CONDITIONALLY_QUALIFIED',
  'REJECTED',
  'PENDING_REVIEW',
] as const
export type QualificationStatus = (typeof QUALIFICATION_STATUSES)[number]

export const CALCULATION_STATUSES = ['PENDING', 'CALCULATED', 'FAILED'] as const
export type CalculationStatus = (typeof CALCULATION_STATUSES)[number]

export type V3ScoreLoadState = 'LOADING' | 'AVAILABLE' | 'PENDING' | 'FAILED' | 'NOT_AVAILABLE'

export interface V3QualificationView {
  status: QualificationStatus | string
  calculation_status: CalculationStatus | string
  total_score?: number | null
  knockout_triggered?: boolean
  score_model_version?: number
  knockout_reason_json?: unknown
}

export interface V3AnswerScoreView {
  id: string
  answer_id: string
  criterion_id: string
  score_model_version: number
  raw_score?: number | null
  normalized_score?: number | null
  weighted_contribution?: number | null
  explanation_json?: unknown
}

export interface V3ResponseScoreView {
  qualification: V3QualificationView
  answer_scores: V3AnswerScoreView[]
}

export interface V3ScoreExplanation {
  source?: string
  input?: unknown
  rule?: string
  rule_version?: number
  score_model_id?: string
  score_model_version?: number
  criterion_code?: string
  criterion_weight?: number
  raw_score?: number | null
  normalized_score?: number | null
  weighted_contribution?: number | null
  knockout?: boolean
  knockout_reason?: string
}

export interface V3ScoreExplanationResponse {
  explanations: V3ScoreExplanation[]
}

export interface V3ScoreCacheEntry {
  state: V3ScoreLoadState
  score?: V3ResponseScoreView | null
  error?: string | null
}
