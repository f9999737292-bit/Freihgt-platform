<script setup lang="ts">
defineProps<{ status: string }>()
</script>

<template>
  <span class="vehicle-status-badge" :class="`vehicle-status-badge--${status.toLowerCase()}`">
    {{ status }}
  </span>
</template>

<style scoped>
.vehicle-status-badge {
  display: inline-flex;
  padding: 0.125rem 0.5rem;
  border-radius: 999px;
  font-size: 0.75rem;
  font-weight: 600;
}

.vehicle-status-badge--active {
  background: #dcfce7;
  color: #166534;
}

.vehicle-status-badge--inactive {
  background: #eef2f6;
  color: #475569;
}
</style>
