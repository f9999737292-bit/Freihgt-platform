<script setup lang="ts">
import type { ControlTowerShipmentStatusRow } from '~/types/controlTower'

defineProps<{
  rows: ControlTowerShipmentStatusRow[]
  loading?: boolean
}>()
</script>

<template>
  <div class="shipment-board">
    <NuxtLink
      v-for="row in rows"
      :key="row.status"
      :to="row.link"
      class="shipment-board__item"
      :class="{ 'shipment-board__item--empty': row.count === 0 }"
    >
      <span class="shipment-board__count">{{ row.count }}</span>
      <span class="shipment-board__status">{{ row.status }}</span>
    </NuxtLink>
  </div>
</template>

<style scoped>
.shipment-board {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(180px, 1fr));
  gap: 0.75rem;
}

.shipment-board__item {
  display: flex;
  flex-direction: column;
  gap: 0.25rem;
  padding: 0.875rem 1rem;
  border: 1px solid var(--color-border);
  border-radius: var(--radius-md);
  background: var(--color-surface);
  text-decoration: none;
  color: inherit;
  transition: border-color 0.15s ease;
}

.shipment-board__item:hover {
  border-color: var(--color-primary);
  text-decoration: none;
}

.shipment-board__item--empty {
  opacity: 0.65;
}

.shipment-board__count {
  font-size: 1.25rem;
  font-weight: 700;
}

.shipment-board__status {
  font-size: 0.75rem;
  color: var(--color-text-muted);
  word-break: break-word;
}
</style>
