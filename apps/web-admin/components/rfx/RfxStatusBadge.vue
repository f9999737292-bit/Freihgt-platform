<script setup lang="ts">
defineProps<{ status: string }>()
</script>

<template>
  <span class="rfx-status-badge" :class="`rfx-status-badge--${status.toLowerCase()}`">
    {{ status }}
  </span>
</template>

<style scoped>
.rfx-status-badge {
  display: inline-flex;
  align-items: center;
  padding: 0.125rem 0.5rem;
  border-radius: 999px;
  font-size: 0.75rem;
  font-weight: 600;
  letter-spacing: 0.02em;
  text-transform: uppercase;
}

.rfx-status-badge--draft {
  background: #eef2f6;
  color: #475569;
}

.rfx-status-badge--published {
  background: #dbeafe;
  color: #1e40af;
}

.rfx-status-badge--cancelled {
  background: #fee2e2;
  color: #991b1b;
}
</style>
