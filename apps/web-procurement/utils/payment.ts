import type { UserCompanyMembership } from '~/types/company'
import type { PaymentActor, PaymentRecord, PaymentStatus } from '~/types/payment'
import { isBuyerMembership, isCarrierMembership } from '~/utils/companyMembership'

const PAYMENT_WRITE_ROLES = [
  'PLATFORM_ADMIN',
  'SHIPPER_ADMIN',
  'FINANCE_MANAGER',
  'CARRIER_ADMIN',
  'CARRIER_ACCOUNTANT',
] as const

const PAYMENT_READ_ROLES = [
  ...PAYMENT_WRITE_ROLES,
  'SHIPPER_LOGIST',
  'FORWARDER_MANAGER',
] as const

export function paymentActorQuery(companyId: string) {
  return { company_id: companyId }
}

export function resolvePaymentActor(
  companyId: string | null | undefined,
  memberships: UserCompanyMembership[],
): PaymentActor | null {
  if (!companyId?.trim()) return null
  const membership = memberships.find((item) => item.company_id === companyId)
  if (!membership) return null
  if (isCarrierMembership(membership)) return 'CARRIER'
  if (isBuyerMembership(membership) || hasFinanceRole(membership)) return 'BUYER'
  return null
}

function hasFinanceRole(membership: UserCompanyMembership): boolean {
  return membership.roles.some((role) => role.code === 'FINANCE_MANAGER')
}

export function formatPaymentMoney(amount?: string | null, currency?: string | null): string {
  if (!amount) return '—'
  const numeric = Number(amount)
  if (Number.isNaN(numeric)) return amount
  const formatted = new Intl.NumberFormat('ru-RU', {
    minimumFractionDigits: 2,
    maximumFractionDigits: 2,
  }).format(numeric)
  return currency ? `${formatted} ${currency}` : formatted
}

export function isAllocationActive(allocation: { voided_at?: string | null }): boolean {
  return !allocation.voided_at
}

export function canShowAllocateAction(payment: PaymentRecord | null): boolean {
  if (!payment) return false
  return (
    payment.status === 'RECEIVED'
    || payment.status === 'PARTIALLY_ALLOCATED'
  ) && payment.unallocated_amount !== '0.00' && payment.unallocated_amount !== '0'
}

export function canShowVoidPaymentAction(payment: PaymentRecord | null): boolean {
  if (!payment) return false
  return payment.status === 'RECEIVED'
}

export function canShowReconcileAction(payment: PaymentRecord | null): boolean {
  if (!payment) return false
  return payment.status === 'FULLY_ALLOCATED'
}

export function canShowVoidAllocationAction(
  payment: PaymentRecord | null,
  allocation: { voided_at?: string | null },
): boolean {
  if (!payment || !isAllocationActive(allocation)) return false
  return payment.status !== 'RECONCILED' && payment.status !== 'VOIDED'
}

export function hasPaymentReadRole(roles: string[]): boolean {
  return roles.some((role) => PAYMENT_READ_ROLES.includes(role as (typeof PAYMENT_READ_ROLES)[number]))
}

export function hasPaymentWriteRole(roles: string[]): boolean {
  return roles.some((role) => PAYMENT_WRITE_ROLES.includes(role as (typeof PAYMENT_WRITE_ROLES)[number]))
}

export function paymentStatusLabelKey(status: PaymentStatus): string {
  return `payments.status.${status}`
}

export function isPositiveAmountInput(value: string): boolean {
  const trimmed = value.trim()
  if (!trimmed) return false
  const numeric = Number(trimmed)
  return !Number.isNaN(numeric) && numeric > 0
}

export function isNonEmptyReason(value: string): boolean {
  return value.trim().length > 0
}
