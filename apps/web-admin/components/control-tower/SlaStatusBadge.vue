<script setup lang="ts">
import type { SlaStatus } from '~/types/controlTower'

defineProps<{
  status: SlaStatus
}>()

const { t } = useI18n()

const labelKey = (status: SlaStatus) => `controlTower.sla.${status}`
const tooltipKey = (status: SlaStatus) => `controlTower.slaTooltips.${status}`
</script>

<template>
  <span
    class="sla-status-badge"
    :class="`sla-status-badge--${status.toLowerCase()}`"
    :title="$t(tooltipKey(status))"
  >
    {{ $t(labelKey(status)) }}
  </span>
</template>

<style scoped>
.sla-status-badge {
  display: inline-flex;
  align-items: center;
  padding: 0.125rem 0.5rem;
  border-radius: 999px;
  font-size: 0.75rem;
  font-weight: 600;
  white-space: nowrap;
}

.sla-status-badge--on_time {
  background: color-mix(in srgb, var(--color-success) 15%, white);
  color: var(--color-success);
}

.sla-status-badge--at_risk {
  background: color-mix(in srgb, var(--color-warning) 15%, white);
  color: var(--color-warning);
}

.sla-status-badge--delayed {
  background: color-mix(in srgb, var(--color-warning) 22%, white);
  color: var(--color-warning);
}

.sla-status-badge--critical {
  background: color-mix(in srgb, var(--color-danger) 15%, white);
  color: var(--color-danger);
}

.sla-status-badge--unknown {
  background: color-mix(in srgb, var(--color-text-muted) 12%, white);
  color: var(--color-text-muted);
}
</style>
