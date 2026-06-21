export interface TransportOrder {
  id: string
  tenant_id: string
  order_number: string
  status: string
  shipper_company_id?: string
  consignee_company_id?: string
  pickup_date?: string
  delivery_date?: string
  requested_pickup_date?: string
  requested_delivery_date?: string
  equipment_type?: string
  created_at?: string
  updated_at?: string
}
