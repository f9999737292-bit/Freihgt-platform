<script setup lang="ts">
import type { ShipmentSlotQueryContext } from '~/types/slot'
import { formatSlotWindow, useSlotApi } from '~/composables/useSlotApi'

const props = defineProps<{
  shipmentId?: string | null
  slotContext?: ShipmentSlotQueryContext
  bookingStatus?: string | null
}>()

const { t } = useI18n()
const { getShipmentSlots } = useSlotApi()

const loading = ref(false)
const delivery = ref<Awaited<ReturnType<typeof getShipmentSlots>>['delivery']>(null)

const summaryLine = computed(() => {
  const target = delivery.value
  if (!target || target.windowStatus === 'unavailable') {
    if (props.bookingStatus === 'DELIVERY_SLOT_BOOKED') {
      return t('slot.bookedTimeUnavailable')
    }
    return t('slot.unavailable')
  }
  const window = formatSlotWindow(target.windowStart, target.windowEnd)
  const projection = t(`slot.projection.${target.arrivalProjection}`)
  if (target.arrivalProjection === 'projected_miss' && target.projectedLateBySeconds) {
    const mins = Math.round(target.projectedLateBySeconds / 60)
    return t('slot.compactProjectedMiss', { window, mins, projection })
  }
  if (target.arrivalProjection === 'at_risk' && target.marginSeconds != null) {
    const mins = Math.max(1, Math.round(target.marginSeconds / 60))
    return t('slot.compactAtRisk', { window, mins, projection })
  }
  return t('slot.compactWithWindow', { window, projection })
})

watch(
  () => [props.shipmentId, props.slotContext] as const,
  async ([shipmentId]) => {
    delivery.value = null
    if (!shipmentId) return
    loading.value = true
    try {
      const summary = await getShipmentSlots(shipmentId, props.slotContext)
      delivery.value = summary.delivery ?? null
    } catch {
      delivery.value = null
    } finally {
      loading.value = false
    }
  },
  { immediate: true, deep: true },
)
</script>

<template>
  <section v-if="shipmentId" class="slot-summary">
    <h4>{{ $t('slot.title') }}</h4>
    <p v-if="loading">{{ $t('common.loading') }}</p>
    <p v-else>{{ summaryLine }}</p>
  </section>
</template>

<style scoped>
.slot-summary h4 {
  margin: 0 0 0.35rem;
  font-size: 0.95rem;
}
.slot-summary p {
  margin: 0;
}
</style>
