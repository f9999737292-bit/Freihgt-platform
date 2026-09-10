/** RFx v3.0E1–E3 version lifecycle, compare, change impact types. */

import type { RfxPublishReadinessResult, RfxQuestionnaireDefinition } from '~/types/rfx-questionnaire'

export const RFX_VERSION_STATUSES = ['DRAFT', 'PUBLISHED', 'SUPERSEDED', 'ARCHIVED'] as const
export type RfxVersionStatus = (typeof RFX_VERSION_STATUSES)[number]

export const RFX_COMPARE_CHANGE_TYPES = [
  'ADDED',
  'REMOVED',
  'CHANGED',
  'REORDERED',
  'UNCHANGED',
] as const
export type RfxCompareChangeType = (typeof RFX_COMPARE_CHANGE_TYPES)[number]

export const RFX_IMPACT_CLASSES = [
  'NON_MATERIAL',
  'MATERIAL_NO_RESPONSES',
  'MATERIAL_WITH_DRAFT_RESPONSES',
  'MATERIAL_WITH_SUBMITTED_RESPONSES',
  'SCORING_AFFECTING',
  'KNOCKOUT_AFFECTING',
] as const
export type RfxImpactClass = (typeof RFX_IMPACT_CLASSES)[number]

export interface RfxVersionRecord {
  id: string
  tenant_id: string
  rfx_event_id: string
  version_number: number
  status: RfxVersionStatus
  questionnaire_enabled: boolean
  is_current_published: boolean
  is_active_draft: boolean
  change_summary?: string | null
  published_at?: string | null
  published_by?: string | null
  superseded_at?: string | null
  superseded_by_version_id?: string | null
  rescoring_required: boolean
  created_at: string
  updated_at: string
  version: number
}

export interface RfxVersionListResponse {
  versions: RfxVersionRecord[]
}

export interface RfxVersionDetailResponse {
  version: RfxVersionRecord
  questionnaire: RfxQuestionnaireDefinition
}

export interface RfxPublishQuestionnaireRequest {
  expected_event_version: number
  expected_draft_version: number
  change_summary: string
  impact_analysis_id?: string
  canonical_diff_hash?: string
}

export interface RfxCompareVersionsRequest {
  source_version_id: string
  target_version_id: string
}

export interface RfxCompareFieldDiff {
  field: string
  before: unknown
  after: unknown
}

export interface RfxCompareItemDiff {
  entity_type: string
  change: RfxCompareChangeType
  section_code?: string
  question_code?: string
  option_code?: string
  rule_code?: string
  criterion_code?: string
  fields?: string[]
  field_diffs?: RfxCompareFieldDiff[]
}

export interface RfxCompareSummary {
  added_count: number
  removed_count: number
  changed_count: number
  reordered_count: number
  unchanged_count: number
}

export interface RfxCompareScoringDiff {
  criteria?: RfxCompareItemDiff[]
  bindings?: RfxCompareItemDiff[]
  model?: RfxCompareItemDiff
}

export interface RfxCompareVersionsResponse {
  source_version: { id: string; version_number: number; status: string }
  target_version: { id: string; version_number: number; status: string }
  source_version_number: number
  target_version_number: number
  summary: RfxCompareSummary
  canonical_diff_hash: string
  differences: RfxCompareItemDiff[]
  sections: RfxCompareItemDiff[]
  questions: RfxCompareItemDiff[]
  options: RfxCompareItemDiff[]
  rules: RfxCompareItemDiff[]
  scoring?: RfxCompareScoringDiff
}

export interface RfxRestoreVersionAsDraftRequest {
  change_summary: string
}

export interface RfxChangeImpactPreviewRequest {
  candidate_version_id: string
}

export interface RfxChangeImpactAnalysisResponse {
  impact_analysis_id: string
  tenant_id: string
  event_id: string
  source_version_id?: string | null
  candidate_version_id: string
  canonical_diff_hash: string
  impact_classes: RfxImpactClass[]
  affected_draft_response_count: number
  affected_submitted_response_count: number
  scoring_affecting: boolean
  knockout_affecting: boolean
  expires_at: string
}

export interface RfxCloneEventFromTemplateRequest {
  template_version_id: string
  rfx_number: string
  rfx_type: string
  category: string
  title: string
  owner_company_id: string
  description?: string | null
  currency_code?: string | null
  valid_from?: string | null
  valid_to?: string | null
  response_deadline?: string | null
}

export interface RfxCloneEventFromTemplateResponse {
  id: string
  draft_version_id: string
  source_template_id: string
  source_template_version_id: string
  source_version_number: number
  source_version_status: 'PUBLISHED' | 'SUPERSEDED'
  source_version_warning: boolean
}

export function isEventVersionEditable(status: RfxVersionStatus): boolean {
  return status === 'DRAFT'
}

export function isEventVersionReadOnly(status: RfxVersionStatus): boolean {
  return status === 'PUBLISHED' || status === 'SUPERSEDED' || status === 'ARCHIVED'
}

export function requiresImpactConfirmation(classes: RfxImpactClass[]): boolean {
  return classes.some((c) => c !== 'NON_MATERIAL')
}

export type RfxPublishReadiness = RfxPublishReadinessResult
