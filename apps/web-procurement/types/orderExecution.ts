export interface OrderExecutionReadiness {
  carrier_accepted: boolean
  driver_assigned: boolean
  vehicle_assigned: boolean
  ready_to_start: boolean
  missing_requirements: string[]
}

export interface OrderExecutionProvenance {
  rfx_event_id: string
  rfx_award_id: string
  rfx_response_id: string
  rfx_lot_id?: string
  amount: number
  currency_code: string
}

export interface ExecutionMilestone {
  id: string
  shipment_version: number
  from_status?: string
  to_status: string
  source: string
  actor_type: string
  occurred_at: string
  recorded_at: string
  reason_code?: string
}

export interface ExecutionSLASignal {
  code: string
  severity: string
  message: string
  at?: string
}

export interface ExecutionPODDocument {
  id: string
  document_number: string
  status: string
  created_at: string
}

export interface OrderExecutionShipment {
  id: string
  shipment_number: string
  status: string
  driver_id?: string
  vehicle_id?: string
  planned_pickup_at?: string
  planned_delivery_at?: string
  actual_pickup_at?: string
  actual_delivery_at?: string
}

export interface OrderExecutionView {
  transport_order_id: string
  transport_order_number: string
  transport_order_status: string
  carrier_company_id: string
  buyer_company_id: string
  rfx_lot_id?: string
  readiness: OrderExecutionReadiness
  provenance: OrderExecutionProvenance
  shipment?: OrderExecutionShipment
  milestones?: ExecutionMilestone[]
  sla_signals?: ExecutionSLASignal[]
  pod_documents?: ExecutionPODDocument[]
}

export interface BuyerTransportOrderItem {
  transport_order_id: string
  transport_order_number: string
  transport_order_status: string
  rfx_event_id: string
  rfx_lot_id?: string
  carrier_company_id: string
  buyer_company_id: string
  amount: number
  currency_code: string
  converted_at: string
  shipment_id?: string
  shipment_status?: string
}

export interface CarrierTransportOrderItem {
  transport_order_id: string
  transport_order_number: string
  transport_order_status: string
  rfx_event_id: string
  rfx_lot_id?: string
  carrier_company_id: string
  buyer_company_id: string
  amount: number
  currency_code: string
  converted_at: string
  shipment_id?: string
  shipment_status?: string
}

export interface ExecuteTransportOrderResult {
  created: boolean
  transport_order_id: string
  transport_order_number: string
  transport_order_status: string
  shipment?: OrderExecutionView['shipment']
}
