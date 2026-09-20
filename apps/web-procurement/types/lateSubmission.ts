export const LATE_SUBMISSION_REASON_CODES = [
  'TECHNICAL_FAILURE',
  'ORGANIZATIONAL_DELAY',
  'BUYER_REQUEST',
  'FORCE_MAJEURE',
  'OTHER',
] as const

export type LateSubmissionReasonCode = (typeof LATE_SUBMISSION_REASON_CODES)[number]

export const LATE_SUBMISSION_STATUSES = [
  'REQUESTED',
  'APPROVED',
  'REJECTED',
  'EXPIRED',
  'CONSUMED',
] as const

export type LateSubmissionStatus = (typeof LATE_SUBMISSION_STATUSES)[number]

export interface LateSubmissionRequest {
  id: string
  rfx_event_id: string
  carrier_company_id: string
  participant_id?: string
  reason_code: LateSubmissionReasonCode | string
  reason_text: string
  requested_until: string
  status: LateSubmissionStatus | string
  approved_valid_from?: string | null
  approved_valid_until?: string | null
  decision_comment?: string | null
  requested_by?: string
  decided_by?: string | null
  decided_at?: string | null
  consumed_at?: string | null
  version: number
  created_at?: string
  updated_at?: string
}

export interface LateSubmissionRequestList {
  items: LateSubmissionRequest[]
}

export interface CreateLateSubmissionRequestBody {
  reason_code: LateSubmissionReasonCode
  reason_text: string
  requested_until: string
}

export interface ApproveLateSubmissionRequestBody {
  expected_version: number
  approved_valid_from: string
  approved_valid_until: string
  decision_comment?: string
}

export interface RejectLateSubmissionRequestBody {
  expected_version: number
  decision_comment?: string
}
