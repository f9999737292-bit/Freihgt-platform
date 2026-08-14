<script setup lang="ts">
import type { ControlTowerEvent } from '~/types/controlTower'
import { useSlaCountdown } from '~/composables/useSlaCountdown'

const props = defineProps<{
  event: ControlTowerEvent
}>()

const { t } = useI18n()
const { label: slaCountdownLabel } = useSlaCountdown(() => props.event.sla ?? null)

function priorityClass(priority?: string) {
  return priority ? `exception-badges__priority--${priority}` : ''
}

function slaClass(status?: string) {
  return status ? `exception-badges__sla--${status}` : ''
}

function escalationClass(level?: string) {
  return level && level !== 'none' ? `exception-badges__escalation--${level}` : ''
}

const isHighVisibility = computed(
  () =>
    props.event.priority === 'p1' ||
    props.event.sla?.status === 'breached' ||
    props.event.escalation?.level === 'level_3',
)
</script>

<template>
  <div class="exception-badges" :class="{ 'exception-badges--critical': isHighVisibility }">
    <span
      v-if="event.priority"
      class="exception-badges__priority"
      :class="priorityClass(event.priority)"
      :aria-label="$t('controlTower.exceptions.priority')"
    >
      {{ $t(`controlTower.exceptions.priorities.${event.priority}`) }}
    </span>
    <span
      v-if="event.sla?.status"
      class="exception-badges__sla"
      :class="slaClass(event.sla.status)"
    >
      {{ $t(`controlTower.exceptions.slaStatus.${event.sla.status}`) }}
      <span v-if="slaCountdownLabel" class="exception-badges__countdown">· {{ slaCountdownLabel }}</span>
    </span>
    <span
      v-if="event.escalation?.level && event.escalation.level !== 'none'"
      class="exception-badges__escalation"
      :class="escalationClass(event.escalation.level)"
    >
      {{ $t(`controlTower.exceptions.escalation.${event.escalation.level}`) }}
    </span>
    <span v-if="event.exceptionCategory" class="exception-badges__category">
      {{ $t(`controlTower.exceptions.categories.${event.exceptionCategory}`) }}
    </span>
    <span
      v-if="event.businessImpact && event.businessImpact !== 'none'"
      class="exception-badges__impact"
    >
      {{ $t('controlTower.exceptions.businessImpactLabel') }}:
      {{ $t(`controlTower.exceptions.businessImpact.${event.businessImpact}`) }}
    </span>
  </div>
</template>

<style scoped>
.exception-badges {
  display: flex;
  flex-wrap: wrap;
  gap: 0.35rem;
  margin-top: 0.5rem;
}

.exception-badges--critical {
  padding: 0.35rem 0.45rem;
  border-radius: var(--radius-sm);
  border: 1px solid color-mix(in srgb, var(--color-danger) 35%, var(--color-border));
  background: color-mix(in srgb, var(--color-danger) 6%, var(--color-surface));
}

.exception-badges__priority,
.exception-badges__sla,
.exception-badges__escalation,
.exception-badges__category,
.exception-badges__impact {
  font-size: 0.6875rem;
  font-weight: 700;
  text-transform: uppercase;
  letter-spacing: 0.03em;
  padding: 0.15rem 0.4rem;
  border-radius: var(--radius-sm);
  background: color-mix(in srgb, var(--color-border) 35%, var(--color-surface));
}

.exception-badges__priority--p1 {
  background: color-mix(in srgb, var(--color-danger) 14%, var(--color-surface));
  color: var(--color-danger);
}

.exception-badges__priority--p2 {
  background: color-mix(in srgb, var(--color-warning) 14%, var(--color-surface));
  color: var(--color-warning);
}

.exception-badges__sla--warning {
  color: var(--color-warning);
}

.exception-badges__sla--breached {
  color: var(--color-danger);
}

.exception-badges__escalation--level_2,
.exception-badges__escalation--level_3 {
  color: var(--color-danger);
}

.exception-badges__countdown {
  text-transform: none;
  font-weight: 600;
}
</style>
