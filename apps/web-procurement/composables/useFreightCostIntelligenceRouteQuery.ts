export function useFreightCostIntelligenceRouteQuery() {
  const route = useRoute()

  return computed(() => {
    const currency = typeof route.query.currency === 'string' ? route.query.currency : undefined
    const limitRaw = typeof route.query.limit === 'string' ? Number(route.query.limit) : undefined
    const offsetRaw = typeof route.query.offset === 'string' ? Number(route.query.offset) : undefined

    return {
      currency,
      limit: limitRaw !== undefined && !Number.isNaN(limitRaw) ? limitRaw : undefined,
      offset: offsetRaw !== undefined && !Number.isNaN(offsetRaw) ? offsetRaw : undefined,
    }
  })
}

export function useFreightCostIntelligenceRouteQueryWatcher(reload: () => void | Promise<void>) {
  const route = useRoute()
  const { currentCompanyId } = useTenantContext()

  watch(
    () => [
      currentCompanyId.value,
      route.query.currency,
      route.query.limit,
      route.query.offset,
    ] as const,
    () => {
      void reload()
    },
  )

  onMounted(() => {
    void reload()
  })
}
