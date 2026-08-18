import type { UserCompanyMembership } from '~/types/company'
import {
  isBuyerMembership,
  isCarrierMembership,
} from '~/utils/companyMembership'
import type {
  FreightSettlementDetail,
  SettlementAccessorial,
  SettlementActor,
} from '~/types/settlement'

export interface SettlementMoneySummary {
  agreedBase: number
  additionalProposed: number
  additionalApproved: number
  additionalDisputed: number
  totalWithoutVat: number
  vatAmount: number
  totalWithVat: number
  currencyCode: string
}

export function resolveSettlementActor(
  companyId: string | null | undefined,
  memberships: UserCompanyMembership[],
): SettlementActor | null {
  if (!companyId?.trim()) return null
  const membership = memberships.find((item) => item.company_id === companyId)
  if (!membership) return null
  if (isCarrierMembership(membership)) return 'CARRIER'
  if (isBuyerMembership(membership)) return 'BUYER'
  return null
}

export function sumAccessorialsByStatus(
  accessorials: SettlementAccessorial[],
  status: SettlementAccessorial['status'],
): number {
  return accessorials
    .filter((item) => item.status === status)
    .reduce((sum, item) => sum + item.amount, 0)
}

export function computeSettlementMoneySummary(
  detail: Pick<
    FreightSettlementDetail,
    | 'base_freight_amount'
    | 'approved_accessorial_total'
    | 'total_without_vat'
    | 'vat_amount'
    | 'total_with_vat'
    | 'currency_code'
    | 'accessorials'
  >,
): SettlementMoneySummary {
  return {
    agreedBase: detail.base_freight_amount,
    additionalProposed: sumAccessorialsByStatus(detail.accessorials, 'PROPOSED'),
    additionalApproved: detail.approved_accessorial_total,
    additionalDisputed: sumAccessorialsByStatus(detail.accessorials, 'DISPUTED'),
    totalWithoutVat: detail.total_without_vat,
    vatAmount: detail.vat_amount,
    totalWithVat: detail.total_with_vat,
    currencyCode: detail.currency_code,
  }
}

export function filterSettlementsByRegister<T extends { billing_register_id?: string | null }>(
  items: T[],
  registerId?: string | null,
): T[] {
  if (!registerId?.trim()) return items
  return items.filter((item) => item.billing_register_id === registerId)
}

export function settlementActorQuery(actor: SettlementActor, companyId: string) {
  return {
    company_id: companyId,
    actor,
  }
}
