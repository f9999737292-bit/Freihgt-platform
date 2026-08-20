export const PAYMENT_STATUSES = [
  'RECEIVED',
  'PARTIALLY_ALLOCATED',
  'FULLY_ALLOCATED',
  'RECONCILED',
  'VOIDED',
] as const

export type PaymentStatus = (typeof PAYMENT_STATUSES)[number]

export type PaymentActor = 'BUYER' | 'CARRIER'

export interface PaymentRecord {
  id: string
  tenant_id: string
  payment_number: string
  payer_company_id: string
  payee_company_id: string
  amount: string
  currency_code: string
  payment_date: string
  source: string
  status: PaymentStatus
  allocated_amount: string
  unallocated_amount: string
  version: number
  created_at: string
  updated_at: string
  reference?: string
  external_reference?: string
  external_id?: string
  created_by?: string
  voided_at?: string
  voided_by?: string
  void_reason?: string
  reconciled_at?: string
  reconciled_by?: string
}

export interface PaymentObligationRecord {
  id: string
  tenant_id: string
  obligation_number: string
  payer_company_id: string
  payee_company_id: string
  source_type: string
  source_id: string
  currency_code: string
  original_amount: string
  paid_amount: string
  outstanding_amount: string
  status: string
  version: number
  created_at: string
  updated_at: string
  due_date?: string
}

export interface PaymentAllocationRecord {
  id: string
  tenant_id: string
  payment_id: string
  obligation_id: string
  allocated_amount: string
  currency_code: string
  created_by: string
  created_at: string
  voided_at?: string
  voided_by?: string
  void_reason?: string
  obligation_number?: string
  obligation_status?: string
  obligation_source_type?: string
  obligation_source_id?: string
  obligation_outstanding_amount?: string
}

export interface PaymentAuditEventRecord {
  id: string
  tenant_id: string
  entity_type: string
  entity_id: string
  event_type: string
  actor_user_id?: string
  actor_company_id?: string
  payload?: Record<string, unknown>
  created_at: string
}

export interface PaymentListResponse {
  items: PaymentRecord[]
  total: number
  limit: number
  offset: number
}

export interface PaginatedListResponse<T> {
  items: T[]
  total: number
  limit: number
  offset: number
}

export interface AllocatePaymentResult {
  payment: PaymentRecord
  obligation: PaymentObligationRecord
  allocation: PaymentAllocationRecord
}

export interface VoidAllocationResult {
  allocation: PaymentAllocationRecord
  payment: PaymentRecord
  obligation: PaymentObligationRecord
}
