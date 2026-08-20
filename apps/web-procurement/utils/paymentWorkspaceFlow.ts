import type {
  PaginatedListResponse,
  PaymentAllocationRecord,
  PaymentAuditEventRecord,
  PaymentObligationRecord,
  PaymentRecord,
} from '~/types/payment'

export const PAYMENT_DETAIL_PAGE_SIZE = 20

export function hasMorePages<T>(page: PaginatedListResponse<T>): boolean {
  return page.offset + page.items.length < page.total
}

export function appendPageItems<T>(existing: T[], page: PaginatedListResponse<T>): T[] {
  return [...existing, ...page.items]
}

export function obligationLabelFromAllocation(allocation: PaymentAllocationRecord): string {
  if (allocation.obligation_number) return allocation.obligation_number
  return `${allocation.obligation_id.slice(0, 8)}…`
}

export function billingRegisterLinkFromAllocation(allocation: PaymentAllocationRecord): string | null {
  if (allocation.obligation_source_type === 'BILLING_REGISTER' && allocation.obligation_source_id) {
    return `/billing-registers/${allocation.obligation_source_id}`
  }
  return null
}

export interface PaymentDetailApi {
  getPayment(id: string): Promise<PaymentRecord>
  listAllocations(
    paymentId: string,
    options?: { limit?: number; offset?: number },
  ): Promise<PaginatedListResponse<PaymentAllocationRecord>>
  listAuditEvents(
    paymentId: string,
    options?: { limit?: number; offset?: number },
  ): Promise<PaginatedListResponse<PaymentAuditEventRecord>>
  listEligibleObligations(
    paymentId: string,
    options?: { limit?: number; offset?: number },
  ): Promise<PaginatedListResponse<PaymentObligationRecord>>
}

export interface PaymentDetailSnapshot {
  payment: PaymentRecord
  allocations: PaymentAllocationRecord[]
  allocationsTotal: number
  auditEvents: PaymentAuditEventRecord[]
  auditTotal: number
  eligibleObligations: PaymentObligationRecord[]
  eligibleTotal: number
}

export async function fetchPaymentDetailInitial(
  api: PaymentDetailApi,
  paymentId: string,
  includeEligible: boolean,
): Promise<PaymentDetailSnapshot> {
  const pageSize = PAYMENT_DETAIL_PAGE_SIZE
  const [payment, allocationsPage, auditPage, eligiblePage] = await Promise.all([
    api.getPayment(paymentId),
    api.listAllocations(paymentId, { limit: pageSize, offset: 0 }),
    api.listAuditEvents(paymentId, { limit: pageSize, offset: 0 }),
    includeEligible
      ? api.listEligibleObligations(paymentId, { limit: pageSize, offset: 0 })
      : Promise.resolve({ items: [], total: 0, limit: pageSize, offset: 0 }),
  ])
  return {
    payment,
    allocations: allocationsPage.items,
    allocationsTotal: allocationsPage.total,
    auditEvents: auditPage.items,
    auditTotal: auditPage.total,
    eligibleObligations: eligiblePage.items,
    eligibleTotal: eligiblePage.total,
  }
}

export async function fetchAllocationsPage(
  api: PaymentDetailApi,
  paymentId: string,
  offset: number,
  limit = PAYMENT_DETAIL_PAGE_SIZE,
): Promise<PaginatedListResponse<PaymentAllocationRecord>> {
  return api.listAllocations(paymentId, { limit, offset })
}

export async function fetchAuditPage(
  api: PaymentDetailApi,
  paymentId: string,
  offset: number,
  limit = PAYMENT_DETAIL_PAGE_SIZE,
): Promise<PaginatedListResponse<PaymentAuditEventRecord>> {
  return api.listAuditEvents(paymentId, { limit, offset })
}

export async function fetchEligiblePage(
  api: PaymentDetailApi,
  paymentId: string,
  offset: number,
  limit = PAYMENT_DETAIL_PAGE_SIZE,
): Promise<PaginatedListResponse<PaymentObligationRecord>> {
  return api.listEligibleObligations(paymentId, { limit, offset })
}

export interface PaymentListApi {
  listPayments(options: {
    status?: string
    currency_code?: string
    from_date?: string
    to_date?: string
    q?: string
    limit?: number
    offset?: number
  }): Promise<PaginatedListResponse<PaymentRecord>>
}

export interface PaymentListFilters {
  q: string
  status: string
  currency_code: string
  from_date: string
  to_date: string
}

export interface PaymentListPagination {
  limit: number
  offset: number
}

export function buildListPaymentsParams(
  filters: PaymentListFilters,
  pagination: PaymentListPagination,
) {
  return {
    q: filters.q || undefined,
    status: filters.status || undefined,
    currency_code: filters.currency_code || undefined,
    from_date: filters.from_date || undefined,
    to_date: filters.to_date || undefined,
    limit: pagination.limit,
    offset: pagination.offset,
  }
}

export async function fetchPaymentListPage(
  api: PaymentListApi,
  filters: PaymentListFilters,
  pagination: PaymentListPagination,
) {
  return api.listPayments(buildListPaymentsParams(filters, pagination))
}

export interface MutationRunner {
  acting: boolean
  run<T>(action: () => Promise<T>): Promise<T | undefined>
}

export function createMutationRunner(
  getActing: () => boolean,
  setActing: (value: boolean) => void,
  onSuccess: () => Promise<void>,
  onConflict: () => Promise<void>,
): MutationRunner {
  return {
    get acting() {
      return getActing()
    },
    async run<T>(action: () => Promise<T>): Promise<T | undefined> {
      if (getActing()) return undefined
      setActing(true)
      try {
        const result = await action()
        await onSuccess()
        return result
      } catch (error) {
        if (isConflictError(error)) {
          await onConflict()
        }
        throw error
      } finally {
        setActing(false)
      }
    },
  }
}

export function isConflictError(error: unknown): boolean {
  return typeof error === 'object'
    && error !== null
    && 'status' in error
    && (error as { status: number }).status === 409
}

export function shouldUseNeutralNotFound(status: number): boolean {
  return status === 403 || status === 404
}
