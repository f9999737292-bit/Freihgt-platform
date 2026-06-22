export const LOW_CODE_ENTITY_TYPES = [
  'TRANSPORT_ORDER',
  'SHIPMENT',
  'BILLING_REGISTER',
  'FREIGHT_REQUEST',
  'DOCUMENT',
  'RFX',
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

export interface ClonePublishedTemplateToDraftResponse {
  id: string
  source_template_id: string
  status: string
  version: number
  code: string
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
  validation_rule_json?: unknown
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
  validation_context?: PreviewRuleContext
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
  FREIGHT_REQUEST: 'DEMO-FR-001',
  DOCUMENT: 'DEMO-DOC-001',
  RFX: 'DEMO-RFX-001',
}

export const DEMO_EMPTY_ENTITY_REFS: Partial<Record<LowCodeEntityType, string>> = {
  TRANSPORT_ORDER: 'DEMO-TO-002',
}

export const PREVIEW_ENTITY_STATUS_PRESETS: Record<LowCodeEntityType, string[]> = {
  TRANSPORT_ORDER: ['DRAFT', 'READY_FOR_SOURCING', 'SOURCING_IN_PROGRESS', 'AWARDED'],
  SHIPMENT: ['PLANNED', 'PICKUP_SLOT_BOOKED', 'IN_TRANSIT', 'DELIVERED', 'READY_FOR_BILLING'],
  BILLING_REGISTER: ['DRAFT', 'CALCULATED', 'APPROVED', 'PAID', 'CLOSED'],
  FREIGHT_REQUEST: ['DRAFT', 'PUBLISHED', 'ACCEPTED', 'CANCELLED'],
  DOCUMENT: ['DRAFT', 'READY_FOR_SIGNING', 'SIGNED', 'ARCHIVED'],
  RFX: ['DRAFT', 'PUBLISHED', 'RESPONSES_OPEN', 'AWARDED'],
}

export function buildCustomFieldValuesEditorLink(
  entityType: string,
  entityId: string,
  entityStatus?: string | null,
): string {
  const query = new URLSearchParams({
    entity_type: entityType,
    entity_id: entityId,
  })
  if (entityStatus?.trim()) query.set('entity_status', entityStatus.trim())
  return `/low-code/custom-field-values?${query.toString()}`
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

export function moneyDraftKeys(fieldCode: string) {
  return {
    amount: `${fieldCode}__amount`,
    currency: `${fieldCode}__currency`,
  }
}

export function seedMoneyDraft(draft: Record<string, string>, fieldCode: string, value: unknown) {
  const keys = moneyDraftKeys(fieldCode)
  const parsed = parseCustomFieldValue(value)
  if (parsed && typeof parsed === 'object' && !Array.isArray(parsed)) {
    const obj = parsed as { amount?: unknown; currency?: unknown }
    draft[keys.amount] = obj.amount != null && obj.amount !== '' ? String(obj.amount) : ''
    draft[keys.currency] = obj.currency != null ? String(obj.currency) : ''
    return
  }
  draft[keys.amount] = ''
  draft[keys.currency] = ''
}

export function parseMoneyDraft(draft: Record<string, string>, fieldCode: string): unknown {
  const keys = moneyDraftKeys(fieldCode)
  const amountStr = draft[keys.amount] ?? ''
  const currency = (draft[keys.currency] ?? '').trim()
  if (!amountStr.trim() && !currency) return null
  const amount = Number(amountStr)
  if (Number.isNaN(amount)) {
    throw new Error('INVALID_NUMBER')
  }
  if (!currency) {
    throw new Error('INVALID_MONEY')
  }
  return { amount, currency }
}

export function valueToEditDraft(fieldType: string, value: unknown): string {
  const parsed = parseCustomFieldValue(value)
  if (parsed === undefined || parsed === null) return ''
  if (fieldType === 'CHECKBOX') return parsed ? 'true' : 'false'
  if (fieldType === 'NUMBER') return String(parsed)
  if (fieldType === 'TEXT' || fieldType === 'SELECT' || fieldType === 'CURRENCY') {
    return String(parsed)
  }
  if (fieldType === 'DATE' || fieldType === 'DATETIME') {
    return previewValueToInputString(fieldType, parsed)
  }
  if (fieldType === 'MULTI_SELECT') {
    return previewMultiSelectValues(parsed).join(', ')
  }
  return formatJsonValue(parsed)
}

export function seedEditDraftForField(
  draft: Record<string, string>,
  fieldType: string,
  fieldCode: string,
  value: unknown,
) {
  if (fieldType === 'MONEY') {
    seedMoneyDraft(draft, fieldCode, value)
    return
  }
  draft[fieldCode] = valueToEditDraft(fieldType, value)
}

export function parseEditDraftToValueJson(
  fieldType: string,
  draft: string,
  fieldCode?: string,
  fullDraft?: Record<string, string>,
): unknown {
  switch (fieldType) {
    case 'NUMBER': {
      if (!draft.trim()) return null
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
    case 'DATE':
      return draft.trim() ? draft.trim() : null
    case 'DATETIME': {
      if (!draft.trim()) return null
      const date = new Date(draft)
      if (Number.isNaN(date.getTime())) {
        throw new Error('INVALID_DATETIME')
      }
      return date.toISOString()
    }
    case 'MULTI_SELECT':
      if (!draft.trim()) return []
      return draft.split(',').map((part) => part.trim()).filter(Boolean)
    case 'MONEY':
      if (!fieldCode || !fullDraft) {
        throw new Error('INVALID_MONEY')
      }
      return parseMoneyDraft(fullDraft, fieldCode)
    default:
      if (!draft.trim()) return null
      return JSON.parse(draft)
  }
}

export function usesJsonTextareaFallback(fieldType: string): boolean {
  return ![
    'TEXT',
    'NUMBER',
    'SELECT',
    'MULTI_SELECT',
    'CHECKBOX',
    'CURRENCY',
    'DATE',
    'DATETIME',
    'MONEY',
  ].includes(fieldType)
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

export const FORM_BUILDER_PALETTE_TYPES = [
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
  'ADDRESS',
  'VEHICLE',
] as const

export type FormBuilderPresetId =
  | 'text'
  | 'select'
  | 'money'
  | 'date'
  | 'checkbox'
  | 'multi_select'
  | 'phone_comment'

const DEFAULT_SELECT_OPTIONS = JSON.stringify(
  {
    options: [
      { value: 'OPTION_A', label: 'Option A' },
      { value: 'OPTION_B', label: 'Option B' },
    ],
  },
  null,
  2,
)

export function nextFieldSortOrder(fields: DraftFormFieldDraft[]): number {
  if (!fields.length) return 100
  return Math.max(...fields.map((field) => field.sort_order)) + 10
}

export function createPaletteField(fieldType: string, sortOrder?: number): DraftFormFieldDraft {
  const base = createEmptyDraftField()
  const slug = fieldType.toLowerCase()
  return {
    ...base,
    field_type: fieldType,
    code: `new_${slug}_field`,
    label: `New ${fieldType.replace(/_/g, ' ').toLowerCase()} field`,
    sort_order: sortOrder ?? 100,
    options_json_text:
      fieldType === 'SELECT' || fieldType === 'MULTI_SELECT' ? DEFAULT_SELECT_OPTIONS : '',
  }
}

export function createPresetField(presetId: FormBuilderPresetId, sortOrder?: number): DraftFormFieldDraft {
  const order = sortOrder ?? 100
  switch (presetId) {
    case 'text':
      return { ...createEmptyDraftField(), code: 'text_field', label: 'Text field', field_type: 'TEXT', sort_order: order }
    case 'select':
      return {
        ...createEmptyDraftField(),
        code: 'select_field',
        label: 'Select field',
        field_type: 'SELECT',
        sort_order: order,
        options_json_text: DEFAULT_SELECT_OPTIONS,
      }
    case 'money':
      return { ...createEmptyDraftField(), code: 'amount', label: 'Amount', field_type: 'MONEY', sort_order: order }
    case 'date':
      return { ...createEmptyDraftField(), code: 'event_date', label: 'Date', field_type: 'DATE', sort_order: order }
    case 'checkbox':
      return { ...createEmptyDraftField(), code: 'confirmed', label: 'Confirmed', field_type: 'CHECKBOX', sort_order: order }
    case 'multi_select':
      return {
        ...createEmptyDraftField(),
        code: 'tags',
        label: 'Tags',
        field_type: 'MULTI_SELECT',
        sort_order: order,
        options_json_text: DEFAULT_SELECT_OPTIONS,
      }
    case 'phone_comment':
      return {
        ...createEmptyDraftField(),
        code: 'comment',
        label: 'Comment / phone note',
        field_type: 'TEXT',
        sort_order: order,
        validation_rule_json_text: JSON.stringify({ maxLength: 500 }, null, 2),
      }
    default:
      return createPaletteField('TEXT', order)
  }
}

export function reindexFieldSortOrders(fields: DraftFormFieldDraft[]): DraftFormFieldDraft[] {
  return fields.map((field, index) => ({ ...field, sort_order: 100 + index * 10 }))
}

export function duplicateDraftField(field: DraftFormFieldDraft, sortOrder: number): DraftFormFieldDraft {
  const suffix = '_copy'
  let code = `${field.code}${suffix}`
  if (code.length > 64) code = `${field.code.slice(0, 58)}${suffix}`
  return {
    ...field,
    _key: crypto.randomUUID(),
    code,
    label: `${field.label} (copy)`,
    sort_order: sortOrder,
  }
}

export function formatJsonDraftText(text: string): { ok: true; value: string } | { ok: false; error: string } {
  const trimmed = text.trim()
  if (!trimmed) return { ok: true, value: '' }
  try {
    return { ok: true, value: JSON.stringify(JSON.parse(trimmed), null, 2) }
  } catch {
    return { ok: false, error: 'INVALID_JSON' }
  }
}

export function validateJsonDraftText(text: string): { ok: true } | { ok: false; error: string } {
  const trimmed = text.trim()
  if (!trimmed) return { ok: true }
  try {
    JSON.parse(trimmed)
    return { ok: true }
  } catch {
    return { ok: false, error: 'INVALID_JSON' }
  }
}

export function validateSelectOptionsJson(text: string): { ok: true } | { ok: false; error: string } {
  const result = validateJsonDraftText(text)
  if (!result.ok) return result
  const trimmed = text.trim()
  if (!trimmed) return { ok: false, error: 'OPTIONS_REQUIRED' }
  try {
    const parsed = JSON.parse(trimmed) as { options?: unknown }
    if (!parsed || typeof parsed !== 'object' || !Array.isArray(parsed.options) || !parsed.options.length) {
      return { ok: false, error: 'OPTIONS_ARRAY_REQUIRED' }
    }
  } catch {
    return { ok: false, error: 'INVALID_JSON' }
  }
  return { ok: true }
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
    fieldsRequired?: string
    invalidJson: string
    duplicateSectionCode: string
    duplicateFieldCode: string
    invalidOptionsJson: string
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

  const sectionCodeSeen = new Map<string, number>()
  const fieldCodeSeen = new Map<string, string>()

  draft.sections.forEach((section, sectionIndex) => {
    const sectionCode = section.code.trim().toLowerCase()
    if (!section.code.trim()) {
      issues.push({ path: `sections.${sectionIndex}.code`, message: messages.sectionCodeRequired })
    } else if (sectionCodeSeen.has(sectionCode)) {
      issues.push({ path: `sections.${sectionIndex}.code`, message: messages.duplicateSectionCode })
    } else {
      sectionCodeSeen.set(sectionCode, sectionIndex)
    }
    if (!section.title.trim()) {
      issues.push({ path: `sections.${sectionIndex}.title`, message: messages.sectionTitleRequired })
    }
    section.fields.forEach((field, fieldIndex) => {
      const fieldPath = `sections.${sectionIndex}.fields.${fieldIndex}`
      const fieldCode = field.code.trim().toLowerCase()
      if (!field.code.trim()) {
        issues.push({ path: `${fieldPath}.code`, message: messages.fieldCodeRequired })
      } else if (fieldCodeSeen.has(fieldCode)) {
        issues.push({ path: `${fieldPath}.code`, message: messages.duplicateFieldCode })
      } else {
        fieldCodeSeen.set(fieldCode, fieldPath)
      }
      if (!field.label.trim()) {
        issues.push({ path: `${fieldPath}.label`, message: messages.fieldLabelRequired })
      }
      if (!field.field_type.trim()) {
        issues.push({ path: `${fieldPath}.field_type`, message: messages.fieldTypeRequired })
      }
      for (const [key, text] of [
        ['validation_rule_json', field.validation_rule_json_text],
        ['visibility_rule_json', field.visibility_rule_json_text],
      ] as const) {
        if (!text.trim()) continue
        if (!validateJsonDraftText(text).ok) {
          issues.push({ path: `${fieldPath}.${key}`, message: messages.invalidJson })
        }
      }
      if (field.options_json_text.trim()) {
        if (!validateJsonDraftText(field.options_json_text).ok) {
          issues.push({ path: `${fieldPath}.options_json`, message: messages.invalidJson })
        } else if (field.field_type === 'SELECT' || field.field_type === 'MULTI_SELECT') {
          if (!validateSelectOptionsJson(field.options_json_text).ok) {
            issues.push({ path: `${fieldPath}.options_json`, message: messages.invalidOptionsJson })
          }
        }
      } else if (field.field_type === 'SELECT' || field.field_type === 'MULTI_SELECT') {
        issues.push({ path: `${fieldPath}.options_json`, message: messages.invalidOptionsJson })
      }
    })
  })

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
        validation_rule_json: field.validation_rule_json,
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
        validation_rule_json: tryParseJsonText(field.validation_rule_json_text),
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

export interface PreviewFieldRequiredState {
  isRequired: boolean
  isConditional: boolean
  isMissing: boolean
}

export function flattenPreviewFields(sections: FormTemplatePreviewSection[]): FormTemplatePreviewField[] {
  const fields: FormTemplatePreviewField[] = []
  for (const section of sections) {
    fields.push(...section.fields)
  }
  return fields
}

export function isConditionalRequiredRule(rule: unknown): boolean {
  if (!rule || typeof rule !== 'object' || Array.isArray(rule)) return false
  const payload = rule as { if?: unknown; then?: unknown }
  if (!payload.if || typeof payload.if !== 'object' || Array.isArray(payload.if)) return false
  if (!Object.keys(payload.if as object).length) return false
  if (!payload.then || typeof payload.then !== 'object' || Array.isArray(payload.then)) return false
  const then = payload.then as { required?: unknown }
  if (then.required === true) return true
  return Array.isArray(then.required) && then.required.some((code) => typeof code === 'string' && code.trim())
}

function applyConditionalRequiredAction(
  then: unknown,
  sourceFieldCode: string,
  target: Set<string>,
) {
  if (!then || typeof then !== 'object' || Array.isArray(then)) return
  const payload = then as { required?: unknown }
  if (payload.required === true) {
    target.add(sourceFieldCode)
    return
  }
  if (!Array.isArray(payload.required)) return
  for (const code of payload.required) {
    if (typeof code === 'string' && code.trim()) target.add(code.trim())
  }
}

export function collectConditionallyRequiredFields(
  sections: FormTemplatePreviewSection[],
  values?: FormTemplatePreviewValues,
  context?: PreviewRuleContext,
): Set<string> {
  const required = new Set<string>()
  for (const field of flattenPreviewFields(sections)) {
    const rule = field.validation_rule_json
    if (!isConditionalRequiredRule(rule)) continue
    const payload = rule as { if?: unknown; then?: unknown }
    if (!evaluatePreviewVisibilityCondition(payload.if, values, context)) continue
    applyConditionalRequiredAction(payload.then, field.code, required)
  }
  return required
}

export function resolvePreviewFieldRequiredState(
  field: FormTemplatePreviewField,
  conditionalRequired: Set<string>,
  values?: FormTemplatePreviewValues,
): PreviewFieldRequiredState {
  const isConditional = !field.required && conditionalRequired.has(field.code)
  const isRequired = field.required || isConditional
  const isMissing = isRequired && !previewHasValue(previewFieldValue(values, field.code))
  return { isRequired, isConditional, isMissing }
}

export function countMissingRequiredPreviewFields(
  sections: FormTemplatePreviewSection[],
  conditionalRequired: Set<string>,
  values?: FormTemplatePreviewValues,
): number {
  let count = 0
  for (const field of flattenPreviewFields(sections)) {
    if (resolvePreviewFieldRequiredState(field, conditionalRequired, values).isMissing) {
      count += 1
    }
  }
  return count
}
