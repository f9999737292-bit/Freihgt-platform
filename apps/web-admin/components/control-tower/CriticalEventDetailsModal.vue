<script setup lang="ts">
import { formatShipmentDate } from '~/types/shipment'
import type { ControlTowerEvent, ControlTowerEventAction } from '~/types/controlTower'
import { useSlaCountdown } from '~/composables/useSlaCountdown'

const props = defineProps<{
  open: boolean
  event: ControlTowerEvent | null
  actions: ControlTowerEventAction[]
  loading?: boolean
}>()

defineEmits<{ close: [] }>()

const { t } = useI18n()
const { label: slaCountdownLabel } = useSlaCountdown(() => props.event?.sla ?? null)

function actionLabel(actionType: string): string {
  const key = `controlTower.events.actionTypes.${actionType}`
  const translated = t(key)
  return translated === key ? actionType : translated
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
  if (action.actionType === 'exception_updated') {
    const parts: string[] = []
    if (meta.priority) parts.push(t(`controlTower.exceptions.priorities.${meta.priority}`))
    if (meta.category) parts.push(t(`controlTower.exceptions.categories.${meta.category}`))
    if (meta.businessImpact) {
      parts.push(t(`controlTower.exceptions.businessImpact.${meta.businessImpact}`))
    }
    return parts.join(' · ')
  }
  if (action.actionType.endsWith('_sla_breached')) {
    const phase = String(meta.phase ?? '')
    if (phase) return t(`controlTower.exceptions.slaPhase.${phase}`)
  }
  if (action.actionType === 'escalation_changed') {
    const level = String(meta.level ?? meta.escalationLevel ?? '')
    if (level) return t(`controlTower.exceptions.escalation.${level}`)
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

      <div class="event-details__section">
        <h3>{{ $t('controlTower.exceptions.detailsTitle') }}</h3>
        <dl class="event-details__grid">
          <div v-if="event.priority">
            <dt>{{ $t('controlTower.exceptions.priority') }}</dt>
            <dd>{{ $t(`controlTower.exceptions.priorities.${event.priority}`) }}</dd>
          </div>
          <div v-if="event.exceptionCategory">
            <dt>{{ $t('controlTower.exceptions.category') }}</dt>
            <dd>{{ $t(`controlTower.exceptions.categories.${event.exceptionCategory}`) }}</dd>
          </div>
          <div v-if="event.businessImpact">
            <dt>{{ $t('controlTower.exceptions.businessImpactLabel') }}</dt>
            <dd>{{ $t(`controlTower.exceptions.businessImpact.${event.businessImpact}`) }}</dd>
          </div>
          <div v-if="event.sla">
            <dt>{{ $t('controlTower.exceptions.sla') }}</dt>
            <dd>
              {{ $t(`controlTower.exceptions.slaStatus.${event.sla.status}`) }}
              <span v-if="slaCountdownLabel"> · {{ slaCountdownLabel }}</span>
            </dd>
          </div>
          <div v-if="event.escalation?.level">
            <dt>{{ $t('controlTower.exceptions.escalationLabel') }}</dt>
            <dd>{{ $t(`controlTower.exceptions.escalation.${event.escalation.level}`) }}</dd>
          </div>
        </dl>
      </div>
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

.event-details__section {
  margin-bottom: 1rem;
  padding: 0.75rem;
  border: 1px solid var(--color-border);
  border-radius: var(--radius-md);
  background: color-mix(in srgb, var(--color-info) 4%, var(--color-surface));
}

.event-details__section h3 {
  margin: 0 0 0.5rem;
  font-size: 0.8125rem;
  text-transform: uppercase;
  letter-spacing: 0.04em;
  color: var(--color-text-muted);
}

.event-details__grid {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(140px, 1fr));
  gap: 0.5rem 1rem;
  margin: 0;
}

.event-details__grid dt {
  font-size: 0.6875rem;
  font-weight: 700;
  text-transform: uppercase;
  color: var(--color-text-muted);
}

.event-details__grid dd {
  margin: 0.15rem 0 0;
  font-size: 0.8125rem;
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
