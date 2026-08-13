<script setup lang="ts">
import { formatShipmentDate } from '~/types/shipment'
import type { ControlTowerEvent } from '~/types/controlTower'

const props = defineProps<{
  events: ControlTowerEvent[]
  loading?: boolean
  canAcknowledge?: boolean
  acknowledgingEventId?: string | null
}>()

const emit = defineEmits<{
  acknowledge: [eventId: string]
}>()

const { t } = useI18n()

function severityClass(severity: string) {
  return `critical-events__severity--${severity.toLowerCase()}`
}

function eventTypeKey(type: string) {
  return `controlTower.events.types.${type}`
}

function isAcknowledged(event: ControlTowerEvent): boolean {
  return Boolean(event.acknowledgement)
}

function isAcknowledging(eventId: string): boolean {
  return props.acknowledgingEventId === eventId
}

function acknowledgedByLabel(event: ControlTowerEvent): string {
  const ack = event.acknowledgement
  if (!ack) return ''
  const name = ack.acknowledgedBy.displayName?.trim()
  if (name) return name
  return t('controlTower.events.acknowledgedByUser', { userId: ack.acknowledgedBy.userId })
}

function onAcknowledge(eventId: string) {
  emit('acknowledge', eventId)
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

        <div
          v-if="isAcknowledged(event)"
          class="critical-events__ack"
          data-testid="critical-event-acknowledged"
        >
          <span class="critical-events__ack-badge">{{ $t('controlTower.events.acknowledged') }}</span>
          <span class="critical-events__ack-meta">
            {{ formatShipmentDate(event.acknowledgement!.acknowledgedAt) }}
            ·
            {{ acknowledgedByLabel(event) }}
          </span>
        </div>

        <div class="critical-events__actions">
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
          <UiButton
            v-if="canAcknowledge && !isAcknowledged(event)"
            size="sm"
            variant="secondary"
            class="critical-events__ack-button"
            :loading="isAcknowledging(event.id)"
            :disabled="Boolean(acknowledgingEventId)"
            data-testid="critical-event-acknowledge"
            @click="onAcknowledge(event.id)"
          >
            {{ $t('controlTower.events.acknowledge') }}
          </UiButton>
        </div>
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

.critical-events__ack {
  display: flex;
  flex-direction: column;
  gap: 0.2rem;
  margin-top: 0.5rem;
  padding: 0.5rem 0.625rem;
  border-radius: var(--radius-sm);
  background: color-mix(in srgb, var(--color-info) 8%, var(--color-surface));
  border: 1px solid color-mix(in srgb, var(--color-info) 25%, var(--color-border));
}

.critical-events__ack-badge {
  font-size: 0.6875rem;
  font-weight: 700;
  text-transform: uppercase;
  letter-spacing: 0.04em;
  color: var(--color-info);
}

.critical-events__ack-meta {
  font-size: 0.75rem;
  color: var(--color-text-muted);
}

.critical-events__actions {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  gap: 0.5rem 0.75rem;
  margin-top: 0.5rem;
}

.critical-events__link {
  font-size: 0.8125rem;
}

.critical-events__ack-button {
  margin-left: auto;
}
</style>
