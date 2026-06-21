<script setup lang="ts">
defineProps<{ status: string }>()
</script>

<template>
  <span class="document-status-badge" :class="`document-status-badge--${status.toLowerCase()}`">
    {{ status }}
  </span>
</template>

<style scoped>
.document-status-badge {
  display: inline-flex;
  padding: 0.125rem 0.5rem;
  border-radius: 999px;
  font-size: 0.75rem;
  font-weight: 600;
  letter-spacing: 0.02em;
  text-transform: uppercase;
}

.document-status-badge--draft {
  background: #eef2f6;
  color: #475569;
}

.document-status-badge--ready_for_signing,
.document-status-badge--signing_in_progress {
  background: #dbeafe;
  color: #1e40af;
}

.document-status-badge--signed,
.document-status-badge--accepted {
  background: #dcfce7;
  color: #166534;
}

.document-status-badge--sent_to_operator {
  background: #fef3c7;
  color: #92400e;
}

.document-status-badge--rejected,
.document-status-badge--cancelled {
  background: #fee2e2;
  color: #991b1b;
}

.document-status-badge--archived {
  background: #ede9fe;
  color: #5b21b6;
}
</style>
