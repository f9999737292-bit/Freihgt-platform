<script setup lang="ts">
import type { ControlTowerBadgeTone } from '~/types/controlTower'

defineProps<{
  label: string
  tone: ControlTowerBadgeTone
}>()
</script>

<template>
  <span class="status-badge" :class="`status-badge--${tone}`">{{ label }}</span>
</template>

<style scoped>
.status-badge {
  display: inline-flex;
  align-items: center;
  padding: 0.125rem 0.5rem;
  border-radius: 999px;
  font-size: 0.6875rem;
  font-weight: 700;
  letter-spacing: 0.03em;
  text-transform: uppercase;
  white-space: nowrap;
}

.status-badge--ok {
  background: #dcfce7;
  color: #166534;
}

.status-badge--warning {
  background: #fef3c7;
  color: #92400e;
}

.status-badge--down {
  background: #fee2e2;
  color: #991b1b;
}

.status-badge--unavailable {
  background: #f1f5f9;
  color: #64748b;
}

.status-badge--neutral {
  background: #eef2f6;
  color: #475569;
}
</style>
