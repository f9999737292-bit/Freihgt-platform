export const BUYER_XLSX_ISSUE_I18N_PREFIX = 'tenders.buyerXlsx.issues'

export const BUYER_XLSX_KNOWN_ISSUE_CODES = [
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
  'cyclic_rule',
  'self_target_rule',
  'formula_denied',
  'merged_cell_denied',
  'hidden_sheet_denied',
  'hidden_content_warning',
  'hidden_row_warning',
  'hidden_column_warning',
  'competitor_column_denied',
  'I18N_MONOLINGUAL_MISMATCH',
  'i18n_monolingual_mismatch',
  'metadata_mismatch',
  'metadata_unreadable',
  'duplicate_metadata_key',
  'unknown_metadata_key',
  'missing_schema_metadata',
  'invalid_version_status',
  'event_id_mismatch',
  'version_id_mismatch',
  'tenant_id_mismatch',
  'event_row_version_mismatch',
  'version_row_version_mismatch',
  'invalid_creation_channel',
  'too_many_rows',
  'too_many_cells',
  'too_many_lots',
  'too_many_sections',
  'too_many_questions',
  'too_many_options',
  'too_many_rules',
  'preview_not_ready',
  'ISSUE_LIMIT_REACHED',
  'issue_limit_reached',
  'missing_lot',
  'invalid_lot',
  'missing_lot_status',
  'missing_lot_number',
  'duplicate_lot_number',
  'missing_lot_name',
  'duplicate_section_code',
  'invalid_section_code',
  'missing_section_title',
  'missing_section_code',
  'duplicate_question_code',
  'invalid_question_code',
  'invalid_question_type',
  'missing_question_label',
  'missing_question_identity',
  'invalid_validation_json',
  'duplicate_option_code',
  'invalid_option_code',
  'missing_option_label',
  'missing_option_identity',
  'select_requires_options',
  'options_not_allowed',
  'duplicate_rule_code',
  'missing_rule_code',
  'invalid_rule_action',
  'dangling_rule_reference',
  'dangling_section_reference',
  'dangling_question_reference',
  'dangling_source_question',
  'dangling_target_question',
  'invalid_rule',
] as const

export type BuyerXlsxKnownIssueCode = (typeof BUYER_XLSX_KNOWN_ISSUE_CODES)[number]

const knownIssueCodes = new Set<string>(BUYER_XLSX_KNOWN_ISSUE_CODES)

export interface BuyerXlsxIssueCopy {
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

export function buyerXlsxIssueLocation(issue: { sheet?: string; row?: number }): string {
  return [issue.sheet, issue.row != null ? String(issue.row) : '']
    .filter(Boolean)
    .join(':')
}

export function resolveBuyerXlsxIssueCopy(issue: {
  message_key?: string
  machine_code?: string
  sheet?: string
  row?: number
}): BuyerXlsxIssueCopy {
  const machineCode = issue.machine_code?.trim() || ''
  const messageKey = issue.message_key?.trim() || ''
  const fromMessage = lastMessageSegment(messageKey)
  const candidates = [machineCode, fromMessage].filter(Boolean)
  const matched = candidates.find((code) => knownIssueCodes.has(code))
  return {
    i18nKey: matched
      ? `${BUYER_XLSX_ISSUE_I18N_PREFIX}.${matched}`
      : `${BUYER_XLSX_ISSUE_I18N_PREFIX}.unknown`,
    known: Boolean(matched),
    technicalCode: machineCode || fromMessage || 'unknown',
    location: buyerXlsxIssueLocation(issue),
  }
}

export function buyerXlsxIssueShowsInternal(text: string, issue: { message_key?: string }): boolean {
  const messageKey = issue.message_key?.trim() || ''
  if (!messageKey) return false
  return text.includes(messageKey)
}
