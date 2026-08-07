<script setup lang="ts">
import type { ControlTowerKpiMetric } from '~/types/controlTower'

defineProps<{
  metric: ControlTowerKpiMetric
}>()
</script>

<template>
  <article class="metric-card" :class="`metric-card--${metric.tone}`">
    <header class="metric-card__header">
      <h3 class="metric-card__title">{{ $t(metric.titleKey) }}</h3>
      <span class="metric-card__indicator" aria-hidden="true" />
    </header>
    <p class="metric-card__value">{{ metric.value }}</p>
    <p v-if="metric.percent !== undefined" class="metric-card__percent">
      {{ metric.percent }}%
    </p>
    <p class="metric-card__description">{{ $t(metric.descriptionKey) }}</p>
  </article>
</template>

<style scoped>
.metric-card {
  display: flex;
  flex-direction: column;
  gap: 0.35rem;
  padding: 1.125rem 1.25rem;
  background: var(--color-surface);
  border: 1px solid var(--color-border);
  border-radius: var(--radius-lg);
  box-shadow: var(--shadow-sm);
  min-height: 132px;
}

.metric-card__header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 0.5rem;
}

.metric-card__title {
  margin: 0;
  font-size: 0.8125rem;
  font-weight: 600;
  color: var(--color-text-muted);
}

.metric-card__indicator {
  width: 0.5rem;
  height: 0.5rem;
  border-radius: 50%;
  background: var(--color-text-muted);
}

.metric-card--ok .metric-card__indicator {
  background: var(--color-success);
}

.metric-card--warning .metric-card__indicator {
  background: var(--color-warning);
}

.metric-card--danger .metric-card__indicator {
  background: var(--color-danger);
}

.metric-card--neutral .metric-card__indicator {
  background: var(--color-info);
}

.metric-card__value {
  margin: 0;
  font-size: 1.75rem;
  font-weight: 700;
  line-height: 1.1;
}

.metric-card__percent {
  margin: 0;
  font-size: 0.8125rem;
  color: var(--color-text-muted);
}

.metric-card__description {
  margin: 0;
  font-size: 0.75rem;
  color: var(--color-text-muted);
}
</style>
