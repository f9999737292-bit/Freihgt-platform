<script setup lang="ts">
import type { ShipmentETAContext, ShipmentETASummary } from '~/types/eta'
import { formatDeviationMinutes, formatETAAge, useEtaApi } from '~/composables/useEtaApi'

const props = defineProps<{
  shipmentId?: string | null
  plannedContext?: ShipmentETAContext
}>()

const { t } = useI18n()
const { getShipmentETA } = useEtaApi()

const loading = ref(false)
const eta = ref<ShipmentETASummary | null>(null)

const delivery = computed(() => eta.value?.delivery ?? null)

const summaryLine = computed(() => {
  if (!delivery.value || delivery.value.status === 'unavailable') {
    return t('eta.unavailable')
  }
  const etaTime = delivery.value.estimatedArrivalAt
    ? new Date(delivery.value.estimatedArrivalAt).toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' })
    : '—'
  const plannedTime = delivery.value.plannedArrivalAt
    ? new Date(delivery.value.plannedArrivalAt).toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' })
    : null
  const deviation = delivery.value.projectedDeviationSeconds
  const deviationMinutes = formatDeviationMinutes(deviation)

  if (delivery.value.freshnessStatus === 'stale' || delivery.value.freshnessStatus === 'expired') {
    const age = formatETAAge(delivery.value.ageSeconds)
    return age
      ? t('eta.compactStale', { eta: etaTime, age })
      : t('eta.compactStaleShort', { eta: etaTime })
  }

  if (plannedTime && deviationMinutes != null && Number(deviation) > 0) {
    return t('eta.compactWithDelay', {
      eta: etaTime,
      planned: plannedTime,
      minutes: deviationMinutes,
      projection: t(`eta.projection.${delivery.value.arrivalProjection}`),
    })
  }

  if (plannedTime) {
    return t('eta.compactWithPlanned', { eta: etaTime, planned: plannedTime })
  }

  return t('eta.compactEtaOnly', { eta: etaTime })
})

watch(
  () => [props.shipmentId, props.plannedContext] as const,
  async ([shipmentId]) => {
    eta.value = null
    if (!shipmentId) return
    loading.value = true
    try {
      eta.value = await getShipmentETA(shipmentId, props.plannedContext)
    } catch {
      eta.value = null
    } finally {
      loading.value = false
    }
  },
  { immediate: true, deep: true },
)
</script>

<template>
  <section v-if="shipmentId" class="eta-summary">
    <h4>{{ $t('eta.title') }}</h4>
    <p v-if="loading">{{ $t('common.loading') }}</p>
    <p v-else>{{ summaryLine }}</p>
    <p
      v-if="delivery && delivery.status !== 'unavailable'"
      class="eta-summary__meta"
    >
      {{ $t('eta.freshnessLabel') }}: {{ $t(`eta.freshness.${delivery.freshnessStatus}`) }}
      · {{ $t('eta.qualityLabel') }}: {{ $t(`eta.quality.${delivery.qualityStatus}`) }}
    </p>
  </section>
</template>

<style scoped>
.eta-summary h4 {
  margin: 0 0 0.35rem;
  font-size: 0.95rem;
}
.eta-summary p {
  margin: 0;
}
.eta-summary__meta {
  margin-top: 0.25rem !important;
  color: var(--color-text-muted, #666);
  font-size: 0.875rem;
}
</style>
