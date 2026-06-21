<script setup lang="ts">
defineProps<{ status: string }>()
</script>

<template>
  <span class="company-status-badge" :class="`company-status-badge--${status.toLowerCase()}`">
    {{ status }}
  </span>
</template>

<style scoped>
.company-status-badge {
  display: inline-flex;
  align-items: center;
  padding: 0.125rem 0.5rem;
  border-radius: 999px;
  font-size: 0.75rem;
  font-weight: 600;
  letter-spacing: 0.02em;
  text-transform: uppercase;
}

.company-status-badge--active {
  background: #dcfce7;
  color: #166534;
}

.company-status-badge--deleted {
  background: #fee2e2;
  color: #991b1b;
}

.company-status-badge--inactive,
.company-status-badge--suspended {
  background: #fef3c7;
  color: #92400e;
}

.company-status-badge--draft {
  background: #eef2f6;
  color: #475569;
}
</style>
