export const CONTRACT_STATUSES = [
  'DRAFT',
  'ACTIVE',
  'SUSPENDED',
  'TERMINATED',
  'EXPIRED',
  'CANCELLED',
] as const

export type ContractStatus = (typeof CONTRACT_STATUSES)[number]

export const RATE_VERSION_STATUSES = ['DRAFT', 'ACTIVE', 'SUPERSEDED'] as const

export type RateCardVersionStatus = (typeof RATE_VERSION_STATUSES)[number]

export const COMPONENT_TYPES = [
  'BASE_FREIGHT',
  'FUEL_SURCHARGE',
  'WAITING',
  'DETENTION',
] as const

export type RateComponentType = (typeof COMPONENT_TYPES)[number]

export const CALCULATION_METHODS = ['FLAT', 'PERCENT', 'UNIT_RATE'] as const

export type CalculationMethod = (typeof CALCULATION_METHODS)[number]

export const RESOLVE_STATUSES = ['MATCHED', 'NO_MATCH', 'AMBIGUOUS'] as const

export type ResolveStatus = (typeof RESOLVE_STATUSES)[number]

export interface TransportContract {
  id: string
  tenant_id: string
  buyer_company_id: string
  carrier_company_id: string
  contract_number: string
  external_reference?: string | null
  name: string
  description?: string | null
  status: ContractStatus
  valid_from: string
  valid_to?: string | null
  currency_code: string
  created_at: string
  updated_at: string
  version: number
}

export interface RateCard {
  id: string
  tenant_id: string
  contract_id: string
  name: string
  description?: string | null
  created_at: string
  updated_at: string
  version: number
}

export interface RateCardVersion {
  id: string
  tenant_id: string
  rate_card_id: string
  version_number: number
  valid_from: string
  valid_to?: string | null
  status: RateCardVersionStatus
  supersedes_version_id?: string | null
  created_at: string
  activated_at?: string | null
  version: number
}

export interface RateLine {
  id: string
  tenant_id: string
  rate_card_version_id: string
  origin_location_id: string
  destination_location_id: string
  equipment_type: string
  transport_mode: string
  created_at: string
  updated_at: string
}

export interface RateComponent {
  id: string
  tenant_id: string
  rate_line_id: string
  component_type: RateComponentType
  calculation_method: CalculationMethod
  amount?: string | null
  percent_value?: string | null
  unit_code?: string | null
  created_at: string
  updated_at: string
}

export interface ContractListResponse {
  items: TransportContract[]
}

export interface RateCardListResponse {
  items: RateCard[]
}

export interface RateVersionListResponse {
  items: RateCardVersion[]
}

export interface RateLineListResponse {
  items: RateLine[]
}

export interface RateComponentListResponse {
  items: RateComponent[]
}

export interface CreateTransportContractInput {
  buyer_company_id: string
  carrier_company_id: string
  contract_number: string
  external_reference?: string | null
  name: string
  description?: string | null
  valid_from: string
  valid_to?: string | null
  currency_code: string
}

export interface PatchTransportContractInput {
  name?: string | null
  description?: string | null
  external_reference?: string | null
  valid_to?: string | null
}

export interface CreateRateCardInput {
  name: string
  description?: string | null
}

export interface CreateRateVersionInput {
  valid_from: string
  valid_to?: string | null
}

export interface PatchRateVersionInput {
  valid_from?: string | null
  valid_to?: string | null
}

export interface CreateRateLineInput {
  origin_location_id: string
  destination_location_id: string
  equipment_type: string
  transport_mode: string
}

export interface PatchRateLineInput {
  origin_location_id?: string | null
  destination_location_id?: string | null
  equipment_type?: string | null
  transport_mode?: string | null
}

export interface CreateRateComponentInput {
  component_type: RateComponentType
  calculation_method: CalculationMethod
  amount?: string | null
  percent_value?: string | null
  unit_code?: string | null
}

export interface PatchRateComponentInput {
  amount?: string | null
  percent_value?: string | null
  unit_code?: string | null
}

export interface RateResolutionRequest {
  buyer_company_id: string
  carrier_company_id: string
  origin_location_id: string
  destination_location_id: string
  equipment_type: string
  transport_mode: string
  pricing_date: string
  currency_code?: string | null
}

export interface RateResolutionComponent {
  component_type: string
  calculation_method: string
  amount?: string
  percent_value?: string
  unit_code?: string
}

export interface RateResolutionResult {
  status: ResolveStatus
  pricing_source?: string
  contract_id?: string
  rate_card_id?: string
  rate_version_id?: string
  rate_line_id?: string
  contract_number?: string
  rate_card_name?: string
  version_number?: number
  origin_location_id?: string
  destination_location_id?: string
  equipment_type?: string
  transport_mode?: string
  currency_code?: string
  component_breakdown_status?: string
  base_amount?: string
  total_amount?: string
  components?: RateResolutionComponent[]
  pricing_date?: string
  resolved_at?: string
  resolver_version?: string
  reason_code?: string
  buyer_company_id?: string
  carrier_company_id?: string
}

export interface ContractListFilters {
  q?: string
  status?: ContractStatus | ''
  carrier_company_id?: string
  limit?: number
  offset?: number
}

export interface LocationSummary {
  id: string
  name: string
  city?: string | null
  region?: string | null
  country_code?: string | null
}
