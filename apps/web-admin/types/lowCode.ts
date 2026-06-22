export const LOW_CODE_ENTITY_TYPES = [
  'TRANSPORT_ORDER',
  'SHIPMENT',
  'BILLING_REGISTER',
] as const

export type LowCodeEntityType = (typeof LOW_CODE_ENTITY_TYPES)[number]

export const LOW_CODE_ADMIN_ENTITY_TYPES = [
  'TRANSPORT_ORDER',
  'RFX',
  'FREIGHT_REQUEST',
  'BID',
  'SHIPMENT',
  'DOCUMENT',
  'BILLING_REGISTER',
  'COMPANY_PROFILE',
  'DRIVER_TASK',
] as const

export type LowCodeAdminEntityType = (typeof LOW_CODE_ADMIN_ENTITY_TYPES)[number]

export const LOW_CODE_FIELD_TYPES = [
  'TEXT',
  'NUMBER',
  'DATE',
  'DATETIME',
  'SELECT',
  'MULTI_SELECT',
  'CHECKBOX',
  'MONEY',
  'CURRENCY',
  'FILE',
  'COMPANY_REFERENCE',
  'DOCUMENT_REFERENCE',
  'ROUTE',
  'ADDRESS',
  'VEHICLE',
  'VAT_TAX',
] as const

export type LowCodeFieldType = (typeof LOW_CODE_FIELD_TYPES)[number]

export const LOW_CODE_TEMPLATE_STATUSES = ['DRAFT', 'PUBLISHED', 'ARCHIVED'] as const

export interface FormTemplateSummary {
  id: string
  tenant_id: string
  entity_type: string
  code: string
  name: string
  status: string
  version: number
  sections_count: number
  fields_count: number
  published_at?: string
}

export interface FormField {
  id: string
  code: string
  label: string
  field_type: string
  required: boolean
  read_only: boolean
  system_field: boolean
  options_json?: unknown
  validation_rule_json?: unknown
  visibility_rule_json?: unknown
  sort_order: number
}

export interface FormSection {
  id: string
  code: string
  title: string
  sort_order: number
  fields: FormField[]
}

export interface FormTemplateDetail {
  id: string
  tenant_id: string
  entity_type: string
  code: string
  name: string
  status: string
  version: number
  published_at?: string
  sections: FormSection[]
}

export interface AdminFormTemplateDetail extends FormTemplateDetail {
  description?: string
}

export interface DraftFormFieldDraft {
  _key: string
  code: string
  label: string
  field_type: string
  required: boolean
  read_only: boolean
  system_field: boolean
  sort_order: number
  options_json_text: string
  validation_rule_json_text: string
  visibility_rule_json_text: string
}

export interface DraftFormSectionDraft {
  _key: string
  code: string
  title: string
  sort_order: number
  fields: DraftFormFieldDraft[]
}

export interface DraftFormTemplateDraft {
  entity_type: string
  code: string
  name: string
  description: string
  sections: DraftFormSectionDraft[]
}

export interface DraftFormFieldPayload {
  code: string
  label: string
  field_type: string
  required: boolean
  read_only: boolean
  system_field: boolean
  sort_order: number
  options_json?: unknown
  validation_rule_json?: unknown
  visibility_rule_json?: unknown
}

export interface DraftFormSectionPayload {
  code: string
  title: string
  sort_order: number
  fields: DraftFormFieldPayload[]
}

export interface DraftFormTemplatePayload {
  entity_type: string
  code: string
  name: string
  description: string
  sections: DraftFormSectionPayload[]
}

export interface CreateDraftFormTemplateResponse {
  id: string
  status: string
  version: number
}

export interface ListAdminFormTemplatesParams {
  entity_type?: string
  status?: string
  limit?: number
}

export interface FormTemplatePreviewField {
  code: string
  label: string
  field_type: string
  required: boolean
  read_only: boolean
  system_field: boolean
  options_json?: unknown
  visibility_rule_json?: unknown
  sort_order: number
}

export interface FormTemplatePreviewSection {
  code: string
  title: string
  sort_order: number
  fields: FormTemplatePreviewField[]
}

export interface FormTemplatePreviewModel {
  name?: string
  code?: string
  sections: FormTemplatePreviewSection[]
}

export type FormTemplatePreviewValues = Record<string, unknown>

export interface PreviewRuleContext {
  role?: string
  entity_status?: string
}

export function mergePreviewRuleContext(
  ...contexts: Array<PreviewRuleContext | undefined | null>
): PreviewRuleContext | undefined {
  const merged: PreviewRuleContext = {}
  for (const context of contexts) {
    if (!context) continue
    if (context.role?.trim()) merged.role = context.role.trim()
    if (context.entity_status?.trim()) merged.entity_status = context.entity_status.trim()
  }
  return merged.role || merged.entity_status ? merged : undefined
}

export function hasPreviewRuleContext(context?: PreviewRuleContext | null): boolean {
  return Boolean(context?.role?.trim() || context?.entity_status?.trim())
}

export interface CustomFieldValueItem {
  field_id: string
  field_code: string
  value_json: unknown
  updated_at: string
}

export interface CustomFieldValuesResponse {
  tenant_id: string
  entity_type: string
  entity_id: string
  items: CustomFieldValueItem[]
}

export interface ListFormTemplatesResponse {
  items: FormTemplateSummary[]
}

export interface SaveCustomFieldValueItem {
  field_code: string
  value_json: unknown
}

export interface SaveCustomFieldValuesPayload {
  entity_type: string
  entity_id: string
  form_template_id: string
  values: SaveCustomFieldValueItem[]
}

export interface SaveCustomFieldValuesResponse {
  status: string
  tenant_id: string
  entity_type: string
  entity_id: string
  saved_count: number
}

export interface AuditEventItem {
  id: string
  tenant_id: string
  entity_type: string
  entity_id: string
  action: string
  actor?: string
  changed_fields: string[]
  old_values: Record<string, unknown>
  new_values: Record<string, unknown>
  created_at: string
}

export interface ListAuditEventsResponse {
  items: AuditEventItem[]
}

export interface ListAuditEventsParams {
  entity_type?: string
  entity_id?: string
  action?: string
  limit?: number
}

export interface SelectOption {
  label: string
  value: string
}

export const DEMO_ENTITY_REFS: Record<LowCodeEntityType, string> = {
  TRANSPORT_ORDER: 'DEMO-TO-001',
  SHIPMENT: 'DEMO-SH-PLANNED',
  BILLING_REGISTER: 'DEMO-BR-001',
}

export function formatLowCodeDate(value?: string): string {
  if (!value) return '—'
  const date = new Date(value)
  if (Number.isNaN(date.getTime())) return value
  return date.toLocaleString()
}

export function formatJsonValue(value: unknown): string {
  if (value === undefined || value === null) return '—'
  if (typeof value === 'string') {
    try {
      return JSON.stringify(JSON.parse(value), null, 2)
    } catch {
      return value
    }
  }
  try {
    return JSON.stringify(value, null, 2)
  } catch {
    return String(value)
  }
}

export function parseCustomFieldValue(value: unknown): unknown {
  if (typeof value === 'string') {
    try {
      return JSON.parse(value)
    } catch {
      return value
    }
  }
  return value
}

export function formatCustomFieldDisplayValue(value: unknown): string {
  const parsed = parseCustomFieldValue(value)
  if (parsed === undefined || parsed === null) return '—'
  if (typeof parsed === 'string') return parsed
  if (typeof parsed === 'number' || typeof parsed === 'boolean') return String(parsed)
  if (Array.isArray(parsed)) {
    if (parsed.every((item) => typeof item === 'string' || typeof item === 'number')) {
      return parsed.join(', ')
    }
    try {
      return JSON.stringify(parsed)
    } catch {
      return String(parsed)
    }
  }
  try {
    return JSON.stringify(parsed)
  } catch {
    return String(parsed)
  }
}

export function isCustomFieldComplexValue(value: unknown): boolean {
  const parsed = parseCustomFieldValue(value)
  if (parsed === null || parsed === undefined) return false
  if (typeof parsed === 'object') {
    if (Array.isArray(parsed)) {
      return !parsed.every((item) => typeof item === 'string' || typeof item === 'number')
    }
    return true
  }
  return false
}

export function flattenFormFields(sections: FormSection[]): FormField[] {
  return sections.flatMap((section) => section.fields ?? [])
}

export function parseSelectOptions(optionsJson: unknown): SelectOption[] {
  const parsed = parseCustomFieldValue(optionsJson)
  if (!parsed || typeof parsed !== 'object' || !('options' in parsed)) {
    return []
  }
  const options = (parsed as { options?: Array<{ value?: string; label?: string }> }).options ?? []
  return options
    .filter((option) => option.value)
    .map((option) => ({
      value: String(option.value),
      label: option.label ? String(option.label) : String(option.value),
    }))
}

export function valueToEditDraft(fieldType: string, value: unknown): string {
  const parsed = parseCustomFieldValue(value)
  if (parsed === undefined || parsed === null) return ''
  if (fieldType === 'CHECKBOX') return parsed ? 'true' : 'false'
  if (fieldType === 'NUMBER') return String(parsed)
  if (fieldType === 'TEXT' || fieldType === 'SELECT' || fieldType === 'CURRENCY') {
    return String(parsed)
  }
  return formatJsonValue(parsed)
}

export function parseEditDraftToValueJson(fieldType: string, draft: string): unknown {
  switch (fieldType) {
    case 'NUMBER': {
      const num = Number(draft)
      if (Number.isNaN(num)) {
        throw new Error('INVALID_NUMBER')
      }
      return num
    }
    case 'CHECKBOX':
      return draft === 'true'
    case 'TEXT':
    case 'SELECT':
    case 'CURRENCY':
      return draft
    default:
      return JSON.parse(draft)
  }
}

export function usesJsonTextareaFallback(fieldType: string): boolean {
  return !['TEXT', 'NUMBER', 'SELECT', 'CHECKBOX', 'CURRENCY'].includes(fieldType)
}

export function createEmptyDraftField(): DraftFormFieldDraft {
  return {
    _key: crypto.randomUUID(),
    code: '',
    label: '',
    field_type: 'TEXT',
    required: false,
    read_only: false,
    system_field: false,
    sort_order: 100,
    options_json_text: '',
    validation_rule_json_text: '{}',
    visibility_rule_json_text: '{}',
  }
}

export function createEmptyDraftSection(): DraftFormSectionDraft {
  return {
    _key: crypto.randomUUID(),
    code: '',
    title: '',
    sort_order: 100,
    fields: [],
  }
}

export function createEmptyDraftTemplate(): DraftFormTemplateDraft {
  return {
    entity_type: 'TRANSPORT_ORDER',
    code: '',
    name: '',
    description: '',
    sections: [createEmptyDraftSection()],
  }
}

export function parseOptionalJsonText(text: string): unknown | undefined {
  const trimmed = text.trim()
  if (!trimmed) return undefined
  return JSON.parse(trimmed)
}

export function adminDetailToDraft(detail: AdminFormTemplateDetail): DraftFormTemplateDraft {
  return {
    entity_type: detail.entity_type,
    code: detail.code,
    name: detail.name,
    description: detail.description ?? '',
    sections: (detail.sections ?? []).map((section) => ({
      _key: crypto.randomUUID(),
      code: section.code,
      title: section.title,
      sort_order: section.sort_order,
      fields: (section.fields ?? []).map((field) => ({
        _key: crypto.randomUUID(),
        code: field.code,
        label: field.label,
        field_type: field.field_type,
        required: field.required,
        read_only: field.read_only,
        system_field: field.system_field,
        sort_order: field.sort_order,
        options_json_text: field.options_json ? formatJsonValue(field.options_json) : '',
        validation_rule_json_text: field.validation_rule_json
          ? formatJsonValue(field.validation_rule_json)
          : '{}',
        visibility_rule_json_text: field.visibility_rule_json
          ? formatJsonValue(field.visibility_rule_json)
          : '{}',
      })),
    })),
  }
}

export function draftToPayload(draft: DraftFormTemplateDraft): DraftFormTemplatePayload {
  return {
    entity_type: draft.entity_type.trim(),
    code: draft.code.trim(),
    name: draft.name.trim(),
    description: draft.description.trim(),
    sections: draft.sections.map((section) => ({
      code: section.code.trim(),
      title: section.title.trim(),
      sort_order: section.sort_order,
      fields: section.fields.map((field) => ({
        code: field.code.trim(),
        label: field.label.trim(),
        field_type: field.field_type,
        required: field.required,
        read_only: field.read_only,
        system_field: field.system_field,
        sort_order: field.sort_order,
        options_json: parseOptionalJsonText(field.options_json_text),
        validation_rule_json: parseOptionalJsonText(field.validation_rule_json_text) ?? {},
        visibility_rule_json: parseOptionalJsonText(field.visibility_rule_json_text) ?? {},
      })),
    })),
  }
}

export interface DraftEditorValidationIssue {
  path: string
  message: string
}

export function validateDraftTemplateDraft(
  draft: DraftFormTemplateDraft,
  messages: {
    entityTypeRequired: string
    codeRequired: string
    nameRequired: string
    sectionCodeRequired: string
    sectionTitleRequired: string
    fieldCodeRequired: string
    fieldLabelRequired: string
    fieldTypeRequired: string
    sectionsRequired: string
    fieldsRequired: string
    invalidJson: string
  },
): DraftEditorValidationIssue[] {
  const issues: DraftEditorValidationIssue[] = []

  if (!draft.entity_type.trim()) {
    issues.push({ path: 'entity_type', message: messages.entityTypeRequired })
  }
  if (!draft.code.trim()) {
    issues.push({ path: 'code', message: messages.codeRequired })
  }
  if (!draft.name.trim()) {
    issues.push({ path: 'name', message: messages.nameRequired })
  }
  if (!draft.sections.length) {
    issues.push({ path: 'sections', message: messages.sectionsRequired })
  }

  let totalFields = 0
  draft.sections.forEach((section, sectionIndex) => {
    if (!section.code.trim()) {
      issues.push({ path: `sections.${sectionIndex}.code`, message: messages.sectionCodeRequired })
    }
    if (!section.title.trim()) {
      issues.push({ path: `sections.${sectionIndex}.title`, message: messages.sectionTitleRequired })
    }
    section.fields.forEach((field, fieldIndex) => {
      totalFields += 1
      if (!field.code.trim()) {
        issues.push({
          path: `sections.${sectionIndex}.fields.${fieldIndex}.code`,
          message: messages.fieldCodeRequired,
        })
      }
      if (!field.label.trim()) {
        issues.push({
          path: `sections.${sectionIndex}.fields.${fieldIndex}.label`,
          message: messages.fieldLabelRequired,
        })
      }
      if (!field.field_type.trim()) {
        issues.push({
          path: `sections.${sectionIndex}.fields.${fieldIndex}.field_type`,
          message: messages.fieldTypeRequired,
        })
      }
      for (const [key, text] of [
        ['options_json', field.options_json_text],
        ['validation_rule_json', field.validation_rule_json_text],
        ['visibility_rule_json', field.visibility_rule_json_text],
      ] as const) {
        if (!text.trim()) continue
        try {
          JSON.parse(text)
        } catch {
          issues.push({
            path: `sections.${sectionIndex}.fields.${fieldIndex}.${key}`,
            message: messages.invalidJson,
          })
        }
      }
    })
  })

  if (draft.sections.length && totalFields === 0) {
    issues.push({ path: 'fields', message: messages.fieldsRequired })
  }

  return issues
}

function tryParseJsonText(text: string): unknown | undefined {
  const trimmed = text.trim()
  if (!trimmed) return undefined
  try {
    return JSON.parse(trimmed)
  } catch {
    return undefined
  }
}

export function formTemplateDetailToPreview(detail: FormTemplateDetail): FormTemplatePreviewModel {
  return {
    name: detail.name,
    code: detail.code,
    sections: (detail.sections ?? []).map((section) => ({
      code: section.code,
      title: section.title,
      sort_order: section.sort_order,
      fields: (section.fields ?? []).map((field) => ({
        code: field.code,
        label: field.label,
        field_type: field.field_type,
        required: field.required,
        read_only: field.read_only,
        system_field: field.system_field,
        options_json: field.options_json,
        visibility_rule_json: field.visibility_rule_json,
        sort_order: field.sort_order,
      })),
    })),
  }
}

export function draftToPreviewModel(draft: DraftFormTemplateDraft): FormTemplatePreviewModel {
  return {
    name: draft.name,
    code: draft.code,
    sections: draft.sections.map((section) => ({
      code: section.code,
      title: section.title,
      sort_order: section.sort_order,
      fields: section.fields.map((field) => ({
        code: field.code,
        label: field.label,
        field_type: field.field_type,
        required: field.required,
        read_only: field.read_only,
        system_field: field.system_field,
        options_json: tryParseJsonText(field.options_json_text),
        visibility_rule_json: tryParseJsonText(field.visibility_rule_json_text),
        sort_order: field.sort_order,
      })),
    })),
  }
}

export function customFieldValuesToPreviewMap(
  items: Array<{ field_code: string; value_json: unknown }>,
): FormTemplatePreviewValues {
  const map: FormTemplatePreviewValues = {}
  for (const item of items) {
    map[item.field_code] = parseCustomFieldValue(item.value_json)
  }
  return map
}

export function previewFieldValue(
  values: FormTemplatePreviewValues | undefined,
  fieldCode: string,
): unknown {
  if (!values || !(fieldCode in values)) return undefined
  return values[fieldCode]
}

export function previewHasValue(value: unknown): boolean {
  return value !== undefined && value !== null && value !== ''
}

export function previewValueToInputString(fieldType: string, value: unknown): string {
  if (!previewHasValue(value)) return ''
  if (fieldType === 'CHECKBOX') return value ? 'true' : 'false'
  if (fieldType === 'NUMBER') return String(value)
  if (fieldType === 'DATETIME' && typeof value === 'string') {
    const date = new Date(value)
    if (!Number.isNaN(date.getTime())) {
      const pad = (n: number) => String(n).padStart(2, '0')
      return `${date.getFullYear()}-${pad(date.getMonth() + 1)}-${pad(date.getDate())}T${pad(date.getHours())}:${pad(date.getMinutes())}`
    }
  }
  if (fieldType === 'DATE' && typeof value === 'string') {
    return value.slice(0, 10)
  }
  if (Array.isArray(value)) return value.map(String).join(', ')
  if (typeof value === 'object') return formatJsonValue(value)
  return String(value)
}

export function previewCheckboxChecked(value: unknown): boolean {
  if (typeof value === 'boolean') return value
  if (typeof value === 'string') return value === 'true' || value === '1'
  return Boolean(value)
}

export function previewSelectOptions(optionsJson: unknown): SelectOption[] {
  const parsed = parseSelectOptions(optionsJson)
  if (parsed.length) return parsed
  const raw = parseCustomFieldValue(optionsJson)
  if (raw && typeof raw === 'object' && 'options' in raw) {
    const options = (raw as { options?: unknown[] }).options ?? []
    return options.map((option) => {
      if (typeof option === 'string' || typeof option === 'number') {
        return { value: String(option), label: String(option) }
      }
      if (option && typeof option === 'object' && 'value' in option) {
        const obj = option as { value?: unknown; label?: unknown }
        return {
          value: String(obj.value ?? ''),
          label: String(obj.label ?? obj.value ?? ''),
        }
      }
      return { value: String(option), label: String(option) }
    }).filter((option) => option.value)
  }
  return []
}

export function previewMultiSelectValues(value: unknown): string[] {
  const parsed = parseCustomFieldValue(value)
  if (Array.isArray(parsed)) return parsed.map(String)
  if (typeof parsed === 'string' && parsed.includes(',')) {
    return parsed.split(',').map((part) => part.trim()).filter(Boolean)
  }
  if (previewHasValue(parsed)) return [String(parsed)]
  return []
}

export function isPreviewComplexFieldType(fieldType: string): boolean {
  return ['ROUTE', 'ADDRESS', 'VEHICLE', 'VAT_TAX', 'MONEY', 'FILE', 'COMPANY_REFERENCE', 'DOCUMENT_REFERENCE'].includes(fieldType)
    || !['TEXT', 'NUMBER', 'DATE', 'DATETIME', 'SELECT', 'MULTI_SELECT', 'CHECKBOX', 'CURRENCY'].includes(fieldType)
}

function previewValuesEqual(left: unknown, right: unknown): boolean {
  if (left === right) return true
  if (left === undefined || left === null || left === '') {
    return right === undefined || right === null || right === ''
  }
  return String(left) === String(right)
}

function previewValueInList(value: unknown, list: unknown[]): boolean {
  return list.some((item) => previewValuesEqual(value, item))
}

function evaluatePreviewContextCondition(key: string, expected: unknown, context?: PreviewRuleContext): boolean {
  if (!context) return false
  if (key === 'context.role') {
    return previewValuesEqual(context.role, expected)
  }
  if (key === 'context.entity_status') {
    if (expected && typeof expected === 'object' && !Array.isArray(expected) && 'in' in expected) {
      const values = (expected as { in?: unknown[] }).in ?? []
      return previewValueInList(context.entity_status, values)
    }
    return previewValuesEqual(context.entity_status, expected)
  }
  return false
}

function evaluatePreviewFieldCondition(
  fieldCode: string,
  expected: unknown,
  values: FormTemplatePreviewValues | undefined,
): boolean {
  const actual = previewFieldValue(values, fieldCode)
  if (expected && typeof expected === 'object' && !Array.isArray(expected)) {
    if ('in' in expected) {
      return previewValueInList(actual, (expected as { in?: unknown[] }).in ?? [])
    }
    if ('not_in' in expected) {
      return !previewValueInList(actual, (expected as { not_in?: unknown[] }).not_in ?? [])
    }
  }
  return previewValuesEqual(actual, expected)
}

export function evaluatePreviewVisibilityCondition(
  condition: unknown,
  values: FormTemplatePreviewValues | undefined,
  context?: PreviewRuleContext,
): boolean {
  if (!condition || typeof condition !== 'object' || Array.isArray(condition)) return true
  const clause = condition as Record<string, unknown>
  const reservedFieldKeys = ['field', 'equals', 'not_equals', 'in', 'not_in']
  let matched = true

  if (typeof clause.field === 'string' && clause.field.trim()) {
    const actual = previewFieldValue(values, clause.field.trim())
    if ('equals' in clause) matched = matched && previewValuesEqual(actual, clause.equals)
    if ('not_equals' in clause) matched = matched && !previewValuesEqual(actual, clause.not_equals)
    if ('in' in clause) {
      matched = matched && previewValueInList(actual, Array.isArray(clause.in) ? clause.in : [])
    }
    if ('not_in' in clause) {
      matched = matched && !previewValueInList(actual, Array.isArray(clause.not_in) ? clause.not_in : [])
    }
  }

  for (const [key, expected] of Object.entries(clause)) {
    if (reservedFieldKeys.includes(key)) continue
    if (key.startsWith('context.')) {
      matched = matched && evaluatePreviewContextCondition(key, expected, context)
      continue
    }
    matched = matched && evaluatePreviewFieldCondition(key, expected, values)
  }

  return matched
}

export function isPreviewVisibilityRuleActive(rule: unknown): boolean {
  if (!rule || typeof rule !== 'object' || Array.isArray(rule)) return false
  const ifClause = (rule as { if?: unknown }).if
  if (!ifClause || typeof ifClause !== 'object' || Array.isArray(ifClause)) return false
  return Object.keys(ifClause as object).length > 0
}

export function isPreviewFieldVisible(
  field: Pick<FormTemplatePreviewField, 'visibility_rule_json'>,
  values: FormTemplatePreviewValues | undefined,
  context?: PreviewRuleContext,
): boolean {
  const rule = field.visibility_rule_json
  if (!isPreviewVisibilityRuleActive(rule)) return true
  const ifClause = (rule as { if?: unknown }).if
  return evaluatePreviewVisibilityCondition(ifClause, values, context)
}

export function filterPreviewSectionsForVisibility(
  sections: FormTemplatePreviewSection[],
  values: FormTemplatePreviewValues | undefined,
  context?: PreviewRuleContext,
): { sections: FormTemplatePreviewSection[]; hiddenFieldCount: number } {
  let hiddenFieldCount = 0
  const filteredSections: FormTemplatePreviewSection[] = []

  for (const section of sections) {
    const visibleFields = section.fields.filter((field) => {
      const visible = isPreviewFieldVisible(field, values, context)
      if (!visible) hiddenFieldCount += 1
      return visible
    })
    if (visibleFields.length) {
      filteredSections.push({ ...section, fields: visibleFields })
    }
  }

  return { sections: filteredSections, hiddenFieldCount }
}
