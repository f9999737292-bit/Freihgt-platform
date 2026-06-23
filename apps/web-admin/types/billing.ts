export interface BillingRegister {
  id: string
  tenant_id: string
  register_number: string
  customer_company_id?: string
  contractor_company_id?: string
  total_with_vat?: number
  total_without_vat?: number
  period_from?: string
  period_to?: string
  currency_code?: string
  status: string
  created_at?: string
}
