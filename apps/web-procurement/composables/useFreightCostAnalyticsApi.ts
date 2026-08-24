import type {
  FreightCostAnalyticsAccessorialsResponse,
  FreightCostAnalyticsCarriersResponse,
  FreightCostAnalyticsLanesResponse,
  FreightCostAnalyticsOpportunitiesResponse,
  FreightCostAnalyticsOverviewDTO,
  FreightCostAnalyticsQuery,
} from '~/types/freightCost'
import {
  createProductionFreightCostDataSource,
  type FreightCostDataSource,
} from '~/utils/freightCostDataSource'

/**
 * Typed adapter for freight cost intelligence analytics (/api/v1/freight-costs/analytics/*).
 * Displays backend-computed benchmarks and opportunities only — no client-side aggregation.
 */
export function useFreightCostAnalyticsApi(dataSource?: FreightCostDataSource) {
  const { apiGet } = useApi()
  const source = dataSource ?? createProductionFreightCostDataSource(apiGet)

  async function getFreightCostAnalyticsOverview(
    query: FreightCostAnalyticsQuery,
  ): Promise<FreightCostAnalyticsOverviewDTO> {
    return source.getAnalyticsOverview(query)
  }

  async function getFreightCostAnalyticsLanes(
    query: FreightCostAnalyticsQuery,
  ): Promise<FreightCostAnalyticsLanesResponse> {
    return source.getAnalyticsLanes(query)
  }

  async function getFreightCostAnalyticsCarriers(
    query: FreightCostAnalyticsQuery,
  ): Promise<FreightCostAnalyticsCarriersResponse> {
    return source.getAnalyticsCarriers(query)
  }

  async function getFreightCostAnalyticsAccessorials(
    query: FreightCostAnalyticsQuery,
  ): Promise<FreightCostAnalyticsAccessorialsResponse> {
    return source.getAnalyticsAccessorials(query)
  }

  async function getFreightCostAnalyticsOpportunities(
    query: FreightCostAnalyticsQuery,
  ): Promise<FreightCostAnalyticsOpportunitiesResponse> {
    return source.getAnalyticsOpportunities(query)
  }

  return {
    mode: source.mode,
    getFreightCostAnalyticsOverview,
    getFreightCostAnalyticsLanes,
    getFreightCostAnalyticsCarriers,
    getFreightCostAnalyticsAccessorials,
    getFreightCostAnalyticsOpportunities,
  }
}
