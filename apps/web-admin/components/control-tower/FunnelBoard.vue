<script setup lang="ts">
import type { ControlTowerFunnelStep } from '~/types/controlTower'

defineProps<{
  steps: ControlTowerFunnelStep[]
  empty?: boolean
  emptyMessage?: string
}>()
</script>

<template>
  <div v-if="empty" class="funnel-empty">
    <UiEmptyState :title="emptyMessage || $t('common.empty')" />
  </div>
  <div v-else class="funnel-board">
    <div v-for="(step, index) in steps" :key="step.key" class="funnel-board__item">
      <div class="funnel-board__step">
        <span class="funnel-board__count">{{ step.count }}</span>
        <span class="funnel-board__label">{{ $t(step.labelKey) }}</span>
      </div>
      <span v-if="index < steps.length - 1" class="funnel-board__arrow" aria-hidden="true">→</span>
    </div>
  </div>
</template>

<style scoped>
.funnel-board {
  display: flex;
  flex-wrap: wrap;
  gap: 0.75rem;
  align-items: center;
}

.funnel-board__item {
  display: flex;
  align-items: center;
  gap: 0.75rem;
}

.funnel-board__step {
  display: flex;
  flex-direction: column;
  gap: 0.25rem;
  min-width: 120px;
  padding: 0.875rem 1rem;
  border: 1px solid var(--color-border);
  border-radius: var(--radius-md);
  background: #f8fafc;
}

.funnel-board__count {
  font-size: 1.25rem;
  font-weight: 700;
}

.funnel-board__label {
  font-size: 0.8125rem;
  color: var(--color-text-muted);
}

.funnel-board__arrow {
  color: var(--color-text-muted);
  font-size: 1.125rem;
}

.funnel-empty {
  padding: 0.5rem 0;
}
</style>
