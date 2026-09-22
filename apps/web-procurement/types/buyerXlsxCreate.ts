export const BUYER_XLSX_CREATE_MODE = 'CREATE_NEW_DRAFT' as const

export const BUYER_XLSX_CREATE_CONFLICT_CODES = [
  'analysis_expired',
  'analysis_already_consumed',
  'idempotency_conflict',
  'canonical_hash_mismatch',
  'proposal_revalidation_failed',
] as const

export type BuyerXlsxCreateConflictCode = (typeof BUYER_XLSX_CREATE_CONFLICT_CODES)[number]

export interface BuyerXlsxCreatePreviewIssue {
  severity: 'error' | 'warning'
  machine_code: string
  sheet?: string
  row?: number
  column?: string
  stable_code?: string
  message_key: string
  params?: Record<string, unknown>
}

export interface BuyerXlsxCreateDraftSummary {
  rfx_number: string
  title: string
  rfx_type: string
  category: string
  description?: string
  response_deadline?: string
  currency_code?: string
  lot_count: number
  section_count: number
  question_count: number
}

export interface BuyerXlsxCreatePreviewResponse {
  mode: typeof BUYER_XLSX_CREATE_MODE
  schema_name: string
  schema_version: string
  owner_company_id: string
  analysis_id?: string
  expires_at?: string
  ready_to_commit: boolean
  normalized_draft_summary: BuyerXlsxCreateDraftSummary
  change_counts?: Record<string, unknown>
  errors: BuyerXlsxCreatePreviewIssue[]
  warnings: BuyerXlsxCreatePreviewIssue[]
}

export interface BuyerXlsxCreateCommitRequest {
  analysis_id: string
}

export interface BuyerXlsxCreateCommitResponse {
  event_id: string
  analysis_id: string
  creation_channel: 'EXCEL'
  status: 'DRAFT'
  draft_version_id: string
  draft_version_number: number
  questionnaire_enabled: boolean
  created_counts?: Record<string, unknown>
  committed_at: string
}

export interface BuyerXlsxCreatePreviewResult {
  status: 200 | 422
  preview: BuyerXlsxCreatePreviewResponse
}

export interface BuyerXlsxCreateMetadata {
  owner_company_id: string
  rfx_number: string
  title: string
  rfx_type: string
  category: string
  description: string
  response_deadline: string
  currency_code: string
}
