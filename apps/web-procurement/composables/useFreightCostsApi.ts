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
import {
  createProductionFreightCostDataSource,
  type FreightCostDataSource,
} from '~/utils/freightCostDataSource'

/**
 * Typed adapter for the future public freight-cost gateway contract (/api/v1/freight-costs/*).
 * v2.1D: public routes are not live — production data source fail-closes until v2.1E.
 */
export function useFreightCostsApi(dataSource?: FreightCostDataSource) {
  const source = dataSource ?? createProductionFreightCostDataSource()

  async function listFreightCosts(query: FreightCostListQuery): Promise<FreightCostListResponse> {
    return source.listOrders(query)
  }

  async function getFreightCostSummary(query: FreightCostSummaryQuery): Promise<FreightCostSummaryAggregateDTO> {
    return source.getSummary(query)
  }

  async function getFreightCostOrderDetail(
    transportOrderId: string,
    companyId: string,
  ): Promise<FreightCostDetailVM> {
    return source.getOrderDetail(transportOrderId, companyId)
  }

  async function getFreightCostVarianceDetail(
    transportOrderId: string,
    companyId: string,
  ): Promise<FreightCostVarianceDetailDTO> {
    return source.getVarianceDetail(transportOrderId, companyId)
  }

  async function getFreightCostAccessorialSummary(
    query: FreightCostSummaryQuery,
  ): Promise<FreightCostAccessorialSpendResponse> {
    return source.getAccessorialSummary(query)
  }

  async function getFreightCostCarrierPerformance(
    query: FreightCostSummaryQuery,
  ): Promise<FreightCostCarrierPerformanceResponse> {
    return source.getCarrierPerformance(query)
  }

  async function getFreightCostLanePerformance(
    query: FreightCostSummaryQuery,
  ): Promise<FreightCostLanePerformanceResponse> {
    return source.getLanePerformance(query)
  }

  return {
    mode: source.mode,
    listFreightCosts,
    getFreightCostSummary,
    getFreightCostOrderDetail,
    getFreightCostVarianceDetail,
    getFreightCostAccessorialSummary,
    getFreightCostCarrierPerformance,
    getFreightCostLanePerformance,
  }
}
