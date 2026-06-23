export interface TransportOrder {
  id: string
  tenant_id: string
  order_number: string
  status: string
  shipper_company_id?: string
  consignee_company_id?: string
  origin_location_id?: string | null
  destination_location_id?: string | null
  cargo_id?: string | null
  pickup_date?: string
  delivery_date?: string
  requested_pickup_date?: string
  requested_delivery_date?: string
  equipment_type?: string
  transport_mode?: string | null
  created_at?: string
  updated_at?: string
}
