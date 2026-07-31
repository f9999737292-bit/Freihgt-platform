<script setup lang="ts">
import type { ShipmentTimelineEvent } from '~/types/shipmentEvent'
import { descriptionCodeToI18nKey, formatEventDateTime, titleCodeToI18nKey } from '~/types/shipmentEvent'

defineProps<{
  event: ShipmentTimelineEvent
}>()

const { t, te } = useI18n()

function localizeCode(code: string, prefix: 'types' | 'categories' | 'sources' | 'severities', value: string) {
  const key = `shipmentEvents.${prefix}.${value}`
  return te(key) ? t(key) : value
}
</script>

<template>
  <article class="timeline-item">
    <div class="timeline-item__marker" :data-severity="event.severity" />
    <div class="timeline-item__body">
      <header class="timeline-item__header">
        <h3 class="timeline-item__title">
          {{ te(titleCodeToI18nKey(event.titleCode)) ? $t(titleCodeToI18nKey(event.titleCode)) : event.type }}
        </h3>
        <time class="timeline-item__time">{{ formatEventDateTime(event.occurredAt) }}</time>
      </header>

      <p v-if="event.descriptionCode && te(descriptionCodeToI18nKey(event.descriptionCode))" class="timeline-item__description">
        {{ $t(descriptionCodeToI18nKey(event.descriptionCode)) }}
      </p>

      <div class="timeline-item__badges">
        <ShipmentEventsShipmentEventBadge kind="category" :value="event.category" />
        <ShipmentEventsShipmentEventBadge kind="source" :value="event.source" />
        <ShipmentEventsShipmentEventBadge kind="severity" :value="event.severity" />
        <span v-if="event.derived" class="derived-badge">{{ $t('shipmentEvents.derived') }}</span>
      </div>

      <ShipmentEventsShipmentEventMetadata v-if="event.metadata && Object.keys(event.metadata).length" :metadata="event.metadata" />

      <div v-if="event.actor?.type" class="timeline-item__actor">
        <span class="label">{{ $t('shipmentEvents.actor') }}:</span>
        <span>{{ localizeCode('', 'sources', event.actor.type) }}</span>
      </div>
    </div>
  </article>
</template>

<style scoped>
.timeline-item {
  display: grid;
  grid-template-columns: 1rem 1fr;
  gap: 0.75rem;
  padding-bottom: 1rem;
}

.timeline-item__marker {
  width: 0.75rem;
  height: 0.75rem;
  margin-top: 0.35rem;
  border-radius: 999px;
  background: var(--color-primary);
}

.timeline-item__marker[data-severity='WARNING'] {
  background: #d97706;
}

.timeline-item__marker[data-severity='CRITICAL'] {
  background: #dc2626;
}

.timeline-item__header {
  display: flex;
  justify-content: space-between;
  gap: 1rem;
  align-items: baseline;
}

.timeline-item__title {
  margin: 0;
  font-size: 0.95rem;
}

.timeline-item__time {
  font-size: 0.8125rem;
  color: var(--color-text-muted);
  white-space: nowrap;
}

.timeline-item__description {
  margin: 0.35rem 0 0.5rem;
  font-size: 0.875rem;
  color: var(--color-text-muted);
}

.timeline-item__badges {
  display: flex;
  flex-wrap: wrap;
  gap: 0.35rem;
}

.derived-badge {
  font-size: 0.75rem;
  padding: 0.125rem 0.5rem;
  border-radius: 999px;
  background: #f3f4f6;
  color: #4b5563;
}

.timeline-item__actor {
  margin-top: 0.5rem;
  font-size: 0.8125rem;
}

.label {
  color: var(--color-text-muted);
  margin-right: 0.25rem;
}
</style>
