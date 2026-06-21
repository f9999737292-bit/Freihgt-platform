<script setup lang="ts">
defineProps<{ status: string }>()
</script>

<template>
  <span class="driver-status-badge" :class="`driver-status-badge--${status.toLowerCase()}`">
    {{ status }}
  </span>
</template>

<style scoped>
.driver-status-badge {
  display: inline-flex;
  padding: 0.125rem 0.5rem;
  border-radius: 999px;
  font-size: 0.75rem;
  font-weight: 600;
}

.driver-status-badge--active {
  background: #dcfce7;
  color: #166534;
}

.driver-status-badge--inactive {
  background: #eef2f6;
  color: #475569;
}
</style>
