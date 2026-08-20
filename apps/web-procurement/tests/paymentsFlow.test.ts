import { describe, expect, it, vi } from 'vitest'
import type {
  PaymentAllocationRecord,
  PaymentAuditEventRecord,
  PaymentObligationRecord,
  PaymentRecord,
} from '~/types/payment'
import type { PaymentDetailApi } from '~/utils/paymentWorkspaceFlow'
import {
  appendPageItems,
  billingRegisterLinkFromAllocation,
  buildListPaymentsParams,
  createMutationRunner,
  fetchAllocationsPage,
  fetchAuditPage,
  fetchEligiblePage,
  fetchPaymentDetailInitial,
  fetchPaymentListPage,
  hasMorePages,
  obligationLabelFromAllocation,
  PAYMENT_DETAIL_PAGE_SIZE,
  shouldUseNeutralNotFound,
} from '~/utils/paymentWorkspaceFlow'
import {
  canShowAllocateAction,
  canShowReconcileAction,
  hasPaymentReadRole,
  hasPaymentWriteRole,
} from '~/utils/payment'

const payment: PaymentRecord = {
  id: 'pay-1',
  tenant_id: 't-1',
  payment_number: 'PAY-001',
  payer_company_id: 'buyer-1',
  payee_company_id: 'carrier-1',
  amount: '100.00',
  currency_code: 'RUB',
  payment_date: '2026-01-01',
  source: 'MANUAL',
  status: 'RECEIVED',
  allocated_amount: '0.00',
  unallocated_amount: '100.00',
  version: 1,
  created_at: '2026-01-01T00:00:00Z',
  updated_at: '2026-01-01T00:00:00Z',
}

const enrichedAllocation = (id: string, obligationNumber: string): PaymentAllocationRecord => ({
  id,
  tenant_id: 't-1',
  payment_id: 'pay-1',
  obligation_id: `obl-${id}`,
  allocated_amount: '10.00',
  currency_code: 'RUB',
  created_by: 'user-1',
  created_at: '2026-01-01T00:00:00Z',
  obligation_number: obligationNumber,
  obligation_status: 'OPEN',
  obligation_source_type: 'BILLING_REGISTER',
  obligation_source_id: `reg-${id}`,
  obligation_outstanding_amount: '90.00',
})

describe('payment workspace flow helpers', () => {
  it('builds list params and detects pagination', () => {
    const params = buildListPaymentsParams(
      { q: 'PAY', status: 'RECEIVED', currency_code: 'RUB', from_date: '', to_date: '' },
      { limit: 20, offset: 20 },
    )
    expect(params).toEqual({ q: 'PAY', status: 'RECEIVED', currency_code: 'RUB', limit: 20, offset: 20 })
    expect(hasMorePages({ items: [{ ...payment }], total: 40, limit: 20, offset: 0 })).toBe(true)
    expect(hasMorePages({ items: [{ ...payment }], total: 1, limit: 20, offset: 0 })).toBe(false)
  })

  it('loads payment list page from api', async () => {
    const listPayments = vi.fn().mockResolvedValue({ items: [payment], total: 1, limit: 20, offset: 0 })
    const result = await fetchPaymentListPage({ listPayments }, {
      q: '', status: '', currency_code: '', from_date: '', to_date: '',
    }, { limit: 20, offset: 0 })
    expect(result.items).toHaveLength(1)
    expect(listPayments).toHaveBeenCalledWith({ limit: 20, offset: 0 })
  })

  it('loads payment detail without obligation lookups', async () => {
    const getObligation = vi.fn()
    const api = {
      getPayment: vi.fn().mockResolvedValue(payment),
      listAllocations: vi.fn().mockResolvedValue({
        items: [enrichedAllocation('1', 'OBL-001')],
        total: 1,
        limit: PAYMENT_DETAIL_PAGE_SIZE,
        offset: 0,
      }),
      listAuditEvents: vi.fn().mockResolvedValue({ items: [], total: 0, limit: 20, offset: 0 }),
      listEligibleObligations: vi.fn().mockResolvedValue({ items: [], total: 0, limit: 20, offset: 0 }),
      getObligation,
    }
    const snapshot = await fetchPaymentDetailInitial(api, 'pay-1', false)
    expect(snapshot.payment.payment_number).toBe('PAY-001')
    expect(getObligation).not.toHaveBeenCalled()
    expect(obligationLabelFromAllocation(snapshot.allocations[0]!)).toBe('OBL-001')
    expect(billingRegisterLinkFromAllocation(snapshot.allocations[0]!)).toBe('/billing-registers/reg-1')
  })

  it('appends allocation pages with correct offset', async () => {
    const page1Items = Array.from({ length: 20 }, (_, i) => enrichedAllocation(String(i), `OBL-${i}`))
    const api = {
      listAllocations: vi.fn().mockResolvedValue({
        items: [enrichedAllocation('20', 'OBL-20')],
        total: 21,
        limit: 20,
        offset: 20,
      }),
    }
    const page = await fetchAllocationsPage(api as unknown as PaymentDetailApi, 'pay-1', page1Items.length)
    expect(api.listAllocations).toHaveBeenCalledWith('pay-1', { limit: 20, offset: 20 })
    const merged = appendPageItems(page1Items, page)
    expect(merged).toHaveLength(21)
    expect(hasMorePages({ items: merged, total: 21, limit: 20, offset: 0 })).toBe(false)
  })

  it('loads audit and eligible next pages', async () => {
    const auditApi = {
      listAuditEvents: vi.fn().mockResolvedValue({
        items: [{ id: 'a2', tenant_id: 't-1', entity_type: 'PAYMENT', entity_id: 'pay-1', event_type: 'payment.created', created_at: '2026-01-02T00:00:00Z' } satisfies PaymentAuditEventRecord],
        total: 2,
        limit: 20,
        offset: 1,
      }),
    }
    const auditPage = await fetchAuditPage(auditApi as unknown as PaymentDetailApi, 'pay-1', 1)
    expect(auditPage.items).toHaveLength(1)

    const eligibleApi = {
      listEligibleObligations: vi.fn().mockResolvedValue({
        items: [{ id: 'o2', tenant_id: 't-1', obligation_number: 'OBL-2', payer_company_id: 'b', payee_company_id: 'c', source_type: 'BILLING_REGISTER', source_id: 'r2', currency_code: 'RUB', original_amount: '10', paid_amount: '0', outstanding_amount: '10', status: 'OPEN', version: 1, created_at: '', updated_at: '' } satisfies PaymentObligationRecord],
        total: 2,
        limit: 20,
        offset: 1,
      }),
    }
    const eligiblePage = await fetchEligiblePage(eligibleApi as unknown as PaymentDetailApi, 'pay-1', 1)
    expect(eligiblePage.items[0]?.obligation_number).toBe('OBL-2')
  })

  it('refetches after successful mutation and on conflict', async () => {
    const onSuccess = vi.fn()
    const onConflict = vi.fn()
    let acting = false
    const runner = createMutationRunner(() => acting, (v) => { acting = v }, onSuccess, onConflict)

    await runner.run(async () => 'ok')
    expect(onSuccess).toHaveBeenCalledTimes(1)

    await expect(runner.run(async () => {
      throw { status: 409, message: 'conflict' }
    })).rejects.toEqual({ status: 409, message: 'conflict' })
    expect(onConflict).toHaveBeenCalledTimes(1)
  })

  it('ignores second mutation while acting', async () => {
    let acting = false
    let runs = 0
    const runner = createMutationRunner(() => acting, (v) => { acting = v }, async () => {}, async () => {})
    acting = true
    const result = await runner.run(async () => { runs += 1 })
    expect(result).toBeUndefined()
    expect(runs).toBe(0)
  })

  it('uses neutral not-found path for 403/404', () => {
    expect(shouldUseNeutralNotFound(404)).toBe(true)
    expect(shouldUseNeutralNotFound(403)).toBe(true)
    expect(shouldUseNeutralNotFound(500)).toBe(false)
  })

  it('gates RBAC actions by role and payment status', () => {
    expect(hasPaymentReadRole(['FINANCE_MANAGER'])).toBe(true)
    expect(hasPaymentWriteRole(['SHIPPER_LOGIST'])).toBe(false)
    expect(hasPaymentWriteRole(['CARRIER_ACCOUNTANT'])).toBe(true)
    expect(canShowAllocateAction(payment)).toBe(true)
    expect(canShowReconcileAction({ ...payment, status: 'FULLY_ALLOCATED' })).toBe(true)
    expect(canShowAllocateAction({ ...payment, status: 'RECONCILED' })).toBe(false)
  })
})
