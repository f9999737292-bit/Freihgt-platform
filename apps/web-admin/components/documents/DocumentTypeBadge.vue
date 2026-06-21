<script setup lang="ts">
defineProps<{ type: string }>()
</script>

<template>
  <span class="document-type-badge">{{ type }}</span>
</template>

<style scoped>
.document-type-badge {
  display: inline-flex;
  padding: 0.125rem 0.5rem;
  border-radius: var(--radius-sm);
  font-size: 0.75rem;
  font-weight: 600;
  background: #f1f5f9;
  color: #334155;
}
</style>
