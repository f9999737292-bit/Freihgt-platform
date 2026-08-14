<script setup lang="ts">
definePageMeta({ middleware: 'auth', layout: 'default' })

import type { User } from '~/types/user'
import type { ControlTowerEvent, ControlTowerEventAction, ControlTowerEventResolutionCode } from '~/types/controlTower'

const route = useRoute()
const router = useRouter()
const { canAccessControlTower } = usePermissions()
const { backendOnline, checkBackendStatus } = useBackendStatus()
const { listUsers } = useUsersApi()

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
  acknowledgingEventId,
  workflowActionEventId,
  acknowledgeCriticalEvent,
  assignCriticalEvent,
  resolveCriticalEvent,
  reopenCriticalEvent,
  fetchCriticalEventActions,
  apiUnavailable,
  dataFreshness,
  summaryPagination,
  shipperCompanies,
  carrierCompanies,
  resetFilters,
  setAutoRefresh,
  parseFiltersFromQuery,
  filtersToQuery,
  loadData,
  statusSummary,
  statusSummaryFreshness,
} = useControlTower()

const showReadModelSourceBanner = computed(
  () =>
    !demoMode.value &&
    statusSummaryFreshness.value?.loaded &&
    statusSummary.value?.source === 'READ_MODEL' &&
    !statusSummaryFreshness.value.fallbackUsed,
)

const showReadModelFallbackBanner = computed(
  () => !demoMode.value && statusSummaryFreshness.value?.fallbackUsed === true,
)

const showReadModelPartialBanner = computed(
  () =>
    !demoMode.value &&
    !statusSummary.value?.limitedDataset &&
    (statusSummaryFreshness.value?.partial === true ||
      (dataFreshness.value?.warnings ?? []).includes('CONTROL_TOWER_READ_MODEL_PARTIAL')),
)

const showLegacyLimitedBanner = computed(
  () =>
    !demoMode.value &&
    (statusSummary.value?.limitedDataset === true ||
      (dataFreshness.value?.warnings ?? []).includes('CONTROL_TOWER_LEGACY_STATUS_SUMMARY_LIMITED')),
)

const hasAccess = computed(() => canAccessControlTower())

const canAcknowledgeEvents = computed(() => hasAccess.value && !demoMode.value)
const canAssignEvents = computed(() => hasAccess.value && !demoMode.value)
const canResolveEvents = computed(() => hasAccess.value && !demoMode.value)

const assignModalOpen = ref(false)
const resolveModalOpen = ref(false)
const detailsModalOpen = ref(false)
const selectedEventId = ref<string | null>(null)
const selectedEvent = ref<ControlTowerEvent | null>(null)
const tenantUsers = ref<User[]>([])
const usersLoading = ref(false)
const detailsLoading = ref(false)
const detailsActions = ref<ControlTowerEventAction[]>([])

const isReassign = computed(() => {
  if (!selectedEventId.value) return false
  const event = criticalEvents.value.find((item) => item.id === selectedEventId.value)
  return event?.status === 'assigned'
})

async function loadTenantUsers() {
  usersLoading.value = true
  try {
    const response = await listUsers({ limit: 200, offset: 0 })
    tenantUsers.value = response.items ?? []
  } catch {
    tenantUsers.value = []
  } finally {
    usersLoading.value = false
  }
}

function onAcknowledgeEvent(eventId: string) {
  void acknowledgeCriticalEvent(eventId)
}

async function onAssignEvent(eventId: string) {
  selectedEventId.value = eventId
  assignModalOpen.value = true
  if (tenantUsers.value.length === 0) {
    await loadTenantUsers()
  }
}

function onResolveEvent(eventId: string) {
  selectedEventId.value = eventId
  resolveModalOpen.value = true
}

function onReopenEvent(eventId: string) {
  void reopenCriticalEvent(eventId)
}

async function onAssignSubmit(userId: string) {
  if (!selectedEventId.value) return
  const ok = await assignCriticalEvent(selectedEventId.value, userId)
  if (ok) assignModalOpen.value = false
}

async function onResolveSubmit(resolutionCode: ControlTowerEventResolutionCode, comment?: string) {
  if (!selectedEventId.value) return
  const ok = await resolveCriticalEvent(selectedEventId.value, resolutionCode, comment)
  if (ok) resolveModalOpen.value = false
}

async function onShowEventDetails(event: ControlTowerEvent) {
  selectedEvent.value = event
  detailsModalOpen.value = true
  detailsLoading.value = true
  detailsActions.value = []
  const response = await fetchCriticalEventActions(event.id)
  detailsActions.value = response?.items ?? []
  detailsLoading.value = false
}

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
  summaryPagination.value.page = 1
  void loadData()
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

      <div
        v-if="showReadModelSourceBanner"
        class="control-tower-v01__read-model-banner control-tower-v01__read-model-banner--source"
      >
        {{ $t('controlTower.readModel.sourceBanner') }}
      </div>

      <div
        v-if="showReadModelFallbackBanner"
        class="control-tower-v01__read-model-banner control-tower-v01__read-model-banner--fallback"
      >
        {{ $t('controlTower.readModel.fallbackBanner') }}
      </div>

      <div
        v-if="showReadModelPartialBanner"
        class="control-tower-v01__read-model-banner control-tower-v01__read-model-banner--partial"
      >
        {{ $t('controlTower.readModel.partialBanner') }}
      </div>

      <div
        v-if="showLegacyLimitedBanner"
        class="control-tower-v01__read-model-banner control-tower-v01__read-model-banner--limited"
      >
        {{ $t('controlTower.readModel.legacyLimitedBanner') }}
      </div>

      <div
        v-if="dataFreshness?.partial"
        class="control-tower-v01__partial-banner"
      >
        <p>{{ $t('controlTower.partialDataWarning') }}</p>
        <ul v-if="dataFreshness.warnings?.length" class="control-tower-v01__partial-list">
          <li v-for="warning in dataFreshness.warnings" :key="warning">
            {{ $t(`controlTower.partialWarnings.${warning}`) }}
          </li>
        </ul>
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
            <ControlTowerCriticalEventsPanel
              :events="criticalEvents"
              :loading="loading"
              :can-acknowledge="canAcknowledgeEvents"
              :can-assign="canAssignEvents"
              :can-resolve="canResolveEvents"
              :acknowledging-event-id="acknowledgingEventId"
              :workflow-action-event-id="workflowActionEventId"
              @acknowledge="onAcknowledgeEvent"
              @assign="onAssignEvent"
              @resolve="onResolveEvent"
              @reopen="onReopenEvent"
              @show-details="onShowEventDetails"
            />
          </UiCard>
        </div>
      </template>

      <ControlTowerCriticalEventAssignModal
        :open="assignModalOpen"
        :loading="Boolean(workflowActionEventId)"
        :users="tenantUsers"
        :users-loading="usersLoading"
        :reassign="isReassign"
        @close="assignModalOpen = false"
        @submit="onAssignSubmit"
      />

      <ControlTowerCriticalEventResolveModal
        :open="resolveModalOpen"
        :loading="Boolean(workflowActionEventId)"
        @close="resolveModalOpen = false"
        @submit="onResolveSubmit"
      />

      <ControlTowerCriticalEventDetailsModal
        :open="detailsModalOpen"
        :event="selectedEvent"
        :actions="detailsActions"
        :loading="detailsLoading"
        @close="detailsModalOpen = false"
      />
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

.control-tower-v01__read-model-banner {
  padding: 0.75rem 1rem;
  border-radius: var(--radius-md);
  font-size: 0.875rem;
  font-weight: 600;
}

.control-tower-v01__read-model-banner--source {
  border: 1px solid color-mix(in srgb, var(--color-info) 35%, var(--color-border));
  background: color-mix(in srgb, var(--color-info) 8%, white);
  color: var(--color-info);
}

.control-tower-v01__read-model-banner--fallback {
  border: 1px solid color-mix(in srgb, var(--color-warning) 35%, var(--color-border));
  background: color-mix(in srgb, var(--color-warning) 8%, white);
  color: var(--color-text);
}

.control-tower-v01__read-model-banner--partial {
  border: 1px solid color-mix(in srgb, var(--color-warning) 35%, var(--color-border));
  background: color-mix(in srgb, var(--color-warning) 6%, white);
  color: var(--color-text);
}

.control-tower-v01__read-model-banner--limited {
  border: 1px solid color-mix(in srgb, var(--color-warning) 35%, var(--color-border));
  background: color-mix(in srgb, var(--color-warning) 5%, white);
  color: var(--color-text);
}

.control-tower-v01__partial-banner {
  padding: 0.75rem 1rem;
  border-radius: var(--radius-md);
  border: 1px solid color-mix(in srgb, var(--color-warning) 35%, var(--color-border));
  background: color-mix(in srgb, var(--color-warning) 8%, white);
  color: var(--color-text);
  font-size: 0.875rem;
}

.control-tower-v01__partial-banner p {
  margin: 0;
  font-weight: 600;
}

.control-tower-v01__partial-list {
  margin: 0.5rem 0 0;
  padding-left: 1.25rem;
  color: var(--color-text-muted);
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
