<script setup lang="ts">
import { formatShipmentDate } from '~/types/shipment'
import type { ControlTowerEvent } from '~/types/controlTower'

defineProps<{
  events: ControlTowerEvent[]
  loading?: boolean
}>()

const { t } = useI18n()

function severityClass(severity: string) {
  return `critical-events__severity--${severity.toLowerCase()}`
}

function eventTypeKey(type: string) {
  return `controlTower.events.types.${type}`
}
</script>

<template>
  <div class="critical-events">
    <div v-if="loading" class="critical-events__empty">{{ $t('common.loading') }}</div>
    <div v-else-if="events.length === 0" class="critical-events__empty">
      {{ $t('controlTower.events.empty') }}
    </div>
    <ul v-else class="critical-events__list">
      <li v-for="event in events" :key="event.id" class="critical-events__item">
        <div class="critical-events__header">
          <strong>{{ event.shipmentNumber }}</strong>
          <span class="critical-events__severity" :class="severityClass(event.severity)">
            {{ event.severity }}
          </span>
        </div>
        <p class="critical-events__type">{{ $t(eventTypeKey(event.type)) }}</p>
        <p class="critical-events__time">{{ formatShipmentDate(event.occurredAt) }}</p>
        <p v-if="event.descriptionKey" class="critical-events__description">
          {{ $t(event.descriptionKey) }}
        </p>
        <p v-else-if="event.description" class="critical-events__description">
          {{ event.description }}
        </p>
        <NuxtLink
          v-if="event.shipmentId"
          :to="`/shipments/${event.shipmentId}/events`"
          class="critical-events__link"
        >
          {{ $t('controlTower.actions.eventHistory') }}
        </NuxtLink>
        <NuxtLink
          v-if="event.shipmentId"
          :to="`/shipments/${event.shipmentId}`"
          class="critical-events__link"
        >
          {{ $t('controlTower.actions.openShipment') }}
        </NuxtLink>
      </li>
    </ul>
  </div>
</template>

<style scoped>
.critical-events__empty {
  padding: 1.5rem;
  text-align: center;
  color: var(--color-text-muted);
  font-size: 0.875rem;
}

.critical-events__list {
  list-style: none;
  margin: 0;
  padding: 0;
  display: flex;
  flex-direction: column;
  gap: 0.75rem;
}

.critical-events__item {
  padding: 0.875rem 1rem;
  border: 1px solid var(--color-border);
  border-radius: var(--radius-md);
  background: var(--color-surface);
}

.critical-events__header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 0.5rem;
}

.critical-events__severity {
  font-size: 0.6875rem;
  font-weight: 700;
  text-transform: uppercase;
  letter-spacing: 0.04em;
}

.critical-events__severity--info {
  color: var(--color-info);
}

.critical-events__severity--warning {
  color: var(--color-warning);
}

.critical-events__severity--critical {
  color: var(--color-danger);
}

.critical-events__type {
  margin: 0.35rem 0 0;
  font-size: 0.875rem;
  font-weight: 600;
}

.critical-events__time,
.critical-events__description {
  margin: 0.25rem 0 0;
  font-size: 0.8125rem;
  color: var(--color-text-muted);
}

.critical-events__link {
  display: inline-block;
  margin-top: 0.5rem;
  font-size: 0.8125rem;
}
</style>
