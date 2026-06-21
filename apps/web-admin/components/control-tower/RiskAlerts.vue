<script setup lang="ts">
import type { ControlTowerRiskAlert } from '~/types/controlTower'

defineProps<{
  alerts: ControlTowerRiskAlert[]
}>()

const { t } = useI18n()
</script>

<template>
  <div v-if="alerts.length === 0" class="risk-alerts risk-alerts--ok">
    <span>{{ $t('controlTower.risks.none') }}</span>
  </div>
  <ul v-else class="risk-alerts">
    <li
      v-for="alert in alerts"
      :key="alert.key"
      class="risk-alerts__item"
      :class="`risk-alerts__item--${alert.severity}`"
    >
      {{ t(alert.messageKey, { count: alert.count ?? 0 }) }}
    </li>
  </ul>
</template>

<style scoped>
.risk-alerts {
  margin: 0;
  padding: 0;
  list-style: none;
  display: grid;
  gap: 0.5rem;
}

.risk-alerts--ok {
  padding: 0.875rem 1rem;
  border-radius: var(--radius-md);
  background: #ecfdf5;
  color: #166534;
  font-size: 0.875rem;
}

.risk-alerts__item {
  padding: 0.75rem 1rem;
  border-radius: var(--radius-md);
  font-size: 0.875rem;
}

.risk-alerts__item--warning {
  background: #fffbeb;
  color: #92400e;
  border: 1px solid #fde68a;
}

.risk-alerts__item--danger {
  background: #fef2f2;
  color: #991b1b;
  border: 1px solid #fecaca;
}
</style>
