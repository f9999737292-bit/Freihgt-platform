<script setup lang="ts">
import type { ShipmentSlotQueryContext, ShipmentSlotSummary, SlotRevision } from '~/types/slot'
import { formatSlotWindow, useSlotApi } from '~/composables/useSlotApi'

const props = defineProps<{
  shipmentId: string
  slotContext?: ShipmentSlotQueryContext
  bookingStatus?: string | null
}>()

const { t } = useI18n()
const { getShipmentSlots, listSlotHistory } = useSlotApi()

const loading = ref(true)
const unavailable = ref(false)
const slots = ref<ShipmentSlotSummary | null>(null)
const history = ref<SlotRevision[]>([])
const showHistory = ref(false)
const historyLoading = ref(false)

const delivery = computed(() => slots.value?.delivery ?? null)

function windowLabel(target: typeof delivery.value) {
  if (!target || target.windowStatus === 'unavailable') {
    if (props.bookingStatus === 'DELIVERY_SLOT_BOOKED' || props.bookingStatus === 'PICKUP_SLOT_BOOKED') {
      return t('slot.timeUnavailable')
    }
    return t('slot.unavailable')
  }
  return formatSlotWindow(target.windowStart, target.windowEnd) || t('slot.unavailable')
}

async function loadSlots() {
  loading.value = true
  unavailable.value = false
  try {
    slots.value = await getShipmentSlots(props.shipmentId, props.slotContext)
  } catch {
    unavailable.value = true
    slots.value = null
  } finally {
    loading.value = false
  }
}

async function loadHistory() {
  if (history.value.length > 0) return
  historyLoading.value = true
  try {
    const response = await listSlotHistory(props.shipmentId, { slotType: 'delivery', limit: 20 })
    history.value = response.items
  } catch {
    history.value = []
  } finally {
    historyLoading.value = false
  }
}

watch(
  () => [props.shipmentId, props.slotContext] as const,
  () => {
    history.value = []
    showHistory.value = false
    loadSlots()
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
      <h3 class="card-title">{{ $t('slot.title') }}</h3>
    </template>

    <div v-if="loading" class="loading-block">{{ $t('common.loading') }}</div>
    <p v-else-if="unavailable" class="slot-muted">{{ $t('slot.intelligenceUnavailable') }}</p>
    <template v-else>
      <dl class="slot-grid">
        <dt>{{ $t('slot.deliverySlot') }}</dt>
        <dd>{{ windowLabel(delivery) }}</dd>

        <template v-if="delivery && delivery.windowStatus === 'available'">
          <dt>{{ $t('slot.status') }}</dt>
          <dd>{{ delivery.slotStatus ? $t(`slot.lifecycle.${delivery.slotStatus}`) : '—' }}</dd>

          <dt>{{ $t('slot.arrivalProjection') }}</dt>
          <dd>{{ $t(`slot.projection.${delivery.arrivalProjection}`) }}</dd>

          <dt>{{ $t('slot.source') }}</dt>
          <dd>
            <span v-if="delivery.sourceType">{{ $t(`slot.source.${delivery.sourceType}`) }}</span>
            <span v-if="delivery.provider"> · {{ delivery.provider }}</span>
          </dd>
        </template>
      </dl>

      <div v-if="delivery && delivery.windowStatus === 'available'" class="slot-actions">
        <UiButton size="sm" variant="secondary" @click="showHistory = !showHistory">
          {{ showHistory ? $t('slot.hideHistory') : $t('slot.history') }}
        </UiButton>
      </div>

      <div v-if="showHistory" class="slot-history">
        <div v-if="historyLoading" class="loading-block">{{ $t('common.loading') }}</div>
        <UiEmptyState v-else-if="!history.length" :title="$t('slot.noHistory')" />
        <table v-else class="slot-history__table">
          <thead>
            <tr>
              <th>{{ $t('slot.window') }}</th>
              <th>{{ $t('slot.status') }}</th>
              <th>{{ $t('slot.source') }}</th>
              <th>{{ $t('slot.rescheduledAt') }}</th>
            </tr>
          </thead>
          <tbody>
            <tr v-for="item in history" :key="item.id">
              <td>{{ formatSlotWindow(item.windowStart, item.windowEnd) }}</td>
              <td>{{ $t(`slot.lifecycle.${item.slotStatus}`) }}</td>
              <td>{{ $t(`slot.source.${item.sourceType}`) }}</td>
              <td>{{ item.sourceObservedAt }}</td>
            </tr>
          </tbody>
        </table>
      </div>
    </template>
  </UiCard>
</template>

<style scoped>
.slot-grid {
  display: grid;
  grid-template-columns: minmax(140px, 220px) 1fr;
  gap: 0.35rem 1rem;
  margin: 0;
}
.slot-grid dt {
  color: var(--color-text-muted, #666);
}
.slot-grid dd {
  margin: 0;
}
.slot-muted {
  color: var(--color-text-muted, #666);
}
.slot-actions {
  margin-top: 0.75rem;
}
.slot-history {
  margin-top: 1rem;
}
.slot-history__table {
  width: 100%;
  border-collapse: collapse;
  font-size: 0.875rem;
}
.slot-history__table th,
.slot-history__table td {
  border-bottom: 1px solid var(--color-border, #e5e7eb);
  padding: 0.35rem 0.5rem;
  text-align: left;
}
</style>
