<script setup lang="ts">
definePageMeta({ middleware: 'auth', layout: 'default' })

const route = useRoute()
const router = useRouter()
const { canAccessControlTower } = usePermissions()
const { backendOnline, checkBackendStatus } = useBackendStatus()

const {
  loading,
  lastUpdatedAt,
  demoMode,
  autoRefreshEnabled,
  autoRefreshIntervalMs,
  filters,
  filteredShipments,
  controlTowerShipments,
  kpiMetrics,
  criticalEvents,
  apiUnavailable,
  shipperCompanies,
  carrierCompanies,
  resetFilters,
  setAutoRefresh,
  parseFiltersFromQuery,
  filtersToQuery,
  loadData,
} = useControlTower()

const hasAccess = computed(() => canAccessControlTower())

const isEmptyDataset = computed(
  () => !loading.value && !demoMode.value && controlTowerShipments.value.length === 0,
)

const isEmptyFiltered = computed(
  () =>
    !loading.value &&
    controlTowerShipments.value.length > 0 &&
    filteredShipments.value.length === 0,
)

async function refreshAll() {
  await checkBackendStatus()
  await loadData()
}

function syncRouteFromFilters() {
  router.replace({ query: filtersToQuery() })
}

function onFiltersChange() {
  syncRouteFromFilters()
}

function onResetFilters() {
  resetFilters()
  syncRouteFromFilters()
}

function onAutoRefreshEnabled(value: boolean) {
  setAutoRefresh(value, autoRefreshIntervalMs.value)
}

function onAutoRefreshInterval(value: number) {
  setAutoRefresh(autoRefreshEnabled.value, value)
}

watch(
  () => route.query,
  (query) => {
    parseFiltersFromQuery(query as Record<string, unknown>)
  },
)

onMounted(async () => {
  parseFiltersFromQuery(route.query as Record<string, unknown>)
  await refreshAll()
})

onBeforeUnmount(() => {
  setAutoRefresh(false)
})
</script>

<template>
  <div class="page-stack control-tower-v01">
    <UiEmptyState
      v-if="!hasAccess"
      :title="$t('controlTower.accessDeniedTitle')"
      :description="$t('controlTower.accessDeniedDescription')"
    />

    <template v-else>
      <ControlTowerToolbar
        :last-updated-at="lastUpdatedAt"
        :backend-online="backendOnline"
        :loading="loading"
        :auto-refresh-enabled="autoRefreshEnabled"
        :auto-refresh-interval-ms="autoRefreshIntervalMs"
        @refresh="refreshAll"
        @update:auto-refresh-enabled="onAutoRefreshEnabled"
        @update:auto-refresh-interval-ms="onAutoRefreshInterval"
      />

      <div v-if="demoMode" class="control-tower-v01__demo-banner">
        {{ $t('controlTower.demoMode') }}
      </div>

      <CommonApiUnavailableState
        v-if="apiUnavailable && !loading"
        :title="$t('controlTower.apiUnavailable')"
        :message="$t('controlTower.backendOffline')"
        @retry="refreshAll"
      />

      <template v-else>
        <section class="control-tower-v01__kpi-grid">
          <ControlTowerMetricCard v-for="metric in kpiMetrics" :key="metric.key" :metric="metric" />
        </section>

        <UiCard>
          <h2 class="control-tower-v01__section-title">{{ $t('controlTower.sections.filters') }}</h2>
          <ControlTowerFiltersBar
            :filters="filters"
            :shipper-companies="shipperCompanies"
            :carrier-companies="carrierCompanies"
            @change="onFiltersChange"
            @reset="onResetFilters"
          />
        </UiCard>

        <div class="control-tower-v01__main-grid">
          <UiCard class="control-tower-v01__table-card">
            <h2 class="control-tower-v01__section-title">
              {{ $t('controlTower.sections.activeShipments') }}
            </h2>
            <UiEmptyState
              v-if="isEmptyDataset"
              :title="$t('controlTower.states.noActiveShipments')"
              :description="$t('controlTower.states.noActiveShipmentsHint')"
            />
            <UiEmptyState
              v-else-if="isEmptyFiltered"
              :title="$t('controlTower.states.noFilteredResults')"
              :description="$t('controlTower.states.noFilteredResultsHint')"
            />
            <ControlTowerActiveShipmentsTable
              v-else
              :rows="filteredShipments"
              :loading="loading"
            />
          </UiCard>

          <UiCard class="control-tower-v01__events-card">
            <h2 class="control-tower-v01__section-title">
              {{ $t('controlTower.sections.criticalEvents') }}
            </h2>
            <ControlTowerCriticalEventsPanel :events="criticalEvents" :loading="loading" />
          </UiCard>
        </div>
      </template>
    </template>
  </div>
</template>

<style scoped>
.control-tower-v01__demo-banner {
  padding: 0.75rem 1rem;
  border-radius: var(--radius-md);
  border: 1px solid var(--color-border);
  background: color-mix(in srgb, var(--color-info) 10%, white);
  color: var(--color-info);
  font-size: 0.875rem;
  font-weight: 600;
}

.control-tower-v01__kpi-grid {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(200px, 1fr));
  gap: 1rem;
}

.control-tower-v01__section-title {
  margin: 0 0 0.75rem;
  font-size: 1.0625rem;
  font-weight: 700;
}

.control-tower-v01__main-grid {
  display: grid;
  grid-template-columns: minmax(0, 2fr) minmax(280px, 1fr);
  gap: 1rem;
  align-items: start;
}

@media (max-width: 1100px) {
  .control-tower-v01__main-grid {
    grid-template-columns: 1fr;
  }
}
</style>
