import type { Ref } from 'vue'
import type { FreightCostAnalyticsQuery } from '~/types/freightCost'

export function shouldApplyFreightCostIntelligenceListLoad(
  loadGeneration: number,
  activeGeneration: number,
  result: { items?: unknown[] } | null | undefined,
): boolean {
  return loadGeneration === activeGeneration && result != null
}

export function useFreightCostIntelligenceListLoad<TResponse extends { items?: unknown[] }>(
  fetcher: (query: FreightCostAnalyticsQuery & { company_id: string }) => Promise<TResponse>,
  runLoad: <T>(loader: () => Promise<T>) => Promise<T | null>,
) {
  const { currentCompanyId } = useTenantContext()
  const routeQuery = useFreightCostIntelligenceRouteQuery()
  const response = ref<TResponse | null>(null) as Ref<TResponse | null>
  let activeGeneration = 0

  async function reload() {
    if (!currentCompanyId.value) {
      return
    }

    const loadGeneration = ++activeGeneration
    const query = routeQuery.value
    const result = await runLoad(() => fetcher({
      company_id: currentCompanyId.value!,
      currency: query.currency,
      limit: query.limit,
      offset: query.offset,
    }))

    if (shouldApplyFreightCostIntelligenceListLoad(loadGeneration, activeGeneration, result)) {
      response.value = result
    }
  }

  useFreightCostIntelligenceRouteQueryWatcher(reload)

  return { response, reload }
}
