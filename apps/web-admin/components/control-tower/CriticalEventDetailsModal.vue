<script setup lang="ts">
import { formatShipmentDate } from '~/types/shipment'
import type { ControlTowerEvent, ControlTowerEventAction } from '~/types/controlTower'

const props = defineProps<{
  open: boolean
  event: ControlTowerEvent | null
  actions: ControlTowerEventAction[]
  loading?: boolean
}>()

defineEmits<{ close: [] }>()

const { t } = useI18n()

function actionLabel(actionType: string): string {
  return t(`controlTower.events.actionTypes.${actionType}`)
}

function metadataLabel(action: ControlTowerEventAction): string {
  const meta = action.metadata ?? {}
  if (action.actionType === 'assigned' || action.actionType === 'reassigned') {
    const userId = String(meta.assignedToUserId ?? '')
    if (userId) return t('controlTower.events.assignedToUser', { userId })
  }
  if (action.actionType === 'resolved') {
    const code = String(meta.resolutionCode ?? '')
    const parts = [code ? t(`controlTower.events.resolutionCodes.${code}`) : '']
    const comment = String(meta.comment ?? '').trim()
    if (comment) parts.push(comment)
    return parts.filter(Boolean).join(' · ')
  }
  return ''
}
</script>

<template>
  <UiModal
    :open="open"
    :title="$t('controlTower.events.actionHistory')"
    @close="$emit('close')"
  >
    <div v-if="event" class="event-details">
      <p class="event-details__meta">
        <strong>{{ event.shipmentNumber }}</strong>
        ·
        {{ $t(`controlTower.events.types.${event.type}`) }}
      </p>
      <p class="event-details__status">
        {{ $t('common.status') }}:
        <span class="event-details__status-badge">
          {{ $t(`controlTower.events.status.${event.status ?? 'open'}`) }}
        </span>
      </p>
    </div>

    <div v-if="loading" class="event-details__empty">{{ $t('common.loading') }}</div>
    <div v-else-if="actions.length === 0" class="event-details__empty">
      {{ $t('controlTower.events.actionHistoryEmpty') }}
    </div>
    <ol v-else class="event-details__timeline">
      <li v-for="(action, index) in actions" :key="`${action.actionType}-${action.occurredAt}-${index}`">
        <div class="event-details__action">{{ actionLabel(action.actionType) }}</div>
        <div class="event-details__actor">
          {{ $t('controlTower.events.acknowledgedByUser', { userId: action.actorUserId }) }}
        </div>
        <div class="event-details__time">{{ formatShipmentDate(action.occurredAt) }}</div>
        <div v-if="metadataLabel(action)" class="event-details__meta-line">
          {{ metadataLabel(action) }}
        </div>
      </li>
    </ol>
  </UiModal>
</template>

<style scoped>
.event-details__meta,
.event-details__status {
  margin: 0 0 0.75rem;
  font-size: 0.875rem;
}

.event-details__status-badge {
  font-weight: 700;
  text-transform: uppercase;
  letter-spacing: 0.04em;
  color: var(--color-info);
}

.event-details__empty {
  padding: 1rem 0;
  text-align: center;
  color: var(--color-text-muted);
  font-size: 0.875rem;
}

.event-details__timeline {
  list-style: none;
  margin: 0;
  padding: 0;
  display: flex;
  flex-direction: column;
  gap: 0.875rem;
}

.event-details__timeline li {
  padding-left: 0.875rem;
  border-left: 2px solid var(--color-border);
}

.event-details__action {
  font-weight: 600;
  font-size: 0.875rem;
}

.event-details__actor,
.event-details__time,
.event-details__meta-line {
  font-size: 0.8125rem;
  color: var(--color-text-muted);
  margin-top: 0.15rem;
}
</style>
