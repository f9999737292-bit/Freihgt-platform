export const LOW_CODE_ENTITY_TYPES = [
  'TRANSPORT_ORDER',
  'SHIPMENT',
  'BILLING_REGISTER',
] as const

export type LowCodeEntityType = (typeof LOW_CODE_ENTITY_TYPES)[number]

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
