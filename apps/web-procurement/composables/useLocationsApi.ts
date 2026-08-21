import type { LocationSummary } from '~/types/contractRate'
import type { PaginatedResponse } from '~/types/api'

export function useLocationsApi() {
  const { apiGet } = useApi()

  async function listLocations(filters: { q?: string; limit?: number; offset?: number } = {}) {
    const query: Record<string, string | number | undefined> = {
      limit: filters.limit ?? 100,
      offset: filters.offset ?? 0,
    }
    if (filters.q?.trim()) query.q = filters.q.trim()
    const data = await apiGet<PaginatedResponse<LocationSummary>>('/api/v1/locations', { query })
    return { ...data, items: data.items ?? [] }
  }

  async function getLocation(id: string) {
    return apiGet<LocationSummary>(`/api/v1/locations/${encodeURIComponent(id)}`)
  }

  return { listLocations, getLocation }
}
