<script setup lang="ts">
defineProps<{ type: string }>()
</script>

<template>
  <span class="fr-type-badge">{{ type }}</span>
</template>

<style scoped>
.fr-type-badge {
  display: inline-flex;
  align-items: center;
  padding: 0.125rem 0.5rem;
  border-radius: 0.25rem;
  font-size: 0.75rem;
  font-weight: 600;
  background: #f1f5f9;
  color: #334155;
}
</style>
