import { describe, expect, it } from 'vitest'
import type { UserCompanyMembership } from '~/types/company'
import type { FreightSettlementDetail } from '~/types/settlement'
import {
  computeSettlementMoneySummary,
  filterSettlementsByRegister,
  resolveSettlementActor,
  settlementActorQuery,
  sumAccessorialsByStatus,
} from '~/utils/settlement'

function membership(
  overrides: Partial<UserCompanyMembership> & Pick<UserCompanyMembership, 'company_id' | 'legal_name'>,
): UserCompanyMembership {
  return {
    membership_id: 'm-1',
    company_type: 'SHIPPER',
    membership_status: 'ACTIVE',
    roles: [{ role_id: 'r-1', code: 'PROCUREMENT_MANAGER', name: 'Procurement' }],
    ...overrides,
  }
}

describe('settlement utilities', () => {
  it('resolves buyer and carrier actors from membership', () => {
    const buyer = membership({ company_id: 'buyer-1', legal_name: 'Buyer' })
    const carrier = membership({
      company_id: 'carrier-1',
      legal_name: 'Carrier',
      company_type: 'CARRIER',
      roles: [{ role_id: 'r-2', code: 'CARRIER_ADMIN', name: 'Carrier admin' }],
    })

    expect(resolveSettlementActor('buyer-1', [buyer, carrier])).toBe('BUYER')
    expect(resolveSettlementActor('carrier-1', [buyer, carrier])).toBe('CARRIER')
    expect(resolveSettlementActor('unknown', [buyer, carrier])).toBeNull()
  })

  it('builds company context query params for API calls', () => {
    expect(settlementActorQuery('company-1')).toEqual({
      company_id: 'company-1',
    })
  })

  it('sums accessorials by status', () => {
    const accessorials = [
      { amount: 100, status: 'PROPOSED' as const },
      { amount: 50, status: 'PROPOSED' as const },
      { amount: 200, status: 'DISPUTED' as const },
    ]
    expect(sumAccessorialsByStatus(accessorials as never, 'PROPOSED')).toBe(150)
    expect(sumAccessorialsByStatus(accessorials as never, 'DISPUTED')).toBe(200)
  })

  it('computes settlement money summary with distinct labels', () => {
    const detail: FreightSettlementDetail = {
      id: 's-1',
      tenant_id: 't-1',
      shipment_id: 'sh-1',
      transport_order_id: 'to-1',
      buyer_company_id: 'b-1',
      carrier_company_id: 'c-1',
      settlement_number: 'FS-001',
      base_freight_amount: 1000,
      currency_code: 'RUB',
      approved_accessorial_total: 300,
      total_without_vat: 1300,
      vat_amount: 260,
      total_with_vat: 1560,
      status: 'UNDER_REVIEW',
      version: 1,
      created_at: '2026-01-01T00:00:00Z',
      updated_at: '2026-01-01T00:00:00Z',
      accessorials: [
        {
          id: 'a-1',
          settlement_id: 's-1',
          charge_code: 'DETENTION',
          amount: 100,
          currency_code: 'RUB',
          status: 'PROPOSED',
          submitted_by: 'u-1',
          submitted_by_company_id: 'c-1',
          created_at: '2026-01-01T00:00:00Z',
          updated_at: '2026-01-01T00:00:00Z',
        },
        {
          id: 'a-2',
          settlement_id: 's-1',
          charge_code: 'FUEL',
          amount: 75,
          currency_code: 'RUB',
          status: 'DISPUTED',
          submitted_by: 'u-1',
          submitted_by_company_id: 'c-1',
          created_at: '2026-01-01T00:00:00Z',
          updated_at: '2026-01-01T00:00:00Z',
        },
      ],
      disputes: [],
      reconciliation: {
        base_freight_amount: 1000,
        approved_accessorial_total: 300,
        settlement_total_without_vat: 1300,
        settlement_total_with_vat: 1560,
        currency_code: 'RUB',
      },
    }

    const summary = computeSettlementMoneySummary(detail)
    expect(summary.agreedBase).toBe(1000)
    expect(summary.additionalProposed).toBe(100)
    expect(summary.additionalApproved).toBe(300)
    expect(summary.additionalDisputed).toBe(75)
    expect(summary.totalWithVat).toBe(1560)
  })

  it('filters settlements by billing register id', () => {
    const items = [
      { id: '1', billing_register_id: 'reg-a' },
      { id: '2', billing_register_id: null },
      { id: '3', billing_register_id: 'reg-a' },
    ]
    expect(filterSettlementsByRegister(items, 'reg-a')).toHaveLength(2)
    expect(filterSettlementsByRegister(items)).toHaveLength(3)
  })
})
