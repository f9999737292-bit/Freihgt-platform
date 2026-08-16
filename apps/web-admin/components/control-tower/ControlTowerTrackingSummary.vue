<script setup lang="ts">
import type { ControlTowerWorkItem } from '~/types/controlTower'
import type { ShipmentTrackingSummary } from '~/types/tracking'
import { formatTrackingAge } from '~/composables/useTrackingApi'

const props = defineProps<{
  shipmentId?: string | null
}>()

const { t } = useI18n()
const { getShipmentTracking } = useTrackingApi()

const loading = ref(false)
const tracking = ref<ShipmentTrackingSummary | null>(null)

const summaryLine = computed(() => {
  if (!tracking.value) return t('tracking.unavailable')
  const status = t(`tracking.status.${tracking.value.trackingStatus}`)
  const age = formatTrackingAge(tracking.value.freshness.ageSeconds ?? tracking.value.lastKnownPosition?.ageSeconds)
  if (tracking.value.trackingStatus === 'not_configured') {
    return t('tracking.notConfiguredShort')
  }
  if (age) {
    return t('tracking.compactSummaryWithAge', { status, age })
  }
  return status
})

watch(
  () => props.shipmentId,
  async (shipmentId) => {
    tracking.value = null
    if (!shipmentId) return
    loading.value = true
    try {
      tracking.value = await getShipmentTracking(shipmentId)
    } catch {
      tracking.value = null
    } finally {
      loading.value = false
    }
  },
  { immediate: true },
)
</script>

<template>
  <section v-if="shipmentId" class="tracking-summary">
    <h4>{{ $t('tracking.title') }}</h4>
    <p v-if="loading">{{ $t('common.loading') }}</p>
    <p v-else>{{ summaryLine }}</p>
    <p v-if="tracking && tracking.trackingStatus !== 'not_configured'" class="tracking-summary__quality">
      {{ $t('tracking.qualityLabel') }}: {{ $t(`tracking.quality.${tracking.quality.status}`) }}
    </p>
  </section>
</template>

<style scoped>
.tracking-summary h4 {
  margin: 0 0 0.35rem;
  font-size: 0.95rem;
}
.tracking-summary p {
  margin: 0;
}
.tracking-summary__quality {
  margin-top: 0.25rem !important;
  color: var(--color-text-muted, #666);
  font-size: 0.875rem;
}
</style>
