import type {
  ShipmentEventQueryFilters,
  ShipmentEventsResponse,
} from '~/types/shipmentEvent'
import { ApiError } from '~/composables/useApi'

export function useShipmentEvents(shipmentId: Ref<string> | ComputedRef<string>) {
  const route = useRoute()
  const router = useRouter()
  const { apiGet } = useApi()

  const data = ref<ShipmentEventsResponse | null>(null)
  const loading = ref(false)
  const accessDenied = ref(false)
  const notFound = ref(false)
  const apiUnavailable = ref(false)
  const apiError = ref<string | null>(null)

  const filters = computed<ShipmentEventQueryFilters>(() => ({
    type: stringQuery('type'),
    category: stringQuery('category'),
    source: stringQuery('source'),
    severity: stringQuery('severity'),
    date_from: stringQuery('date_from'),
    date_to: stringQuery('date_to'),
    derived: stringQuery('derived'),
    order: (stringQuery('order') as 'asc' | 'desc' | undefined) ?? 'desc',
    page: numberQuery('page') ?? 1,
    limit: numberQuery('limit') ?? 50,
  }))

  function stringQuery(key: string): string | undefined {
    const value = route.query[key]
    if (typeof value !== 'string' || !value.trim()) return undefined
    return value.trim()
  }

  function numberQuery(key: string): number | undefined {
    const value = route.query[key]
    if (typeof value !== 'string' || !value.trim()) return undefined
    const parsed = Number(value)
    return Number.isFinite(parsed) ? parsed : undefined
  }

  function buildQueryParams(next: ShipmentEventQueryFilters): Record<string, string | number | undefined> {
    return {
      type: next.type,
      category: next.category,
      source: next.source,
      severity: next.severity,
      date_from: next.date_from,
      date_to: next.date_to,
      derived: next.derived,
      order: next.order ?? 'desc',
      page: next.page ?? 1,
      limit: next.limit ?? 50,
    }
  }

  async function fetchEvents() {
    const id = unref(shipmentId)
    if (!id) return

    loading.value = true
    accessDenied.value = false
    notFound.value = false
    apiUnavailable.value = false
    apiError.value = null

    try {
      data.value = await apiGet<ShipmentEventsResponse>(`/api/v1/shipments/${id}/events`, {
        query: buildQueryParams(filters.value),
        skipTenant: true,
      })
    } catch (error) {
      data.value = null
      if (error instanceof ApiError) {
        if (error.status === 403) {
          accessDenied.value = true
          return
        }
        if (error.status === 404) {
          notFound.value = true
          return
        }
        if (error.status === 0 || error.status >= 500) {
          apiUnavailable.value = true
          return
        }
        apiError.value = error.message
        return
      }
      apiUnavailable.value = true
    } finally {
      loading.value = false
    }
  }

  function updateFilters(next: Partial<ShipmentEventQueryFilters>) {
    const merged = { ...filters.value, ...next, page: next.page ?? 1 }
    router.replace({
      path: route.path,
      query: Object.fromEntries(
        Object.entries(buildQueryParams(merged)).filter(([, value]) => value !== undefined && value !== ''),
      ),
    })
  }

  function resetFilters() {
    router.replace({ path: route.path, query: {} })
  }

  watch([shipmentId, () => route.query], fetchEvents, { immediate: true, deep: true })

  const isEmpty = computed(() => !loading.value && !!data.value && data.value.timeline.total === 0)
  const isDerivedOnly = computed(() => {
    if (!data.value || data.value.timeline.total === 0) return false
    return data.value.timeline.items.every((item) => item.derived)
  })
  const isPartial = computed(() => !!data.value?.dataFreshness.partial)

  return {
    data,
    loading,
    accessDenied,
    notFound,
    apiUnavailable,
    apiError,
    filters,
    isEmpty,
    isDerivedOnly,
    isPartial,
    fetchEvents,
    updateFilters,
    resetFilters,
  }
}
