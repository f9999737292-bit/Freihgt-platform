<script setup lang="ts">
import type { ShipmentEventsResponse } from '~/types/shipmentEvent'

defineProps<{
  response: ShipmentEventsResponse | null
  loading?: boolean
}>()
</script>

<template>
  <div class="timeline">
    <div v-if="loading" class="loading-block">{{ $t('shipmentEvents.loading') }}</div>
    <UiEmptyState v-else-if="!response || response.timeline.total === 0" :title="$t('shipmentEvents.empty')" />
    <template v-else>
      <ShipmentEventsShipmentTimelineItem
        v-for="event in response.timeline.items"
        :key="event.id"
        :event="event"
      />
      <p class="timeline-meta">
        {{ $t('shipmentEvents.paginationSummary', {
          page: response.timeline.page,
          total: response.timeline.total,
        }) }}
      </p>
    </template>
  </div>
</template>

<style scoped>
.timeline {
  display: flex;
  flex-direction: column;
  gap: 0.5rem;
}

.loading-block {
  padding: 1rem 0;
  color: var(--color-text-muted);
}

.timeline-meta {
  margin: 0.5rem 0 0;
  font-size: 0.8125rem;
  color: var(--color-text-muted);
}
</style>
