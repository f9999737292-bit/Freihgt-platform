export interface BillingRegister {
  id: string
  tenant_id: string
  register_number: string
  customer_company_id?: string
  contractor_company_id?: string
  total_with_vat?: number
  status: string
  created_at?: string
}
