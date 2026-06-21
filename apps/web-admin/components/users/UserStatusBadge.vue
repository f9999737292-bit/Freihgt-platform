<script setup lang="ts">
defineProps<{ status: string }>()
</script>

<template>
  <span class="user-status-badge" :class="`user-status-badge--${status.toLowerCase()}`">
    {{ status }}
  </span>
</template>

<style scoped>
.user-status-badge {
  display: inline-flex;
  align-items: center;
  padding: 0.125rem 0.5rem;
  border-radius: 999px;
  font-size: 0.75rem;
  font-weight: 600;
  letter-spacing: 0.02em;
  text-transform: uppercase;
}

.user-status-badge--active {
  background: #dcfce7;
  color: #166534;
}

.user-status-badge--deleted {
  background: #fee2e2;
  color: #991b1b;
}
</style>
