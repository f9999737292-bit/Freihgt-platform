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

export function createProductionFreightCostDataSource(): FreightCostDataSource {
  return {
    mode: 'LIVE_API_V2_1E',
    listOrders() {
      return liveUnavailableRejection()
    },
    getSummary() {
      return liveUnavailableRejection()
    },
    getOrderDetail() {
      return liveUnavailableRejection()
    },
    getVarianceDetail() {
      return liveUnavailableRejection()
    },
    getAccessorialSummary() {
      return liveUnavailableRejection()
    },
    getCarrierPerformance() {
      return liveUnavailableRejection()
    },
    getLanePerformance() {
      return liveUnavailableRejection()
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
