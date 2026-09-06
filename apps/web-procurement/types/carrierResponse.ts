/** RFx v3.0C carrier questionnaire response — aligned with OpenAPI (snake_case). */

export const CARRIER_RESPONSE_PRODUCT_STATUSES = [
  'NOT_STARTED',
  'IN_PROGRESS',
  'SUBMITTED',
] as const

export type CarrierResponseProductStatus = (typeof CARRIER_RESPONSE_PRODUCT_STATUSES)[number]

export const CARRIER_QUESTION_TYPES = [
  'TEXT',
  'LONG_TEXT',
  'NUMBER',
  'YES_NO',
  'SINGLE_SELECT',
  'MULTI_SELECT',
  'DATE',
  'MONEY',
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

export type CarrierQuestionType = (typeof CARRIER_QUESTION_TYPES)[number]

export const WAVE1_CARRIER_QUESTION_TYPES = [
  'TEXT',
  'LONG_TEXT',
  'NUMBER',
  'YES_NO',
  'SINGLE_SELECT',
  'MULTI_SELECT',
  'DATE',
] as const satisfies readonly CarrierQuestionType[]

export type Wave1CarrierQuestionType = (typeof WAVE1_CARRIER_QUESTION_TYPES)[number]

export function isWave1CarrierQuestionType(type: CarrierQuestionType): type is Wave1CarrierQuestionType {
  return (WAVE1_CARRIER_QUESTION_TYPES as readonly string[]).includes(type)
}

export const CARRIER_AUTOSAVE_STATUSES = [
  'empty',
  'dirty',
  'validating',
  'invalid',
  'valid',
  'saving',
  'saved',
  'save_failed',
  'conflict',
] as const

export type CarrierAutosaveStatus = (typeof CARRIER_AUTOSAVE_STATUSES)[number]

export const CARRIER_RULE_ACTIONS = ['SHOW', 'HIDE', 'REQUIRE'] as const
export type CarrierRuleAction = (typeof CARRIER_RULE_ACTIONS)[number]

export const CARRIER_SECTION_STATES = ['COMPLETE', 'WARNING', 'ERROR', 'INCOMPLETE'] as const
export type CarrierSectionState = (typeof CARRIER_SECTION_STATES)[number]

export interface CarrierConditionalExpression {
  operator: string
  source_question_code?: string
  value?: unknown
  children?: CarrierConditionalExpression[]
}

export interface CarrierQuestionOption {
  id: string
  option_code: string
  label: string
  sort_order: number
  version: number
}

export interface CarrierQuestion {
  id: string
  section_id: string
  question_code: string
  question_type: CarrierQuestionType
  label: string
  help_text?: string | null
  required: boolean
  validation_rule_json?: Record<string, unknown> | null
  sort_order: number
  version: number
  options?: CarrierQuestionOption[]
}

export interface CarrierSection {
  id: string
  section_code: string
  title: string
  description?: string | null
  sort_order: number
  version: number
}

export interface CarrierSectionWithQuestions {
  section: CarrierSection
  questions: CarrierQuestion[]
}

export interface CarrierQuestionRule {
  id: string
  target_question_id?: string | null
  rule_code: string
  action: CarrierRuleAction
  condition_json?: CarrierConditionalExpression | Record<string, unknown> | null
  sort_order: number
  version: number
}

export interface CarrierQuestionnaireDefinition {
  event_id: string
  rfx_version_id: string
  version_number: number
  questionnaire_enabled: boolean
  version_status: 'DRAFT' | 'PUBLISHED' | 'SUPERSEDED' | 'ARCHIVED'
  sections: CarrierSectionWithQuestions[]
  rules: CarrierQuestionRule[]
}

export interface CarrierAnswerRecord {
  id: string
  question_id: string
  value: unknown
  answer_source?: string
  validation_version?: number
  updated_at?: string
  updated_by?: string | null
  version: number
}

export interface CarrierResponseWorkspace {
  id: string
  tenant_id: string
  rfx_event_id: string
  participant_company_id: string
  rfx_version_id?: string | null
  status: string
  product_status: CarrierResponseProductStatus
  save_version: number
  completion_percent: number
  last_saved_at?: string | null
  last_saved_by?: string | null
  submitted_at?: string | null
  created_at?: string
  updated_at?: string
  version: number
  questionnaire: CarrierQuestionnaireDefinition
  answers: CarrierAnswerRecord[]
}

export interface CarrierAnswerPatchItem {
  section_id?: string
  question_id: string
  field?: string
  value: unknown
}

export interface CarrierPatchAnswersRequest {
  save_version: number
  answers: CarrierAnswerPatchItem[]
}

export interface CarrierResponseSaveResult {
  response_id: string
  save_version: number
  last_saved_at: string
  last_saved_by: string
  completion_percent: number
}

export interface CarrierValidationErrorItem {
  section_id?: string
  question_id?: string
  field: string
  rule: string
  message_key: string
  params?: Record<string, unknown>
}

export interface CarrierResponseValidationResult {
  valid: boolean
  blocking_error_count: number
  errors: CarrierValidationErrorItem[]
  completion_percent: number
}

export interface CarrierResponseSubmitResult {
  response_id: string
  status: string
  submitted_at: string
  save_version: number
}

export interface CarrierGlobalErrorItem {
  sectionId: string
  sectionTitle: string
  questionId: string
  questionCode: string
  questionLabel: string
  messageKey: string
  params?: Record<string, unknown>
  rule: string
  localValue?: unknown
}

export interface CarrierSectionSummary {
  sectionId: string
  sectionCode: string
  title: string
  state: CarrierSectionState
  errorCount: number
  warningCount: number
  incompleteCount: number
}
