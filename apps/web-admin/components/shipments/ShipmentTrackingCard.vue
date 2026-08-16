<script setup lang="ts">
import type { ShipmentLocationHistoryItem, ShipmentTrackingSummary } from '~/types/tracking'
import { formatTrackingAge } from '~/composables/useTrackingApi'

const props = defineProps<{
  shipmentId: string
}>()

const { t } = useI18n()
const { getShipmentTracking, listShipmentLocations } = useTrackingApi()

const loading = ref(true)
const unavailable = ref(false)
const tracking = ref<ShipmentTrackingSummary | null>(null)
const history = ref<ShipmentLocationHistoryItem[]>([])
const showHistory = ref(false)
const historyLoading = ref(false)

const statusLabelKey = computed(() => {
  const status = tracking.value?.trackingStatus ?? 'not_configured'
  return `tracking.status.${status}`
})

const freshnessLabelKey = computed(() => {
  const status = tracking.value?.freshness.status ?? 'unknown'
  return `tracking.freshness.${status}`
})

const qualityLabelKey = computed(() => {
  const status = tracking.value?.quality.status ?? 'unknown'
  return `tracking.quality.${status}`
})

const ageLabel = computed(() => {
  const seconds = tracking.value?.freshness.ageSeconds ?? tracking.value?.lastKnownPosition?.ageSeconds
  const formatted = formatTrackingAge(seconds)
  return formatted ? t('tracking.lastTelemetryAgo', { age: formatted }) : null
})

async function loadTracking() {
  loading.value = true
  unavailable.value = false
  try {
    tracking.value = await getShipmentTracking(props.shipmentId)
  } catch {
    unavailable.value = true
    tracking.value = null
  } finally {
    loading.value = false
  }
}

async function loadHistory() {
  if (history.value.length > 0) return
  historyLoading.value = true
  try {
    const response = await listShipmentLocations(props.shipmentId, { limit: 20 })
    history.value = response.items
  } catch {
    history.value = []
  } finally {
    historyLoading.value = false
  }
}

watch(
  () => props.shipmentId,
  () => {
    history.value = []
    showHistory.value = false
    loadTracking()
  },
  { immediate: true },
)

watch(showHistory, (open) => {
  if (open) loadHistory()
})
</script>

<template>
  <UiCard>
    <template #header>
      <h3 class="card-title">{{ $t('tracking.title') }}</h3>
    </template>

    <div v-if="loading" class="loading-block">{{ $t('common.loading') }}</div>
    <p v-else-if="unavailable" class="tracking-muted">{{ $t('tracking.unavailable') }}</p>
    <template v-else-if="tracking">
      <dl class="tracking-grid">
        <dt>{{ $t('tracking.statusLabel') }}</dt>
        <dd>{{ $t(statusLabelKey) }}</dd>

        <template v-if="tracking.provider">
          <dt>{{ $t('tracking.provider') }}</dt>
          <dd>{{ tracking.provider }}</dd>
        </template>

        <template v-if="tracking.trackingStatus !== 'not_configured'">
          <dt>{{ $t('tracking.freshnessLabel') }}</dt>
          <dd>{{ $t(freshnessLabelKey) }}</dd>

          <dt>{{ $t('tracking.qualityLabel') }}</dt>
          <dd>{{ $t(qualityLabelKey) }}</dd>
        </template>

        <template v-if="ageLabel">
          <dt>{{ $t('tracking.lastTelemetry') }}</dt>
          <dd>{{ ageLabel }}</dd>
        </template>

        <template v-if="tracking.lastKnownPosition">
          <dt>{{ $t('tracking.lastKnownPosition') }}</dt>
          <dd>
            {{ tracking.lastKnownPosition.latitude.toFixed(5) }},
            {{ tracking.lastKnownPosition.longitude.toFixed(5) }}
          </dd>
        </template>
      </dl>

      <div v-if="tracking.trackingStatus !== 'not_configured'" class="tracking-actions">
        <UiButton size="sm" variant="secondary" @click="showHistory = !showHistory">
          {{ showHistory ? $t('tracking.hideHistory') : $t('tracking.locationHistory') }}
        </UiButton>
      </div>

      <div v-if="showHistory" class="tracking-history">
        <div v-if="historyLoading" class="loading-block">{{ $t('common.loading') }}</div>
        <UiEmptyState v-else-if="!history.length" :title="$t('tracking.noHistory')" />
        <table v-else class="tracking-history__table">
          <thead>
            <tr>
              <th>{{ $t('tracking.recordedAt') }}</th>
              <th>{{ $t('tracking.receivedAt') }}</th>
              <th>{{ $t('tracking.coordinates') }}</th>
              <th>{{ $t('tracking.speed') }}</th>
            </tr>
          </thead>
          <tbody>
            <tr v-for="item in history" :key="item.id">
              <td>{{ item.recordedAt }}</td>
              <td>{{ item.receivedAt }}</td>
              <td>{{ item.latitude.toFixed(5) }}, {{ item.longitude.toFixed(5) }}</td>
              <td>{{ item.speedKph ?? '—' }}</td>
            </tr>
          </tbody>
        </table>
      </div>
    </template>
  </UiCard>
</template>

<style scoped>
.tracking-grid {
  display: grid;
  grid-template-columns: minmax(140px, 220px) 1fr;
  gap: 0.35rem 1rem;
  margin: 0;
}
.tracking-grid dt {
  color: var(--color-text-muted, #666);
}
.tracking-grid dd {
  margin: 0;
}
.tracking-muted {
  color: var(--color-text-muted, #666);
}
.tracking-actions {
  margin-top: 0.75rem;
}
.tracking-history {
  margin-top: 1rem;
}
.tracking-history__table {
  width: 100%;
  border-collapse: collapse;
  font-size: 0.875rem;
}
.tracking-history__table th,
.tracking-history__table td {
  border-bottom: 1px solid var(--color-border, #e5e7eb);
  padding: 0.35rem 0.5rem;
  text-align: left;
}
</style>
