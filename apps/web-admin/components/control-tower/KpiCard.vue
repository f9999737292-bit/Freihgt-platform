<script setup lang="ts">
import type { ControlTowerBadgeTone } from '~/types/controlTower'

defineProps<{
  title: string
  value: string | number
  description: string
  badgeLabel: string
  badgeTone: ControlTowerBadgeTone
  link: string
  unavailable?: boolean
}>()
</script>

<template>
  <NuxtLink :to="link" class="kpi-card" :class="{ 'kpi-card--unavailable': unavailable }">
    <div class="kpi-card__header">
      <span class="kpi-card__title">{{ title }}</span>
      <ControlTowerStatusBadge :label="badgeLabel" :tone="badgeTone" />
    </div>
    <strong class="kpi-card__value">{{ value }}</strong>
    <p class="kpi-card__description">{{ description }}</p>
    <p v-if="unavailable" class="kpi-card__warning">{{ $t('controlTower.apiUnavailable') }}</p>
  </NuxtLink>
</template>

<style scoped>
.kpi-card {
  display: flex;
  flex-direction: column;
  gap: 0.5rem;
  padding: 1.25rem;
  background: var(--color-surface);
  border: 1px solid var(--color-border);
  border-radius: var(--radius-lg);
  text-decoration: none;
  color: inherit;
  box-shadow: var(--shadow-sm);
  min-height: 140px;
}

.kpi-card:hover {
  border-color: var(--color-primary);
  text-decoration: none;
}

.kpi-card--unavailable {
  opacity: 0.85;
}

.kpi-card__header {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 0.5rem;
}

.kpi-card__title {
  color: var(--color-text-muted);
  font-size: 0.875rem;
  font-weight: 600;
}

.kpi-card__value {
  font-size: 1.75rem;
  line-height: 1.1;
}

.kpi-card__description {
  margin: 0;
  color: var(--color-text-muted);
  font-size: 0.8125rem;
}

.kpi-card__warning {
  margin: 0;
  color: #b45309;
  font-size: 0.75rem;
}
</style>
