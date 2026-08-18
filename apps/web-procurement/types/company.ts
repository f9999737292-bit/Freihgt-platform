export const COMPANY_TYPES = [
  'SHIPPER',
  'CONSIGNEE',
  'CARRIER',
  'FORWARDER',
  'LSP',
] as const

export interface Company {
  id: string
  tenant_id: string
  legal_name: string
  short_name?: string | null
  company_type: string
  country_code: string
  preferred_locale: string
  status: string
}

export interface CompanyMemberRole {
  role_id: string
  code: string
  name: string
}

export interface CompanyListFilters {
  search?: string
  company_type?: string
  status?: string
  limit?: number
  offset?: number
}

export interface UserCompanyMembership {
  membership_id: string
  company_id: string
  legal_name: string
  short_name?: string | null
  company_type: string
  position?: string | null
  membership_status: string
  roles: CompanyMemberRole[]
}
