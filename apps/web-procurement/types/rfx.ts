export const FREIGHT_REQUEST_STATUSES = [
  'DRAFT',
  'PUBLISHED',
  'RESPONSES_OPEN',
  'AWARDED',
] as const

export type FreightRequestStatus = (typeof FREIGHT_REQUEST_STATUSES)[number]

export const FREIGHT_REQUEST_TYPES = [
  'SPOT',
  'MINI_TENDER',
  'LANE_TENDER',
  'CONTRACT_TENDER',
  'SEASONAL_TENDER',
  'PROJECT_TENDER',
] as const

export type FreightRequestType = (typeof FREIGHT_REQUEST_TYPES)[number]

export const BID_STATUSES = [
  'DRAFT',
  'SUBMITTED',
  'ACCEPTED',
  'REJECTED',
] as const

export type BidStatus = (typeof BID_STATUSES)[number]

export interface FreightRequest {
  id: string
  tenant_id: string
  freight_request_number: string
  request_type: FreightRequestType | string
  shipper_company_id: string
  status: FreightRequestStatus | string
  response_deadline: string | null
  currency_code: string | null
  transport_order_id?: string | null
  created_at: string
  updated_at: string
  version: number
}

export interface ListFreightRequestsFilters {
  request_type?: string
  status?: string
  shipper_company_id?: string
  limit?: number
  offset?: number
}
