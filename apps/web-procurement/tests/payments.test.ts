import { describe, expect, it } from 'vitest'
import type { UserCompanyMembership } from '~/types/company'
import type { PaymentRecord } from '~/types/payment'
import {
  canShowAllocateAction,
  canShowReconcileAction,
  canShowVoidAllocationAction,
  canShowVoidPaymentAction,
  formatPaymentMoney,
  hasPaymentReadRole,
  hasPaymentWriteRole,
  isAllocationActive,
  isNonEmptyReason,
  isPositiveAmountInput,
  resolvePaymentActor,
} from '~/utils/payment'

function membership(
  overrides: Partial<UserCompanyMembership> & Pick<UserCompanyMembership, 'company_id' | 'legal_name'>,
): UserCompanyMembership {
  return {
    membership_id: 'm-1',
    company_type: 'SHIPPER',
    membership_status: 'ACTIVE',
    roles: [{ role_id: 'r-1', code: 'FINANCE_MANAGER', name: 'Finance' }],
    ...overrides,
  }
}

const basePayment = (status: PaymentRecord['status']): PaymentRecord => ({
  id: 'p-1',
  tenant_id: 't-1',
  payment_number: 'PAY-001',
  payer_company_id: 'buyer-1',
  payee_company_id: 'carrier-1',
  amount: '100.00',
  currency_code: 'RUB',
  payment_date: '2026-01-01',
  source: 'MANUAL',
  status,
  allocated_amount: '0.00',
  unallocated_amount: '100.00',
  version: 1,
  created_at: '2026-01-01T00:00:00Z',
  updated_at: '2026-01-01T00:00:00Z',
})

describe('payment utilities', () => {
  it('resolves buyer and carrier actors', () => {
    const buyer = membership({ company_id: 'buyer-1', legal_name: 'Buyer' })
    const carrier = membership({
      company_id: 'carrier-1',
      legal_name: 'Carrier',
      company_type: 'CARRIER',
      roles: [{ role_id: 'r-2', code: 'CARRIER_ADMIN', name: 'Carrier admin' }],
    })
    expect(resolvePaymentActor('buyer-1', [buyer, carrier])).toBe('BUYER')
    expect(resolvePaymentActor('carrier-1', [buyer, carrier])).toBe('CARRIER')
  })

  it('formats payment money for display only', () => {
    expect(formatPaymentMoney('100.50', 'RUB')).toContain('RUB')
    expect(formatPaymentMoney(null)).toBe('—')
  })

  it('gates allocate and reconcile actions by backend status', () => {
    expect(canShowAllocateAction(basePayment('RECEIVED'))).toBe(true)
    expect(canShowReconcileAction(basePayment('FULLY_ALLOCATED'))).toBe(true)
    expect(canShowVoidPaymentAction(basePayment('RECEIVED'))).toBe(true)
    expect(canShowAllocateAction(basePayment('RECONCILED'))).toBe(false)
    expect(canShowVoidAllocationAction(basePayment('RECONCILED'), { voided_at: undefined })).toBe(false)
  })

  it('detects active vs voided allocations', () => {
    expect(isAllocationActive({})).toBe(true)
    expect(isAllocationActive({ voided_at: '2026-01-02T00:00:00Z' })).toBe(false)
  })

  it('validates reason and amount inputs', () => {
    expect(isPositiveAmountInput('10.00')).toBe(true)
    expect(isPositiveAmountInput('0')).toBe(false)
    expect(isNonEmptyReason(' duplicate ')).toBe(true)
    expect(isNonEmptyReason('   ')).toBe(false)
  })

  it('maps payment RBAC roles', () => {
    expect(hasPaymentReadRole(['FINANCE_MANAGER'])).toBe(true)
    expect(hasPaymentWriteRole(['SHIPPER_LOGIST'])).toBe(false)
    expect(hasPaymentWriteRole(['CARRIER_ACCOUNTANT'])).toBe(true)
  })
})
