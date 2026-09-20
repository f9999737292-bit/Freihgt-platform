export const CARRIER_XLSX_ISSUE_I18N_PREFIX = 'carrierTenders.xlsx.issues'

export const CARRIER_XLSX_KNOWN_ISSUE_CODES = [
  'invalid_multipart',
  'invalid_content_type',
  'file_too_large',
  'invalid_xlsx_signature',
  'invalid_workbook',
  'unsafe_package',
  'unsupported_schema',
  'missing_sheet',
  'unexpected_sheet',
  'sheet_order_mismatch',
  'invalid_header',
  'missing_header_row',
  'missing_required_header',
  'header_order_mismatch',
  'header_column_count_mismatch',
  'duplicate_header',
  'unexpected_column',
  'missing_required_value',
  'invalid_type',
  'invalid_bool',
  'invalid_int',
  'invalid_float',
  'invalid_json',
  'string_too_long',
  'duplicate_stable_code',
  'dangling_reference',
  'formula_denied',
  'merged_cell_denied',
  'hidden_sheet_denied',
  'hidden_content_warning',
  'hidden_row_warning',
  'hidden_column_warning',
  'competitor_column_denied',
  'metadata_mismatch',
  'metadata_unreadable',
  'duplicate_metadata_key',
  'unknown_metadata_key',
  'missing_schema_metadata',
  'event_id_mismatch',
  'tenant_id_mismatch',
  'event_row_version_mismatch',
  'too_many_rows',
  'too_many_cells',
  'preview_not_ready',
  'ISSUE_LIMIT_REACHED',
  'issue_limit_reached',
  'missing_lot',
  'invalid_lot',
  'missing_lot_number',
  'duplicate_lot_number',
  'duplicate_question_code',
  'missing_question_code',
  'unknown_question_code',
  'response_not_editable',
  'export_mode_not_editable',
  'unknown_lot',
  'duplicate_event_level_offer',
  'rfx_lot_id_required',
  'rfx_lot_not_found',
  'invalid_amount',
  'invalid_answer',
  'invalid_offer_line',
  'currency_mismatch',
  'sheet_unreadable',
] as const

export type CarrierXlsxKnownIssueCode = (typeof CARRIER_XLSX_KNOWN_ISSUE_CODES)[number]

const knownIssueCodes = new Set<string>(CARRIER_XLSX_KNOWN_ISSUE_CODES)

export interface CarrierXlsxIssueCopy {
  i18nKey: string
  known: boolean
  technicalCode: string
  location: string
}

function lastMessageSegment(messageKey: string): string {
  const trimmed = messageKey.trim()
  if (!trimmed) return ''
  const parts = trimmed.split('.')
  return parts[parts.length - 1] || ''
}

export function carrierXlsxIssueLocation(issue: { sheet?: string; row?: number }): string {
  return [issue.sheet, issue.row != null ? String(issue.row) : '']
    .filter(Boolean)
    .join(':')
}

export function resolveCarrierXlsxIssueCopy(issue: {
  message_key?: string
  machine_code?: string
  sheet?: string
  row?: number
}): CarrierXlsxIssueCopy {
  const machineCode = issue.machine_code?.trim() || ''
  const messageKey = issue.message_key?.trim() || ''
  const fromMessage = lastMessageSegment(messageKey)
  const candidates = [machineCode, fromMessage].filter(Boolean)
  const matched = candidates.find((code) => knownIssueCodes.has(code))
  return {
    i18nKey: matched
      ? `${CARRIER_XLSX_ISSUE_I18N_PREFIX}.${matched}`
      : `${CARRIER_XLSX_ISSUE_I18N_PREFIX}.unknown`,
    known: Boolean(matched),
    technicalCode: machineCode || fromMessage || 'unknown',
    location: carrierXlsxIssueLocation(issue),
  }
}

export function carrierXlsxIssueShowsInternal(text: string, issue: { message_key?: string }): boolean {
  const messageKey = issue.message_key?.trim() || ''
  if (!messageKey) return false
  return text.includes(messageKey)
}
