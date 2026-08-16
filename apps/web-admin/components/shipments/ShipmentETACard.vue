<script setup lang="ts">
import type { ETAObservation, ShipmentETAContext, ShipmentETASummary } from '~/types/eta'
import { formatDeviationMinutes, formatETAAge, useEtaApi } from '~/composables/useEtaApi'

const props = defineProps<{
  shipmentId: string
  plannedContext?: ShipmentETAContext
}>()

const { t } = useI18n()
const { getShipmentETA, listETAHistory } = useEtaApi()

const loading = ref(true)
const unavailable = ref(false)
const eta = ref<ShipmentETASummary | null>(null)
const history = ref<ETAObservation[]>([])
const showHistory = ref(false)
const historyLoading = ref(false)

const delivery = computed(() => eta.value?.delivery ?? null)

const statusLabelKey = computed(() => {
  const status = delivery.value?.status ?? 'unavailable'
  return `eta.status.${status}`
})

const freshnessLabelKey = computed(() => {
  const status = delivery.value?.freshnessStatus ?? 'unknown'
  return `eta.freshness.${status}`
})

const qualityLabelKey = computed(() => {
  const status = delivery.value?.qualityStatus ?? 'unknown'
  return `eta.quality.${status}`
})

const sourceLabelKey = computed(() => {
  const source = delivery.value?.sourceType
  if (!source) return null
  return `eta.source.${source}`
})

const projectionLabelKey = computed(() => {
  const projection = delivery.value?.arrivalProjection ?? 'unknown'
  return `eta.projection.${projection}`
})

const ageLabel = computed(() => {
  const formatted = formatETAAge(delivery.value?.ageSeconds)
  return formatted ? t('eta.updatedAgo', { age: formatted }) : null
})

const deviationLabel = computed(() => {
  const seconds = delivery.value?.projectedDeviationSeconds
  if (seconds == null || delivery.value?.status === 'unavailable') return null
  const minutes = formatDeviationMinutes(seconds)
  if (minutes == null) return null
  if (Number(seconds) < 0) {
    return t('eta.projectedEarlyMinutes', { minutes })
  }
  if (Number(seconds) > 0) {
    return t('eta.projectedDelayMinutes', { minutes })
  }
  return t('eta.projection.on_time')
})

async function loadETA() {
  loading.value = true
  unavailable.value = false
  try {
    eta.value = await getShipmentETA(props.shipmentId, props.plannedContext)
  } catch {
    unavailable.value = true
    eta.value = null
  } finally {
    loading.value = false
  }
}

async function loadHistory() {
  if (history.value.length > 0) return
  historyLoading.value = true
  try {
    const response = await listETAHistory(props.shipmentId, { targetType: 'delivery', limit: 20 })
    history.value = response.items
  } catch {
    history.value = []
  } finally {
    historyLoading.value = false
  }
}

watch(
  () => [props.shipmentId, props.plannedContext] as const,
  () => {
    history.value = []
    showHistory.value = false
    loadETA()
  },
  { immediate: true, deep: true },
)

watch(showHistory, (open) => {
  if (open) loadHistory()
})
</script>

<template>
  <UiCard>
    <template #header>
      <h3 class="card-title">{{ $t('eta.title') }}</h3>
    </template>

    <div v-if="loading" class="loading-block">{{ $t('common.loading') }}</div>
    <p v-else-if="unavailable" class="eta-muted">{{ $t('eta.unavailable') }}</p>
    <template v-else-if="delivery">
      <p v-if="delivery.status === 'unavailable'" class="eta-muted">{{ $t('eta.unavailable') }}</p>
      <template v-else>
        <dl class="eta-grid">
          <dt>{{ $t('eta.predictedArrival') }}</dt>
          <dd>{{ delivery.estimatedArrivalAt || '—' }}</dd>

          <dt>{{ $t('eta.plannedArrival') }}</dt>
          <dd>{{ delivery.plannedArrivalAt || '—' }}</dd>

          <dt>{{ $t('eta.projectedDeviation') }}</dt>
          <dd>{{ deviationLabel || '—' }}</dd>

          <dt>{{ $t('eta.arrivalProjection') }}</dt>
          <dd>{{ $t(projectionLabelKey) }}</dd>

          <dt>{{ $t('eta.sourceLabel') }}</dt>
          <dd>
            <span v-if="sourceLabelKey">{{ $t(sourceLabelKey) }}</span>
            <span v-if="delivery.provider"> · {{ delivery.provider }}</span>
            <span v-if="!sourceLabelKey && !delivery.provider">—</span>
          </dd>

          <dt>{{ $t('eta.freshnessLabel') }}</dt>
          <dd>{{ $t(freshnessLabelKey) }}</dd>

          <dt>{{ $t('eta.qualityLabel') }}</dt>
          <dd>{{ $t(qualityLabelKey) }}</dd>

          <dt>{{ $t('eta.statusLabel') }}</dt>
          <dd>{{ $t(statusLabelKey) }}</dd>

          <template v-if="ageLabel">
            <dt>{{ $t('eta.lastUpdate') }}</dt>
            <dd>{{ ageLabel }}</dd>
          </template>
        </dl>

        <div class="eta-actions">
          <UiButton size="sm" variant="secondary" @click="showHistory = !showHistory">
            {{ showHistory ? $t('eta.hideHistory') : $t('eta.history') }}
          </UiButton>
        </div>

        <div v-if="showHistory" class="eta-history">
          <div v-if="historyLoading" class="loading-block">{{ $t('common.loading') }}</div>
          <UiEmptyState v-else-if="!history.length" :title="$t('eta.noHistory')" />
          <table v-else class="eta-history__table">
            <thead>
              <tr>
                <th>{{ $t('eta.estimatedArrival') }}</th>
                <th>{{ $t('eta.sourceLabel') }}</th>
                <th>{{ $t('eta.lastUpdate') }}</th>
                <th>{{ $t('eta.qualityLabel') }}</th>
              </tr>
            </thead>
            <tbody>
              <tr v-for="item in history" :key="item.id">
                <td>{{ item.estimatedArrivalAt }}</td>
                <td>{{ item.sourceType }}{{ item.provider ? ` · ${item.provider}` : '' }}</td>
                <td>{{ item.sourceObservedAt }}</td>
                <td>{{ $t(`eta.quality.${item.qualityStatus}`) }}</td>
              </tr>
            </tbody>
          </table>
        </div>
      </template>
    </template>
    <p v-else class="eta-muted">{{ $t('eta.unavailable') }}</p>
  </UiCard>
</template>

<style scoped>
.eta-grid {
  display: grid;
  grid-template-columns: minmax(140px, 220px) 1fr;
  gap: 0.35rem 1rem;
  margin: 0;
}
.eta-grid dt {
  color: var(--color-text-muted, #666);
}
.eta-grid dd {
  margin: 0;
}
.eta-muted {
  color: var(--color-text-muted, #666);
}
.eta-actions {
  margin-top: 0.75rem;
}
.eta-history {
  margin-top: 1rem;
}
.eta-history__table {
  width: 100%;
  border-collapse: collapse;
  font-size: 0.875rem;
}
.eta-history__table th,
.eta-history__table td {
  border-bottom: 1px solid var(--color-border, #e5e7eb);
  padding: 0.35rem 0.5rem;
  text-align: left;
}
</style>
