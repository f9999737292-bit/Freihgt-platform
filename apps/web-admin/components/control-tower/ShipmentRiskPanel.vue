<script setup lang="ts">
import { formatShipmentDate } from '~/types/shipment'
import type { ControlTowerShipmentRisk } from '~/types/controlTower'

const props = defineProps<{
  risks: ControlTowerShipmentRisk[]
  loading?: boolean
  canAcknowledge?: boolean
  canMitigate?: boolean
  acknowledgingRiskId?: string | null
  riskActionId?: string | null
}>()

const emit = defineEmits<{
  acknowledge: [riskId: string]
  mitigate: [risk: ControlTowerShipmentRisk]
}>()

const { t, te } = useI18n()

function levelClass(level: string) {
  return `shipment-risks__level--${level}`
}

function topSignals(risk: ControlTowerShipmentRisk) {
  return [...(risk.signals ?? [])].sort((a, b) => b.weight - a.weight).slice(0, 3)
}

function signalText(signal: ControlTowerShipmentRisk['signals'][number]): string {
  const key = signal.explanationKey
  if (te(key)) {
    return t(key, signal.value ?? {})
  }
  return signal.signalCode
}

function isBusy(riskId: string): boolean {
  return Boolean(props.acknowledgingRiskId || props.riskActionId) &&
    (props.acknowledgingRiskId === riskId || props.riskActionId === riskId)
}

function canAck(risk: ControlTowerShipmentRisk): boolean {
  return props.canAcknowledge === true && ['active', 'acknowledged'].includes(risk.status)
}

function canMit(risk: ControlTowerShipmentRisk): boolean {
  return props.canMitigate === true && !['cleared', 'materialized'].includes(risk.status)
}
</script>

<template>
  <div class="shipment-risks">
    <div v-if="loading" class="shipment-risks__empty">{{ $t('common.loading') }}</div>
    <div v-else-if="risks.length === 0" class="shipment-risks__empty">
      {{ $t('controlTower.risk.empty') }}
    </div>
    <ul v-else class="shipment-risks__list">
      <li v-for="risk in risks" :key="risk.riskId" class="shipment-risks__item">
        <div class="shipment-risks__header">
          <strong>{{ risk.shipmentNumber }}</strong>
          <div class="shipment-risks__badges">
            <span class="shipment-risks__level" :class="levelClass(risk.level)">
              {{ $t(`controlTower.risk.levels.${risk.level}`) }} · {{ risk.score }}
            </span>
            <span class="shipment-risks__status">
              {{ $t(`controlTower.risk.status.${risk.status}`) }}
            </span>
          </div>
        </div>

        <p class="shipment-risks__type">
          {{ $t(`controlTower.risk.types.${risk.predictedExceptionType}`) }}
        </p>

        <p v-if="risk.threatenedDeadlineAt" class="shipment-risks__deadline">
          {{ $t('controlTower.risk.deadline') }}:
          {{ formatShipmentDate(risk.threatenedDeadlineAt) }}
        </p>

        <div v-if="topSignals(risk).length" class="shipment-risks__reasons">
          <span class="shipment-risks__reasons-label">{{ $t('controlTower.risk.reasons') }}</span>
          <ul>
            <li v-for="signal in topSignals(risk)" :key="signal.signalCode">
              {{ signalText(signal) }}
            </li>
          </ul>
        </div>

        <div v-if="risk.actualEventId" class="shipment-risks__materialized">
          {{ $t('controlTower.risk.materializedLink', { eventId: risk.actualEventId }) }}
        </div>

        <div class="shipment-risks__actions">
          <button
            v-if="canAck(risk)"
            type="button"
            class="shipment-risks__btn"
            :disabled="isBusy(risk.riskId)"
            @click="emit('acknowledge', risk.riskId)"
          >
            {{ $t('controlTower.risk.acknowledge') }}
          </button>
          <button
            v-if="canMit(risk)"
            type="button"
            class="shipment-risks__btn shipment-risks__btn--primary"
            :disabled="isBusy(risk.riskId)"
            @click="emit('mitigate', risk)"
          >
            {{ $t('controlTower.risk.startMitigation') }}
          </button>
        </div>
      </li>
    </ul>
  </div>
</template>

<style scoped>
.shipment-risks__empty {
  padding: 1rem 0;
  color: var(--color-text-muted);
  font-size: 0.875rem;
}

.shipment-risks__list {
  list-style: none;
  margin: 0;
  padding: 0;
  display: flex;
  flex-direction: column;
  gap: 1rem;
}

.shipment-risks__item {
  border: 1px solid var(--color-border);
  border-radius: var(--radius-md);
  padding: 0.875rem 1rem;
}

.shipment-risks__header {
  display: flex;
  justify-content: space-between;
  gap: 0.75rem;
  align-items: flex-start;
}

.shipment-risks__badges {
  display: flex;
  flex-direction: column;
  align-items: flex-end;
  gap: 0.25rem;
}

.shipment-risks__level {
  font-size: 0.75rem;
  font-weight: 700;
  padding: 0.125rem 0.5rem;
  border-radius: 999px;
}

.shipment-risks__level--critical {
  background: color-mix(in srgb, var(--color-danger) 15%, white);
  color: var(--color-danger);
}

.shipment-risks__level--high {
  background: color-mix(in srgb, var(--color-warning) 18%, white);
  color: var(--color-warning);
}

.shipment-risks__level--medium {
  background: color-mix(in srgb, var(--color-info) 12%, white);
  color: var(--color-info);
}

.shipment-risks__level--low {
  background: var(--color-surface-muted);
  color: var(--color-text-muted);
}

.shipment-risks__status {
  font-size: 0.75rem;
  color: var(--color-text-muted);
}

.shipment-risks__type {
  margin: 0.5rem 0 0;
  font-weight: 600;
  font-size: 0.875rem;
}

.shipment-risks__deadline,
.shipment-risks__materialized {
  margin: 0.375rem 0 0;
  font-size: 0.8125rem;
  color: var(--color-text-muted);
}

.shipment-risks__reasons {
  margin-top: 0.625rem;
  font-size: 0.8125rem;
}

.shipment-risks__reasons-label {
  font-weight: 600;
}

.shipment-risks__reasons ul {
  margin: 0.25rem 0 0;
  padding-left: 1.125rem;
}

.shipment-risks__actions {
  display: flex;
  gap: 0.5rem;
  margin-top: 0.75rem;
}

.shipment-risks__btn {
  border: 1px solid var(--color-border);
  background: white;
  border-radius: var(--radius-sm);
  padding: 0.375rem 0.75rem;
  font-size: 0.8125rem;
  cursor: pointer;
}

.shipment-risks__btn--primary {
  border-color: var(--color-primary);
  color: var(--color-primary);
}

.shipment-risks__btn:disabled {
  opacity: 0.6;
  cursor: not-allowed;
}
</style>
