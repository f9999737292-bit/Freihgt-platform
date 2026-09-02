<script setup lang="ts">
import { shipmentDetailRoute } from '~/utils/shipmentDetailNavigation'

definePageMeta({ middleware: 'auth', layout: 'default' })

const route = useRoute()
const router = useRouter()

const shipmentId = computed(() => String(route.params.id))

const {
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
} = useShipmentEvents(shipmentId)

const draftFilters = ref({ ...filters.value })
watch(filters, (value) => {
  draftFilters.value = { ...value }
}, { deep: true })

function applyFilters() {
  updateFilters(draftFilters.value)
}

function handleResetFilters() {
  resetFilters()
}

function goToShipmentDetail() {
  router.push(shipmentDetailRoute(shipmentId.value))
}
</script>

<template>
  <div class="page-stack">
    <nav class="breadcrumbs" aria-label="Breadcrumb">
      <NuxtLink to="/shipments">{{ $t('shipments.title') }}</NuxtLink>
      <span class="breadcrumbs__sep">/</span>
      <NuxtLink :to="`/shipments/${shipmentId}`">{{ data?.shipment.number || shipmentId }}</NuxtLink>
      <span class="breadcrumbs__sep">/</span>
      <span>{{ $t('shipmentEvents.title') }}</span>
    </nav>

    <UiPageHeader :title="$t('shipmentEvents.title')">
      <template #subtitle>
        <span v-if="data?.shipment.number">{{ data.shipment.number }}</span>
      </template>
      <template #actions>
        <UiButton variant="secondary" data-testid="event-history-back-to-shipment" @click="goToShipmentDetail">
          {{ $t('shipmentEvents.backToShipment') }}
        </UiButton>
        <UiButton variant="secondary" @click="fetchEvents">{{ $t('common.refresh') }}</UiButton>
      </template>
    </UiPageHeader>

    <div v-if="accessDenied" class="state-card state-card--error">
      <h3>{{ $t('shipmentEvents.accessDeniedTitle') }}</h3>
      <p>{{ $t('shipmentEvents.accessDeniedMessage') }}</p>
    </div>

    <UiEmptyState v-else-if="notFound" :title="$t('shipmentEvents.notFoundTitle')" />

    <CommonApiUnavailableState
      v-else-if="apiUnavailable"
      :title="$t('shipmentEvents.unavailableTitle')"
      :message="$t('shipmentEvents.unavailableMessage')"
      @retry="fetchEvents"
    />

    <div v-else-if="apiError" class="state-card state-card--error">
      <h3>{{ $t('common.error') }}</h3>
      <p>{{ apiError }}</p>
    </div>

    <template v-else>
      <div v-if="isPartial" class="partial-warning">
        <strong>{{ $t('shipmentEvents.partialWarningTitle') }}</strong>
        <p>{{ $t('shipmentEvents.partialWarningMessage') }}</p>
        <ul v-if="data?.dataFreshness.warnings?.length">
          <li v-for="warning in data.dataFreshness.warnings" :key="warning">
            {{ $t(`shipmentEvents.warnings.${warning}`, warning) }}
          </li>
        </ul>
      </div>

      <div v-if="isDerivedOnly && !loading" class="derived-note">
        {{ $t('shipmentEvents.derivedOnlyNote') }}
      </div>

      <ShipmentEventsShipmentEventFilters
        v-model="draftFilters"
        :disabled="loading"
        @apply="applyFilters"
        @reset="handleResetFilters"
      />

      <UiCard>
        <template #header>
          <h3 class="card-title">{{ $t('shipmentEvents.timelineTitle') }}</h3>
        </template>
        <ShipmentEventsShipmentTimeline :response="data" :loading="loading" />
        <UiEmptyState
          v-if="isEmpty && !loading && Object.keys(route.query).length > 0"
          :title="$t('shipmentEvents.filtersEmpty')"
        />
      </UiCard>
    </template>
  </div>
</template>

<style scoped>
.breadcrumbs {
  display: flex;
  align-items: center;
  gap: 0.5rem;
  font-size: 0.875rem;
  color: var(--color-text-muted);
}

.breadcrumbs__sep {
  opacity: 0.5;
}

.state-card {
  padding: 1rem 1.25rem;
  border-radius: var(--radius-lg);
  border: 1px solid var(--color-border);
}

.state-card--error {
  border-color: #fecaca;
  background: #fef2f2;
  color: #991b1b;
}

.partial-warning {
  padding: 0.75rem 1rem;
  border-radius: var(--radius-lg);
  border: 1px solid #fde68a;
  background: #fffbeb;
  color: #92400e;
  font-size: 0.875rem;
}

.partial-warning ul {
  margin: 0.35rem 0 0;
  padding-left: 1.25rem;
}

.derived-note {
  font-size: 0.875rem;
  color: var(--color-text-muted);
}
</style>
