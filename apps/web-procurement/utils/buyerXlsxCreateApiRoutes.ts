export const BUYER_XLSX_CREATE_PREVIEW_PATH = '/api/v1/rfx-events/xlsx-create/preview'
export const BUYER_XLSX_CREATE_COMMIT_PATH = '/api/v1/rfx-events/xlsx-create/commit'

export const BUYER_XLSX_CREATE_MAX_UPLOAD_BYTES = 5 * 1024 * 1024

export const BUYER_XLSX_CREATE_REQUIRED_MULTIPART_FIELDS = [
  'file',
  'owner_company_id',
  'rfx_number',
  'title',
  'rfx_type',
  'category',
] as const

export const BUYER_XLSX_CREATE_OPTIONAL_MULTIPART_FIELDS = [
  'description',
  'response_deadline',
  'currency_code',
] as const

export const BUYER_XLSX_CREATE_FORBIDDEN_MULTIPART_FIELDS = [
  'tenant_id',
  'auto_publish',
  'publish',
  'participants',
  'participant_ids',
  'erp',
  'erp_metadata',
  'canonical_payload_hash',
  'machine_code',
] as const

export const BUYER_XLSX_CREATE_CONTENT_TYPE =
  'application/vnd.openxmlformats-officedocument.spreadsheetml.sheet'
