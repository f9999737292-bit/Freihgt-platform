export type SettlementActor = 'BUYER' | 'CARRIER'

export type SettlementStatus =
  | 'DRAFT'
  | 'UNDER_REVIEW'
  | 'DISPUTED'
  | 'APPROVED'
  | 'DOCUMENTS_READY'
  | 'READY_FOR_PAYMENT'
  | 'CANCELLED'

export type AccessorialStatus = 'PROPOSED' | 'APPROVED' | 'REJECTED' | 'DISPUTED'

export type DisputeStatus = 'OPEN' | 'RESOLVED' | 'WITHDRAWN'

export interface FreightSettlement {
  id: string
  tenant_id: string
  shipment_id: string
  transport_order_id: string
  buyer_company_id: string
  carrier_company_id: string
  award_link_id?: string | null
  settlement_number: string
  base_freight_amount: number
  currency_code: string
  vat_rate?: number | null
  approved_accessorial_total: number
  total_without_vat: number
  vat_amount: number
  total_with_vat: number
  status: SettlementStatus
  service_accepted_at?: string | null
  service_accepted_by?: string | null
  billing_register_id?: string | null
  billing_register_item_id?: string | null
  version: number
  created_at: string
  updated_at: string
}

export interface SettlementAccessorial {
  id: string
  settlement_id: string
  charge_code: string
  description?: string | null
  amount: number
  currency_code: string
  status: AccessorialStatus
  submitted_by: string
  submitted_by_company_id: string
  evidence_document_id?: string | null
  evidence_type?: string | null
  created_at: string
  updated_at: string
}

export interface SettlementDispute {
  id: string
  settlement_id: string
  accessorial_id?: string | null
  reason: string
  raised_by: string
  raised_by_company_id: string
  status: DisputeStatus
  resolution_note?: string | null
  resolved_by?: string | null
  resolved_at?: string | null
  created_at: string
  updated_at: string
}

export interface SettlementReconciliation {
  base_freight_amount: number
  approved_accessorial_total: number
  settlement_total_without_vat: number
  settlement_total_with_vat: number
  currency_code: string
}

export interface FreightSettlementDetail extends FreightSettlement {
  accessorials: SettlementAccessorial[]
  disputes: SettlementDispute[]
  reconciliation: SettlementReconciliation
  eligible_for_billing?: boolean
  billing_block_reason?: string
}

export interface BillingRegisterItem {
  id: string
  register_id: string
  settlement_id?: string | null
  shipment_id: string
  transport_order_id?: string | null
  base_amount: number
  extra_charges: number
  penalties: number
  amount_without_vat: number
  vat_rate?: number | null
  vat_amount: number
  amount_with_vat: number
  status: string
  created_at: string
}

export interface BillingRegisterDetail extends BillingRegister {
  items: BillingRegisterItem[]
  closing_document_packages?: Array<{ id: string; package_number: string; status: string }>
  invoices?: Array<{ id: string; invoice_number: string; total_amount: number; status: string }>
  acts?: Array<{ id: string; act_number: string; total_amount: number; status: string }>
  vat_invoices?: Array<{ id: string; vat_invoice_number: string; amount_with_vat: number; status: string }>
  upd_documents?: Array<{ id: string; upd_number: string; amount_with_vat: number; status: string }>
}

export interface SettlementListResponse {
  items: FreightSettlement[]
  total: number
}

export interface BillingRegister {
  id: string
  tenant_id: string
  register_number: string
  customer_company_id: string
  contractor_company_id: string
  contract_id?: string | null
  period_from: string
  period_to: string
  currency_code: string
  vat_rate?: number | null
  status: string
  total_without_vat: number
  vat_amount: number
  total_with_vat: number
  created_at: string
  approved_at?: string | null
  approved_by?: string | null
  updated_at: string
  version: number
}

export interface BillingRegisterListResponse {
  items: BillingRegister[]
  total: number
}

export interface CreateSettlementRequest {
  shipment_id: string
  idempotency_key?: string
  settlement_number?: string
}

export interface ProposeAccessorialRequest {
  charge_code: string
  description?: string
  amount: number
  evidence_document_id?: string
  evidence_type?: string
}

export interface RaiseDisputeRequest {
  accessorial_id?: string
  reason: string
}

export interface ResolveDisputeRequest {
  resolution_note: string
}

export interface IncludeRegisterRequest {
  register_number: string
}
