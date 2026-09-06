/** RFx v3.0B questionnaire / studio types aligned with OpenAPI (snake_case). */

export const RFX_QUESTION_TYPES = [
  'TEXT',
  'LONG_TEXT',
  'NUMBER',
  'MONEY',
  'YES_NO',
  'SINGLE_SELECT',
  'MULTI_SELECT',
  'DATE',
  'DATETIME',
  'FILE',
  'TABLE',
  'ADDRESS',
  'COUNTRY',
  'COMPANY',
  'VEHICLE_CATEGORY',
  'CERTIFICATE',
  'PERCENT',
  'RATING',
] as const

export type RfxQuestionType = (typeof RFX_QUESTION_TYPES)[number]

export const WAVE1_QUESTION_TYPES = [
  'TEXT',
  'LONG_TEXT',
  'NUMBER',
  'YES_NO',
  'SINGLE_SELECT',
  'MULTI_SELECT',
  'DATE',
] as const satisfies readonly RfxQuestionType[]

export type Wave1QuestionType = (typeof WAVE1_QUESTION_TYPES)[number]

export const IMPLEMENTED_QUESTION_TYPES = WAVE1_QUESTION_TYPES

export const COMING_NEXT_WAVE_TYPES = RFX_QUESTION_TYPES.filter(
  (t): t is Exclude<RfxQuestionType, Wave1QuestionType> =>
    !(WAVE1_QUESTION_TYPES as readonly string[]).includes(t),
)

export function isWave1QuestionType(type: RfxQuestionType): type is Wave1QuestionType {
  return (WAVE1_QUESTION_TYPES as readonly string[]).includes(type)
}

export const RULE_ACTIONS = ['SHOW', 'HIDE', 'REQUIRE'] as const
export type RfxRuleAction = (typeof RULE_ACTIONS)[number]

export const READINESS_STATUSES = ['PASS', 'FAIL', 'WARN'] as const
export type RfxReadinessStatus = (typeof READINESS_STATUSES)[number]

export const AUTOSAVE_STATUSES = [
  'idle',
  'dirty',
  'saving',
  'saved',
  'invalid',
  'conflict',
  'save_failed',
] as const

export type AutosaveStatus = (typeof AUTOSAVE_STATUSES)[number]

/** v3.0B — option reorder API is not available; UI must not call a reorder endpoint. */
export const OPTION_REORDER_UI = 'NOT_AVAILABLE_V3_0B' as const

export interface RfxConditionalExpression {
  operator: string
  source_question_code?: string
  value?: unknown
  children?: RfxConditionalExpression[]
}

export interface RfxQuestionOption {
  id: string
  tenant_id?: string
  question_id?: string
  option_code: string
  label: string
  sort_order: number
  created_at?: string
  updated_at?: string
  version: number
}

export interface RfxQuestion {
  id: string
  tenant_id?: string
  section_id: string
  question_code: string
  question_type: RfxQuestionType
  label: string
  help_text?: string | null
  required: boolean
  validation_rule_json?: Record<string, unknown> | null
  sort_order: number
  created_at?: string
  updated_at?: string
  version: number
  options?: RfxQuestionOption[]
}

export interface RfxSection {
  id: string
  tenant_id?: string
  rfx_version_id?: string
  section_code: string
  title: string
  description?: string | null
  sort_order: number
  created_at?: string
  updated_at?: string
  version: number
}

export interface RfxQuestionRule {
  id: string
  tenant_id?: string
  rfx_version_id?: string
  target_question_id?: string | null
  rule_code: string
  action: RfxRuleAction
  condition_json?: RfxConditionalExpression | Record<string, unknown> | null
  sort_order: number
  created_at?: string
  updated_at?: string
  version: number
}

export interface RfxSectionWithQuestions {
  section: RfxSection
  questions: RfxQuestion[]
}

export interface RfxStudioEventRecord {
  id: string
  tenant_id?: string
  rfx_number: string
  rfx_type: string
  category: string
  title: string
  description?: string | null
  owner_company_id: string
  status: string
  currency_code?: string | null
  valid_from?: string | null
  valid_to?: string | null
  response_deadline?: string | null
  created_at?: string
  updated_at?: string
  version: number
}

export interface RfxVersionRecord {
  id: string
  tenant_id?: string
  rfx_event_id: string
  version_number: number
  status: 'DRAFT' | 'PUBLISHED' | 'SUPERSEDED' | 'ARCHIVED'
  questionnaire_enabled: boolean
  published_at?: string | null
  published_by?: string | null
  created_at?: string
  updated_at?: string
  version: number
}

export interface RfxStudioResponse {
  event: RfxStudioEventRecord
  draft_version: RfxVersionRecord | null
  sections: RfxSectionWithQuestions[]
  rules: RfxQuestionRule[]
}

export interface RfxQuestionnaireDefinition {
  event_id: string
  rfx_version_id: string
  version_number: number
  questionnaire_enabled: boolean
  version_status: 'DRAFT' | 'PUBLISHED' | 'SUPERSEDED' | 'ARCHIVED'
  sections: RfxSectionWithQuestions[]
  rules: RfxQuestionRule[]
}

export interface RfxPublishReadinessItem {
  code: string
  status: RfxReadinessStatus
  message: string
  details?: Record<string, unknown>
}

export interface RfxPublishReadinessResult {
  ready: boolean
  blocking_fail_count: number
  warning_count: number
  items: RfxPublishReadinessItem[]
}

export interface RfxSaveDraftRequest {
  expected_version?: number
}

export interface RfxVersionedMutationRequest {
  expected_version: number
}

export interface RfxCreateSectionRequest {
  section_code: string
  title: string
  description?: string | null
  sort_order?: number
}

export interface RfxUpdateSectionRequest {
  expected_version: number
  title?: string
  description?: string | null
  sort_order?: number
}

export interface RfxReorderSectionsRequest {
  ordered_ids: string[]
}

export interface RfxCreateQuestionRequest {
  section_id: string
  question_code: string
  question_type: RfxQuestionType
  label: string
  help_text?: string | null
  required?: boolean
  validation_rule_json?: Record<string, unknown>
  sort_order?: number
}

export interface RfxUpdateQuestionRequest {
  expected_version: number
  question_type?: RfxQuestionType
  label?: string
  help_text?: string | null
  required?: boolean
  validation_rule_json?: Record<string, unknown>
  sort_order?: number
}

export interface RfxReorderQuestionsRequest {
  section_id: string
  ordered_ids: string[]
}

export interface RfxCreateOptionRequest {
  option_code: string
  label: string
  sort_order?: number
}

export interface RfxUpdateOptionRequest {
  expected_version: number
  label?: string
  sort_order?: number
}

export interface RfxCreateRuleRequest {
  rule_code: string
  action: RfxRuleAction
  target_question_code?: string
  condition_json?: RfxConditionalExpression | Record<string, unknown>
  sort_order?: number
}

export interface RfxUpdateRuleRequest {
  expected_version: number
  action?: RfxRuleAction
  target_question_code?: string
  condition_json?: RfxConditionalExpression | Record<string, unknown>
  sort_order?: number
}

export function nextSortOrder(items: { sort_order: number }[]): number {
  if (items.length === 0) return 1
  return Math.max(...items.map((i) => i.sort_order)) + 1
}

export function reorderByIds<T extends { id: string }>(items: T[], orderedIds: string[]): T[] {
  const map = new Map(items.map((item) => [item.id, item]))
  return orderedIds.map((id) => map.get(id)).filter((item): item is T => item !== undefined)
}
