import type {
  FreightCostAccessorialSpendResponse,
  FreightCostCarrierPerformanceResponse,
  FreightCostDetailVM,
  FreightCostLanePerformanceResponse,
  FreightCostListQuery,
  FreightCostListResponse,
  FreightCostSummaryAggregateDTO,
  FreightCostSummaryQuery,
  FreightCostVarianceDetailDTO,
} from '~/types/freightCost'
import { ApiError } from '~/utils/apiClient'

export type FreightCostDataSourceMode = 'LIVE_API_V2_1E' | 'MOCK'

export interface FreightCostDataSource {
  readonly mode: FreightCostDataSourceMode
  listOrders(query: FreightCostListQuery): Promise<FreightCostListResponse>
  getSummary(query: FreightCostSummaryQuery): Promise<FreightCostSummaryAggregateDTO>
  getOrderDetail(transportOrderId: string, companyId: string): Promise<FreightCostDetailVM>
  getVarianceDetail(transportOrderId: string, companyId: string): Promise<FreightCostVarianceDetailDTO>
  getAccessorialSummary(query: FreightCostSummaryQuery): Promise<FreightCostAccessorialSpendResponse>
  getCarrierPerformance(query: FreightCostSummaryQuery): Promise<FreightCostCarrierPerformanceResponse>
  getLanePerformance(query: FreightCostSummaryQuery): Promise<FreightCostLanePerformanceResponse>
}

export class FreightCostLiveUnavailableError extends ApiError {
  constructor() {
    super(503, {
      code: 'FREIGHT_COST_LIVE_UNAVAILABLE',
      message: 'Freight cost public API is not available until v2.1E',
      details: { phase: 'v2.1E' },
    })
  }
}

function liveUnavailableRejection(): Promise<never> {
  return Promise.reject(new FreightCostLiveUnavailableError())
}

function toQueryParams(
  query: FreightCostListQuery | FreightCostSummaryQuery,
  companyId?: string,
): Record<string, string | number | undefined> {
  const params: Record<string, string | number | undefined> = {}
  const company = companyId ?? ('company_id' in query ? query.company_id : undefined)
  if (company) params.company_id = company
  if (query.from) params.from = query.from
  if (query.to) params.to = query.to
  if (query.date_dimension) params.date_dimension = query.date_dimension
  if (query.currency) params.currency = query.currency
  if (query.carrier_id) params.carrier_id = query.carrier_id
  if ('origin_location_code' in query && query.origin_location_code) {
    params.origin_location_code = query.origin_location_code
  }
  if ('destination_location_code' in query && query.destination_location_code) {
    params.destination_location_code = query.destination_location_code
  }
  if ('order_status' in query && query.order_status) params.order_status = query.order_status
  if ('settlement_status' in query && query.settlement_status) {
    params.settlement_status = query.settlement_status
  }
  if ('variance_state' in query && query.variance_state) params.variance_state = query.variance_state
  if ('reconciliation_state' in query && query.reconciliation_state) {
    params.reconciliation_state = query.reconciliation_state
  }
  if ('q' in query && query.q) params.q = query.q
  if ('limit' in query && query.limit !== undefined) params.limit = query.limit
  if ('offset' in query && query.offset !== undefined) params.offset = query.offset
  return params
}

export function createProductionFreightCostDataSource(): FreightCostDataSource {
  return {
    mode: 'LIVE_API_V2_1E',
    async listOrders(query) {
      const { apiGet } = useApi()
      return apiGet<FreightCostListResponse>('/api/v1/freight-costs', {
        query: toQueryParams(query),
      })
    },
    async getSummary(query) {
      const { apiGet } = useApi()
      return apiGet<FreightCostSummaryAggregateDTO>('/api/v1/freight-costs/summary', {
        query: toQueryParams(query),
      })
    },
    async getOrderDetail(transportOrderId, companyId) {
      const { apiGet } = useApi()
      return apiGet<FreightCostDetailVM>(
        `/api/v1/freight-costs/transport-orders/${encodeURIComponent(transportOrderId)}`,
        { query: toQueryParams({}, companyId) },
      )
    },
    async getVarianceDetail(transportOrderId, companyId) {
      const { apiGet } = useApi()
      return apiGet<FreightCostVarianceDetailDTO>(
        `/api/v1/freight-costs/transport-orders/${encodeURIComponent(transportOrderId)}/variance-detail`,
        { query: toQueryParams({}, companyId) },
      )
    },
    async getAccessorialSummary(query) {
      const { apiGet } = useApi()
      return apiGet<FreightCostAccessorialSpendResponse>('/api/v1/freight-costs/accessorials/summary', {
        query: toQueryParams(query),
      })
    },
    async getCarrierPerformance(query) {
      const { apiGet } = useApi()
      return apiGet<FreightCostCarrierPerformanceResponse>(
        '/api/v1/freight-costs/carriers/performance',
        { query: toQueryParams(query) },
      )
    },
    async getLanePerformance(query) {
      const { apiGet } = useApi()
      return apiGet<FreightCostLanePerformanceResponse>('/api/v1/freight-costs/lanes/performance', {
        query: toQueryParams(query),
      })
    },
  }
}

export function isFreightCostLiveUnavailableError(error: unknown): boolean {
  return error instanceof ApiError && error.code === 'FREIGHT_COST_LIVE_UNAVAILABLE'
}

export function createMockFreightCostDataSource(
  handlers: Partial<FreightCostDataSource>,
): FreightCostDataSource {
  return {
    mode: 'MOCK',
    listOrders: handlers.listOrders ?? liveUnavailableRejection,
    getSummary: handlers.getSummary ?? liveUnavailableRejection,
    getOrderDetail: handlers.getOrderDetail ?? liveUnavailableRejection,
    getVarianceDetail: handlers.getVarianceDetail ?? liveUnavailableRejection,
    getAccessorialSummary: handlers.getAccessorialSummary ?? liveUnavailableRejection,
    getCarrierPerformance: handlers.getCarrierPerformance ?? liveUnavailableRejection,
    getLanePerformance: handlers.getLanePerformance ?? liveUnavailableRejection,
  }
}

export const FREIGHT_COST_PUBLIC_API_PATHS = [
  '/api/v1/freight-costs',
  '/api/v1/freight-costs/summary',
  '/api/v1/freight-costs/variance',
  '/api/v1/freight-costs/accessorials/summary',
  '/api/v1/freight-costs/carriers/performance',
  '/api/v1/freight-costs/lanes/performance',
] as const

export const FREIGHT_COST_FORBIDDEN_BROWSER_PATHS = [
  '/internal/v1/freight-cost',
] as const

export const FREIGHT_COST_FORBIDDEN_BROWSER_HEADERS = [
  'X-Internal-Service-Token',
] as const
