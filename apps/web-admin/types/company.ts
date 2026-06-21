export const COMPANY_TYPES = [
  'SHIPPER',
  'CONSIGNEE',
  'CARRIER',
  'FORWARDER',
  'LSP',
  'WAREHOUSE',
  'TERMINAL',
  'SUPPLIER',
  'GOVERNMENT_AUTHORITY',
  'EDO_OPERATOR',
  'EPD_OPERATOR',
  'PLATFORM_OPERATOR',
] as const

export type CompanyType = (typeof COMPANY_TYPES)[number]

export const PREFERRED_LOCALES = ['ru-RU', 'en-US', 'zh-CN'] as const

export const COMPANY_STATUSES = ['ACTIVE', 'DELETED'] as const

export interface Company {
  id: string
  tenant_id: string
  legal_name: string
  short_name?: string | null
  legal_name_en?: string | null
  legal_name_zh?: string | null
  company_type: CompanyType | string
  tax_id?: string | null
  registration_number?: string | null
  country_code: string
  preferred_locale: string
  status: string
  created_at?: string
  updated_at?: string
  version?: number
}

export interface CreateCompanyPayload {
  tenant_id: string
  legal_name: string
  short_name?: string
  legal_name_en?: string
  legal_name_zh?: string
  company_type: string
  tax_id?: string
  registration_number?: string
  country_code: string
  preferred_locale: string
}

export interface UpdateCompanyPayload {
  legal_name?: string
  short_name?: string
  legal_name_en?: string
  legal_name_zh?: string
  company_type?: string
  tax_id?: string
  registration_number?: string
  country_code?: string
  preferred_locale?: string
  status?: string
}

export interface CompanyMemberRole {
  role_id: string
  code: string
  name: string
}

export interface CompanyMember {
  membership_id: string
  user_id: string
  email: string
  full_name: string
  phone?: string | null
  position?: string | null
  status: string
  roles: CompanyMemberRole[]
}

export interface AddCompanyMemberPayload {
  tenant_id: string
  user_id: string
  position?: string
  role_id?: string
}

export interface CompanyListFilters {
  search?: string
  company_type?: string
  status?: string
  country_code?: string
  limit?: number
  offset?: number
}

export interface CompanyFormErrors {
  legal_name?: string
  company_type?: string
  country_code?: string
  preferred_locale?: string
}

export function emptyCreateForm(): CreateCompanyPayload {
  return {
    tenant_id: '',
    legal_name: '',
    short_name: '',
    legal_name_en: '',
    legal_name_zh: '',
    company_type: 'SHIPPER',
    tax_id: '',
    registration_number: '',
    country_code: 'RU',
    preferred_locale: 'ru-RU',
  }
}

export function validateCompanyForm(payload: {
  legal_name: string
  company_type: string
  country_code: string
  preferred_locale: string
}): CompanyFormErrors {
  const errors: CompanyFormErrors = {}

  if (!payload.legal_name.trim()) {
    errors.legal_name = 'required'
  }
  if (!payload.company_type.trim()) {
    errors.company_type = 'required'
  }
  if (!payload.country_code.trim() || payload.country_code.trim().length !== 2) {
    errors.country_code = 'invalid'
  }
  if (!payload.preferred_locale.trim()) {
    errors.preferred_locale = 'required'
  }

  return errors
}

export function hasFormErrors(errors: CompanyFormErrors): boolean {
  return Object.keys(errors).length > 0
}

export function formatCompanyDate(value?: string | null): string {
  if (!value) return '—'
  const date = new Date(value)
  if (Number.isNaN(date.getTime())) return value
  return date.toLocaleString()
}

export function optionalString(value?: string): string | undefined {
  const trimmed = value?.trim()
  return trimmed ? trimmed : undefined
}
