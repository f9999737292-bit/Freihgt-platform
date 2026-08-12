<script setup lang="ts">
defineProps<{ status: string }>()
</script>

<template>
  <span class="fr-status-badge" :class="`fr-status-badge--${status.toLowerCase()}`">
    {{ status }}
  </span>
</template>

<style scoped>
.fr-status-badge {
  display: inline-flex;
  align-items: center;
  padding: 0.125rem 0.5rem;
  border-radius: 999px;
  font-size: 0.75rem;
  font-weight: 600;
  letter-spacing: 0.02em;
  text-transform: uppercase;
}

.fr-status-badge--draft {
  background: #eef2f6;
  color: #475569;
}

.fr-status-badge--published,
.fr-status-badge--responses_open {
  background: #dbeafe;
  color: #1e40af;
}

.fr-status-badge--awarded {
  background: #dcfce7;
  color: #166534;
}
</style>
