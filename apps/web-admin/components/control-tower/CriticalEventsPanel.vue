<script setup lang="ts">
import { formatShipmentDate } from '~/types/shipment'
import type { ControlTowerEvent, ControlTowerEventWorkflowStatus } from '~/types/controlTower'

const props = defineProps<{
  events: ControlTowerEvent[]
  loading?: boolean
  canAcknowledge?: boolean
  canAssign?: boolean
  canResolve?: boolean
  canManageException?: boolean
  acknowledgingEventId?: string | null
  workflowActionEventId?: string | null
}>()

const emit = defineEmits<{
  acknowledge: [eventId: string]
  assign: [eventId: string]
  resolve: [eventId: string]
  reopen: [eventId: string]
  editException: [event: ControlTowerEvent]
  showDetails: [event: ControlTowerEvent]
}>()

const { t } = useI18n()

function severityClass(severity: string) {
  return `critical-events__severity--${severity.toLowerCase()}`
}

function eventTypeKey(type: string) {
  return `controlTower.events.types.${type}`
}

function eventStatus(event: ControlTowerEvent): ControlTowerEventWorkflowStatus {
  return event.status ?? 'open'
}

function statusClass(status: ControlTowerEventWorkflowStatus) {
  return `critical-events__status--${status}`
}

function isBusy(eventId: string): boolean {
  return Boolean(props.acknowledgingEventId || props.workflowActionEventId) &&
    (props.acknowledgingEventId === eventId || props.workflowActionEventId === eventId)
}

function isAnyActionInProgress(): boolean {
  return Boolean(props.acknowledgingEventId || props.workflowActionEventId)
}

function acknowledgedByLabel(event: ControlTowerEvent): string {
  const ack = event.acknowledgement
  if (!ack) return ''
  const name = ack.acknowledgedBy.displayName?.trim()
  if (name) return name
  return t('controlTower.events.acknowledgedByUser', { userId: ack.acknowledgedBy.userId })
}

function assignedToLabel(event: ControlTowerEvent): string {
  const assignment = event.assignment
  if (!assignment) return ''
  return t('controlTower.events.assignedToUser', { userId: assignment.assignedToUserId })
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
          <div class="critical-events__badges">
            <span class="critical-events__status" :class="statusClass(eventStatus(event))">
              {{ $t(`controlTower.events.status.${eventStatus(event)}`) }}
            </span>
            <span class="critical-events__severity" :class="severityClass(event.severity)">
              {{ event.severity }}
            </span>
          </div>
        </div>
        <p class="critical-events__type">{{ $t(eventTypeKey(event.type)) }}</p>
        <p class="critical-events__time">{{ formatShipmentDate(event.occurredAt) }}</p>
        <p v-if="event.descriptionKey" class="critical-events__description">
          {{ $t(event.descriptionKey) }}
        </p>
        <p v-else-if="event.description" class="critical-events__description">
          {{ event.description }}
        </p>

        <ControlTowerCriticalEventExceptionBadges :event="event" />

        <div v-if="event.acknowledgement" class="critical-events__info-block">
          <span class="critical-events__info-label">{{ $t('controlTower.events.acknowledged') }}</span>
          <span class="critical-events__info-meta">
            {{ formatShipmentDate(event.acknowledgement.acknowledgedAt) }}
            ·
            {{ acknowledgedByLabel(event) }}
          </span>
        </div>

        <div v-if="event.assignment" class="critical-events__info-block">
          <span class="critical-events__info-label">{{ $t('controlTower.events.assignedTo') }}</span>
          <span class="critical-events__info-meta">
            {{ assignedToLabel(event) }}
            ·
            {{ formatShipmentDate(event.assignment.assignedAt) }}
          </span>
        </div>

        <div v-if="event.resolution" class="critical-events__info-block">
          <span class="critical-events__info-label">{{ $t('controlTower.events.resolution') }}</span>
          <span class="critical-events__info-meta">
            {{ $t(`controlTower.events.resolutionCodes.${event.resolution.resolutionCode}`) }}
            ·
            {{ formatShipmentDate(event.resolution.resolvedAt) }}
          </span>
        </div>

        <div class="critical-events__actions">
          <UiButton
            v-if="canManageException && eventStatus(event) !== 'resolved'"
            size="sm"
            variant="ghost"
            @click="emit('editException', event)"
          >
            {{ $t('controlTower.exceptions.edit') }}
          </UiButton>
          <UiButton
            size="sm"
            variant="ghost"
            @click="emit('showDetails', event)"
          >
            {{ $t('controlTower.events.actionHistory') }}
          </UiButton>
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
            v-if="canAcknowledge && eventStatus(event) === 'open'"
            size="sm"
            variant="secondary"
            class="critical-events__action-button"
            :loading="isBusy(event.id)"
            :disabled="isAnyActionInProgress()"
            data-testid="critical-event-acknowledge"
            @click="emit('acknowledge', event.id)"
          >
            {{ $t('controlTower.events.acknowledge') }}
          </UiButton>
          <UiButton
            v-if="canAssign && eventStatus(event) === 'acknowledged'"
            size="sm"
            variant="secondary"
            class="critical-events__action-button"
            :loading="isBusy(event.id)"
            :disabled="isAnyActionInProgress()"
            @click="emit('assign', event.id)"
          >
            {{ $t('controlTower.events.assign') }}
          </UiButton>
          <template v-if="canAssign && eventStatus(event) === 'assigned'">
            <UiButton
              size="sm"
              variant="secondary"
              class="critical-events__action-button"
              :loading="isBusy(event.id)"
              :disabled="isAnyActionInProgress()"
              @click="emit('assign', event.id)"
            >
              {{ $t('controlTower.events.reassign') }}
            </UiButton>
            <UiButton
              v-if="canResolve"
              size="sm"
              variant="primary"
              class="critical-events__action-button"
              :loading="isBusy(event.id)"
              :disabled="isAnyActionInProgress()"
              @click="emit('resolve', event.id)"
            >
              {{ $t('controlTower.events.resolve') }}
            </UiButton>
          </template>
          <UiButton
            v-if="canAssign && eventStatus(event) === 'resolved'"
            size="sm"
            variant="secondary"
            class="critical-events__action-button"
            :loading="isBusy(event.id)"
            :disabled="isAnyActionInProgress()"
            @click="emit('reopen', event.id)"
          >
            {{ $t('controlTower.events.reopen') }}
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

.critical-events__badges {
  display: flex;
  align-items: center;
  gap: 0.35rem;
}

.critical-events__status {
  font-size: 0.6875rem;
  font-weight: 700;
  text-transform: uppercase;
  letter-spacing: 0.04em;
  padding: 0.15rem 0.4rem;
  border-radius: var(--radius-sm);
  background: color-mix(in srgb, var(--color-info) 10%, var(--color-surface));
  color: var(--color-info);
}

.critical-events__status--open {
  background: color-mix(in srgb, var(--color-warning) 12%, var(--color-surface));
  color: var(--color-warning);
}

.critical-events__status--assigned {
  background: color-mix(in srgb, var(--color-info) 12%, var(--color-surface));
  color: var(--color-info);
}

.critical-events__status--resolved {
  background: color-mix(in srgb, var(--color-success, #16a34a) 12%, var(--color-surface));
  color: var(--color-success, #16a34a);
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

.critical-events__info-block {
  display: flex;
  flex-direction: column;
  gap: 0.15rem;
  margin-top: 0.5rem;
  padding: 0.5rem 0.625rem;
  border-radius: var(--radius-sm);
  background: color-mix(in srgb, var(--color-info) 6%, var(--color-surface));
  border: 1px solid color-mix(in srgb, var(--color-info) 18%, var(--color-border));
}

.critical-events__info-label {
  font-size: 0.6875rem;
  font-weight: 700;
  text-transform: uppercase;
  letter-spacing: 0.04em;
  color: var(--color-info);
}

.critical-events__info-meta {
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

.critical-events__action-button {
  margin-left: auto;
}
</style>
