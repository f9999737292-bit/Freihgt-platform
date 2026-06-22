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
