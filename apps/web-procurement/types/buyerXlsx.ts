export const BUYER_XLSX_CONFLICT_CODES = [
  'stale_target',
  'analysis_expired',
  'analysis_already_consumed',
  'idempotency_conflict',
  'actor_binding_denied',
  'canonical_hash_mismatch',
  'proposal_revalidation_failed',
] as const

export type BuyerXlsxConflictCode = (typeof BUYER_XLSX_CONFLICT_CODES)[number]

export interface BuyerXlsxPreviewIssue {
  severity: 'error' | 'warning'
  machine_code: string
  sheet?: string
  row?: number
  column?: string
  stable_code?: string
  message_key: string
  params?: Record<string, unknown>
}

export interface BuyerXlsxPreviewSummary {
  errors?: number
  warnings?: number
  sections_added?: number
  sections_changed?: number
  sections_removed?: number
  questions_added?: number
  questions_removed?: number
  lots_added?: number
  lots_changed?: number
  lots_removed?: number
}

export interface BuyerXlsxPreviewResponse {
  schema_name: string
  schema_version: string
  mode: 'UPDATE_DRAFT'
  target_event_id: string
  target_draft_version_id: string
  target_version_number: number
  target_event_row_version: number
  target_draft_row_version: number
  canonical_payload_hash?: string
  analysis_id?: string
  expires_at?: string
  ready_to_commit: boolean
  summary: BuyerXlsxPreviewSummary
  questionnaire_diff?: Record<string, unknown>
  lots_diff?: Record<string, unknown>
  errors: BuyerXlsxPreviewIssue[]
  warnings: BuyerXlsxPreviewIssue[]
}

export interface BuyerXlsxCommitRequest {
  analysis_id: string
}

export interface BuyerXlsxEntityChangeCounts {
  added?: number
  updated?: number
  deleted?: number
}

export interface BuyerXlsxCommitResponse {
  event_id: string
  draft_version_id: string
  analysis_id: string
  event_version: number
  draft_version: number
  committed_at: string
  changes?: {
    lots?: BuyerXlsxEntityChangeCounts
    sections?: BuyerXlsxEntityChangeCounts
    questions?: BuyerXlsxEntityChangeCounts
    options?: BuyerXlsxEntityChangeCounts
    rules?: BuyerXlsxEntityChangeCounts
  }
}

export interface BuyerXlsxPreviewResult {
  status: 200 | 422
  preview: BuyerXlsxPreviewResponse
}
