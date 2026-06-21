<script setup lang="ts">
defineProps<{ status: string }>()
</script>

<template>
  <span class="bid-status-badge" :class="`bid-status-badge--${status.toLowerCase()}`">
    {{ status }}
  </span>
</template>

<style scoped>
.bid-status-badge {
  display: inline-flex;
  align-items: center;
  padding: 0.125rem 0.5rem;
  border-radius: 999px;
  font-size: 0.75rem;
  font-weight: 600;
  letter-spacing: 0.02em;
  text-transform: uppercase;
}

.bid-status-badge--draft {
  background: #eef2f6;
  color: #475569;
}

.bid-status-badge--submitted {
  background: #dbeafe;
  color: #1e40af;
}

.bid-status-badge--accepted {
  background: #dcfce7;
  color: #166534;
}

.bid-status-badge--rejected {
  background: #fee2e2;
  color: #991b1b;
}
</style>
