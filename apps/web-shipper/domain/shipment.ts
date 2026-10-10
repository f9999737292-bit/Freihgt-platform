/** Customer shipment fields returned by the shipper read routes. Ids stay ids. */
export interface ShipperShipment {
  id: string
  shipment_number?: string
  status?: string
  transport_order_id?: string | null
  shipper_company_id?: string
  consignee_company_id?: string
  carrier_company_id?: string | null
  forwarder_company_id?: string | null
  origin_location_id?: string
  destination_location_id?: string
  cargo_id?: string | null
  transport_mode?: string
  planned_pickup_at?: string | null
  planned_delivery_at?: string | null
  actual_pickup_at?: string | null
  actual_delivery_at?: string | null
}

export function displayFact(value: string | null | undefined): string {
  const text = value?.trim() ?? ''
  return text === '' ? '—' : text
}
