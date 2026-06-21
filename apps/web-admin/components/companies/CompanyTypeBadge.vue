<script setup lang="ts">
defineProps<{ type: string }>()
</script>

<template>
  <span class="company-type-badge">{{ type }}</span>
</template>

<style scoped>
.company-type-badge {
  display: inline-flex;
  align-items: center;
  padding: 0.125rem 0.5rem;
  border-radius: var(--radius-sm);
  font-size: 0.75rem;
  font-weight: 600;
  letter-spacing: 0.02em;
  background: #eef2f6;
  color: #334155;
  border: 1px solid var(--color-border);
}
</style>
