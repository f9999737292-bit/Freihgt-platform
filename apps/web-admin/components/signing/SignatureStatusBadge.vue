<script setup lang="ts">
defineProps<{ status: string }>()
</script>

<template>
  <span class="signature-status-badge" :class="`signature-status-badge--${status.toLowerCase()}`">
    {{ status }}
  </span>
</template>

<style scoped>
.signature-status-badge {
  display: inline-flex;
  padding: 0.125rem 0.5rem;
  border-radius: 999px;
  font-size: 0.75rem;
  font-weight: 600;
}

.signature-status-badge--created,
.signature-status-badge--in_progress {
  background: #dbeafe;
  color: #1e40af;
}

.signature-status-badge--completed {
  background: #dcfce7;
  color: #166534;
}

.signature-status-badge--expired,
.signature-status-badge--cancelled,
.signature-status-badge--failed {
  background: #fee2e2;
  color: #991b1b;
}
</style>
