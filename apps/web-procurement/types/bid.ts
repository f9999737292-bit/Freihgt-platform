export interface BidItem {
  id?: string
  description?: string | null
  base_amount?: number
  fuel_surcharge?: number
  toll_amount?: number
  extra_charges?: number
  amount_without_vat?: number
  vat_rate?: number | null
  vat_amount?: number
  amount_with_vat?: number
  comment?: string | null
}

export interface Bid {
  id: string
  tenant_id?: string
  freight_request_id?: string
  carrier_company_id: string
  bid_number: string
  status: string
  total_amount?: number
  currency_code?: string | null
  vat_rate?: number | null
  vat_amount?: number
  total_amount_with_vat?: number
  valid_until?: string | null
  submitted_at?: string | null
  items?: BidItem[]
  created_at?: string
  updated_at?: string
}
