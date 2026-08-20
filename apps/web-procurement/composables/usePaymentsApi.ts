import type {
  AllocatePaymentResult,
  PaginatedListResponse,
  PaymentAllocationRecord,
  PaymentAuditEventRecord,
  PaymentListResponse,
  PaymentObligationRecord,
  PaymentRecord,
  VoidAllocationResult,
} from '~/types/payment'
import { paymentActorQuery } from '~/utils/payment'
import { PAYMENT_DETAIL_PAGE_SIZE } from '~/utils/paymentWorkspaceFlow'

function requireCompanyId(companyId: string | null | undefined): string {
  const value = companyId?.trim()
  if (!value) throw new Error('Company ID is required')
  return value
}

function normalizePaginated<T>(
  data: PaginatedListResponse<T>,
  fallbackLimit: number,
  fallbackOffset: number,
): PaginatedListResponse<T> {
  return {
    items: data.items ?? [],
    total: data.total ?? data.items?.length ?? 0,
    limit: data.limit ?? fallbackLimit,
    offset: data.offset ?? fallbackOffset,
  }
}

export function usePaymentsApi() {
  const { apiGet, apiPost } = useApi()
  const tenantStore = useTenantStore()

  function query() {
    return paymentActorQuery(requireCompanyId(tenantStore.currentCompanyId))
  }

  async function listPayments(options: {
    status?: string
    currency_code?: string
    from_date?: string
    to_date?: string
    q?: string
    limit?: number
    offset?: number
  } = {}) {
    const params: Record<string, string | number | undefined> = {
      ...query(),
      limit: options.limit ?? 20,
      offset: options.offset ?? 0,
    }
    if (options.status) params.status = options.status
    if (options.currency_code) params.currency_code = options.currency_code
    if (options.from_date) params.from_date = options.from_date
    if (options.to_date) params.to_date = options.to_date
    if (options.q) params.q = options.q
    const data = await apiGet<PaymentListResponse>('/api/v1/payments', { query: params })
    return normalizePaginated(data, params.limit as number, params.offset as number)
  }

  async function getPayment(id: string) {
    return apiGet<PaymentRecord>(`/api/v1/payments/${encodeURIComponent(id)}`, { query: query() })
  }

  async function listAllocations(
    paymentId: string,
    options: { limit?: number; offset?: number } = {},
  ) {
    const limit = options.limit ?? PAYMENT_DETAIL_PAGE_SIZE
    const offset = options.offset ?? 0
    const data = await apiGet<PaginatedListResponse<PaymentAllocationRecord>>(
      `/api/v1/payments/${encodeURIComponent(paymentId)}/allocations`,
      { query: { ...query(), limit, offset } },
    )
    return normalizePaginated(data, limit, offset)
  }

  async function listEligibleObligations(
    paymentId: string,
    options: { limit?: number; offset?: number } = {},
  ) {
    const limit = options.limit ?? PAYMENT_DETAIL_PAGE_SIZE
    const offset = options.offset ?? 0
    const data = await apiGet<PaginatedListResponse<PaymentObligationRecord>>(
      `/api/v1/payments/${encodeURIComponent(paymentId)}/eligible-obligations`,
      { query: { ...query(), limit, offset } },
    )
    return normalizePaginated(data, limit, offset)
  }

  async function listAuditEvents(
    paymentId: string,
    options: { limit?: number; offset?: number } = {},
  ) {
    const limit = options.limit ?? PAYMENT_DETAIL_PAGE_SIZE
    const offset = options.offset ?? 0
    const data = await apiGet<PaginatedListResponse<PaymentAuditEventRecord>>(
      `/api/v1/payments/${encodeURIComponent(paymentId)}/audit-events`,
      { query: { ...query(), limit, offset } },
    )
    return normalizePaginated(data, limit, offset)
  }

  async function allocate(paymentId: string, obligationId: string, allocatedAmount: string) {
    return apiPost<AllocatePaymentResult>(
      `/api/v1/payments/${encodeURIComponent(paymentId)}/allocations`,
      { obligation_id: obligationId, allocated_amount: allocatedAmount },
      { query: query() },
    )
  }

  async function voidAllocation(allocationId: string, reason: string) {
    return apiPost<VoidAllocationResult>(
      `/api/v1/payment-allocations/${encodeURIComponent(allocationId)}/void`,
      { reason },
      { query: query() },
    )
  }

  async function voidPayment(paymentId: string, reason: string) {
    return apiPost<PaymentRecord>(
      `/api/v1/payments/${encodeURIComponent(paymentId)}/void`,
      { reason },
      { query: query() },
    )
  }

  async function reconcilePayment(paymentId: string) {
    return apiPost<PaymentRecord>(
      `/api/v1/payments/${encodeURIComponent(paymentId)}/reconcile`,
      {},
      { query: query() },
    )
  }

  return {
    listPayments,
    getPayment,
    listAllocations,
    listEligibleObligations,
    listAuditEvents,
    allocate,
    voidAllocation,
    voidPayment,
    reconcilePayment,
  }
}
