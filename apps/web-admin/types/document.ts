export const DOCUMENT_TYPES = [
  'ETRN',
  'EPD',
  'WAYBILL',
  'POD',
  'DISCREPANCY_ACT',
  'CLAIM',
  'INVOICE',
  'VAT_INVOICE',
  'ACT',
  'UPD',
  'ECMR',
] as const

export const DOCUMENT_STATUSES = [
  'DRAFT',
  'READY_FOR_SIGNING',
  'SIGNING_IN_PROGRESS',
  'SIGNED',
  'SENT_TO_OPERATOR',
  'ACCEPTED',
  'REJECTED',
  'ARCHIVED',
  'CANCELLED',
] as const

export const RELATED_ENTITY_TYPES = [
  'SHIPMENT',
  'TRANSPORT_ORDER',
  'BILLING_REGISTER',
  'INVOICE',
  'COMPANY',
] as const

export const LEGAL_LANGUAGES = ['ru-RU', 'en-US', 'zh-CN'] as const

export const FILE_TYPES = ['PDF', 'XML', 'JSON', 'IMAGE', 'SIGNATURE', 'OTHER'] as const

export const SIGNATURE_TYPES = [
  'SIMPLE_ELECTRONIC',
  'ENHANCED_UNQUALIFIED',
  'ENHANCED_QUALIFIED',
] as const

export interface DocumentRecord {
  id: string
  tenant_id: string
  document_number: string
  document_type: string
  document_status: string
  owner_company_id?: string
  related_entity_type?: string | null
  related_entity_id?: string | null
  legal_language?: string
  created_at?: string
  updated_at?: string
  version?: number
}

export interface DocumentVersion {
  id: string
  document_id?: string
  version_number: number
  payload_json?: Record<string, unknown> | null
  payload_xml_path?: string | null
  pdf_file_path?: string | null
  created_at?: string
}

export interface DocumentFile {
  id: string
  document_id?: string
  document_version_id?: string | null
  file_type: string
  storage_provider: string
  bucket_name?: string | null
  object_key: string
  file_name?: string | null
  mime_type?: string | null
  file_size_bytes?: number | null
  checksum_sha256?: string | null
  created_at?: string
}

export interface DocumentDetail extends DocumentRecord {
  owner_company_id: string
  legal_language: string
  latest_version?: DocumentVersion | null
  files?: DocumentFile[]
}

export interface SigningSession {
  id: string
  tenant_id?: string
  document_id?: string
  status: string
  required_signers_count: number
  completed_signers_count: number
  created_at?: string
  expires_at?: string | null
}

export interface Signature {
  id: string
  signing_session_id?: string
  signer_user_id?: string | null
  signer_company_id?: string | null
  signature_type?: string
  certificate_fingerprint?: string | null
  signed_at?: string | null
  verification_status?: string
  created_at?: string
}

export interface ListDocumentsFilters {
  document_type?: string
  document_status?: string
  related_entity_type?: string
  related_entity_id?: string
  search?: string
  limit?: number
  offset?: number
}

export interface CreateDocumentPayload {
  tenant_id: string
  document_number: string
  document_type: string
  owner_company_id: string
  related_entity_type: string
  related_entity_id: string
  legal_language: string
  payload_json: Record<string, unknown>
}

export interface CreateDocumentVersionPayload {
  tenant_id: string
  payload_json: Record<string, unknown>
  payload_xml_path?: string | null
  pdf_file_path?: string | null
}

export interface AddDocumentFilePayload {
  tenant_id: string
  document_version_id: string
  file_type: string
  storage_provider: string
  bucket_name?: string
  object_key: string
  file_name: string
  mime_type: string
  file_size_bytes?: number
  checksum_sha256?: string
}

export interface CreateSigningSessionPayload {
  tenant_id: string
  required_signers_count: number
  expires_at?: string
}

export interface CancelDocumentPayload {
  tenant_id: string
  reason: string
}

export interface AddSignaturePayload {
  tenant_id: string
  signer_user_id: string
  signer_company_id: string
  signature_type: string
  signature_payload_path?: string
  certificate_fingerprint?: string
}

export interface DocumentCreateForm {
  document_number: string
  document_type: string
  owner_company_id: string
  related_entity_type: string
  related_entity_id: string
  legal_language: string
  payload_json: string
}

export interface DocumentVersionForm {
  payload_json: string
  payload_xml_path: string
  pdf_file_path: string
}

export interface DocumentFileForm {
  document_version_id: string
  file_type: string
  storage_provider: string
  bucket_name: string
  object_key: string
  file_name: string
  mime_type: string
  file_size_bytes: string
  checksum_sha256: string
}

export interface SigningSessionForm {
  required_signers_count: string
  expires_at: string
}

export interface SignatureForm {
  signer_user_id: string
  signer_company_id: string
  signature_type: string
  certificate_fingerprint: string
}

export interface DocumentFormErrors {
  document_number?: string
  document_type?: string
  owner_company_id?: string
  related_entity_type?: string
  related_entity_id?: string
  legal_language?: string
  payload_json?: string
}

export interface DocumentVersionFormErrors {
  payload_json?: string
}

export interface DocumentFileFormErrors {
  document_version_id?: string
  file_type?: string
  file_name?: string
  mime_type?: string
}

export interface SigningSessionFormErrors {
  required_signers_count?: string
  expires_at?: string
}

export interface SignatureFormErrors {
  signer_user_id?: string
  signer_company_id?: string
  signature_type?: string
}

function defaultExpiresLocal(): string {
  const d = new Date()
  d.setDate(d.getDate() + 14)
  d.setHours(18, 0, 0, 0)
  const pad = (n: number) => String(n).padStart(2, '0')
  return `${d.getFullYear()}-${pad(d.getMonth() + 1)}-${pad(d.getDate())}T${pad(d.getHours())}:${pad(d.getMinutes())}`
}

export function emptyDocumentCreateForm(
  shipmentId = '',
  documentType = 'POD',
): DocumentCreateForm {
  return {
    document_number: `DOC-${Date.now().toString().slice(-6)}`,
    document_type: documentType || 'POD',
    owner_company_id: '',
    related_entity_type: 'SHIPMENT',
    related_entity_id: shipmentId,
    legal_language: 'ru-RU',
    payload_json: JSON.stringify(
      { comment: 'Proof of delivery created from web-admin' },
      null,
      2,
    ),
  }
}

export function emptyDocumentVersionForm(): DocumentVersionForm {
  return {
    payload_json: JSON.stringify({ comment: 'Updated from web-admin' }, null, 2),
    payload_xml_path: '',
    pdf_file_path: '',
  }
}

export function emptyDocumentFileForm(versionId = ''): DocumentFileForm {
  return {
    document_version_id: versionId,
    file_type: 'PDF',
    storage_provider: 'S3',
    bucket_name: 'freight-documents',
    object_key: 'documents/DOC-TEST-000001.pdf',
    file_name: 'DOC-TEST-000001.pdf',
    mime_type: 'application/pdf',
    file_size_bytes: '123456',
    checksum_sha256: 'test-checksum',
  }
}

export function emptySigningSessionForm(): SigningSessionForm {
  return {
    required_signers_count: '2',
    expires_at: defaultExpiresLocal(),
  }
}

export function emptySignatureForm(): SignatureForm {
  return {
    signer_user_id: '',
    signer_company_id: '',
    signature_type: 'SIMPLE_ELECTRONIC',
    certificate_fingerprint: 'test-fingerprint',
  }
}

export function toRFC3339(value: string): string {
  if (!value.trim()) return ''
  if (value.includes('T') && !value.endsWith('Z') && !value.includes('+')) {
    return new Date(value).toISOString()
  }
  return value
}

export function parseJsonObject(value: string): Record<string, unknown> | null {
  if (!value.trim()) return {}
  try {
    const parsed = JSON.parse(value)
    if (parsed && typeof parsed === 'object' && !Array.isArray(parsed)) {
      return parsed as Record<string, unknown>
    }
    return null
  } catch {
    return null
  }
}

export function validateDocumentCreateForm(form: DocumentCreateForm): DocumentFormErrors {
  const errors: DocumentFormErrors = {}
  if (!form.document_number.trim()) errors.document_number = 'required'
  if (!form.document_type.trim()) errors.document_type = 'required'
  if (!form.owner_company_id.trim()) errors.owner_company_id = 'required'
  if (!form.related_entity_type.trim()) errors.related_entity_type = 'required'
  if (!form.related_entity_id.trim()) errors.related_entity_id = 'required'
  if (!form.legal_language.trim()) errors.legal_language = 'required'
  return errors
}

export function validateDocumentVersionForm(form: DocumentVersionForm): DocumentVersionFormErrors {
  const errors: DocumentVersionFormErrors = {}
  if (!form.payload_json.trim()) errors.payload_json = 'invalidJson'
  else if (!parseJsonObject(form.payload_json)) errors.payload_json = 'invalidJson'
  return errors
}

export function validateDocumentFileForm(form: DocumentFileForm): DocumentFileFormErrors {
  const errors: DocumentFileFormErrors = {}
  if (!form.document_version_id.trim()) errors.document_version_id = 'required'
  if (!form.file_type.trim()) errors.file_type = 'required'
  if (!form.file_name.trim()) errors.file_name = 'required'
  if (!form.mime_type.trim()) errors.mime_type = 'required'
  return errors
}

export function validateSigningSessionForm(form: SigningSessionForm): SigningSessionFormErrors {
  const errors: SigningSessionFormErrors = {}
  const count = Number(form.required_signers_count)
  if (!form.required_signers_count.trim() || Number.isNaN(count) || count <= 0) {
    errors.required_signers_count = 'positive'
  }
  if (!form.expires_at.trim()) errors.expires_at = 'required'
  return errors
}

export function validateSignatureForm(form: SignatureForm): SignatureFormErrors {
  const errors: SignatureFormErrors = {}
  if (!form.signer_user_id.trim()) errors.signer_user_id = 'required'
  if (!form.signer_company_id.trim()) errors.signer_company_id = 'required'
  if (!form.signature_type.trim()) errors.signature_type = 'required'
  return errors
}

export function hasFormErrors<T extends object>(errors: T): boolean {
  return Object.keys(errors).length > 0
}

export function canCancelDocument(status: string): boolean {
  return !['SIGNED', 'ARCHIVED', 'CANCELLED'].includes(status)
}

export function canArchiveDocument(status: string): boolean {
  return status === 'SIGNED' || status === 'ACCEPTED'
}

export function formatDocumentDate(value?: string | null): string {
  if (!value) return '—'
  const date = new Date(value)
  if (Number.isNaN(date.getTime())) return value
  return date.toLocaleString()
}

export function formatPayloadJson(value?: Record<string, unknown> | null): string {
  if (!value || !Object.keys(value).length) return '—'
  try {
    return JSON.stringify(value, null, 2)
  } catch {
    return '—'
  }
}

export function mergeDocumentVersion(
  versions: DocumentVersion[],
  version: DocumentVersion,
): DocumentVersion[] {
  const next = [...versions.filter((item) => item.id !== version.id), version]
  return next.sort((a, b) => (a.version_number ?? 0) - (b.version_number ?? 0))
}

export function signingSessionStorageKey(documentId: string): string {
  return `doc-signing-session-${documentId}`
}
