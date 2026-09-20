export const CARRIER_XLSX_CONFLICT_CODES = [
  'stale_target',
  'analysis_expired',
  'analysis_already_consumed',
  'idempotency_conflict',
  'actor_binding_denied',
  'canonical_hash_mismatch',
  'proposal_revalidation_failed',
  'response_not_editable',
] as const

export type CarrierXlsxConflictCode = (typeof CARRIER_XLSX_CONFLICT_CODES)[number]

export interface CarrierXlsxPreviewIssue {
  severity: 'error' | 'warning'
  machine_code: string
  sheet?: string
  row?: number
  column?: string
  stable_code?: string
  message_key: string
  params?: Record<string, unknown>
}

export interface CarrierXlsxPreviewSummary {
  errors?: number
  warnings?: number
  answers_added?: number
  answers_changed?: number
  answers_removed?: number
  offer_lines_added?: number
  offer_lines_changed?: number
  offer_lines_removed?: number
}

export interface CarrierXlsxPreviewResponse {
  schema_name: string
  schema_version: string
  mode: 'UPDATE_CARRIER_DRAFT'
  target_event_id: string
  target_response_id: string
  target_rfx_version_id: string
  target_version_number: number
  target_event_row_version: number
  target_response_save_version: number
  canonical_payload_hash?: string
  analysis_id?: string
  expires_at?: string
  ready_to_commit: boolean
  summary: CarrierXlsxPreviewSummary
  answers_diff?: Record<string, unknown>
  offer_lines_diff?: Record<string, unknown>
  errors: CarrierXlsxPreviewIssue[]
  warnings: CarrierXlsxPreviewIssue[]
}

export interface CarrierXlsxCommitRequest {
  analysis_id: string
}

export interface CarrierXlsxEntityChangeCounts {
  added?: number
  updated?: number
  deleted?: number
}

export interface CarrierXlsxCommitResponse {
  event_id: string
  response_id: string
  analysis_id: string
  save_version: number
  committed_at: string
  changes?: {
    answers?: CarrierXlsxEntityChangeCounts
    offer_lines?: CarrierXlsxEntityChangeCounts
  }
}

export interface CarrierXlsxPreviewResult {
  status: 200 | 422
  preview: CarrierXlsxPreviewResponse
}
