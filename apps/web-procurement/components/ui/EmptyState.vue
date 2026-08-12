<script setup lang="ts">
defineProps<{
  title: string
  description?: string
}>()
</script>

<template>
  <div class="ui-empty-state">
    <h3>{{ title }}</h3>
    <p v-if="description">{{ description }}</p>
    <div v-if="$slots.actions" class="ui-empty-state__actions">
      <slot name="actions" />
    </div>
  </div>
</template>

<style scoped>
.ui-empty-state {
  padding: 2rem 1.5rem;
  border-radius: 0.5rem;
  background: #fff;
  border: 1px dashed #d1d5db;
  text-align: center;
}

.ui-empty-state h3 {
  margin: 0 0 0.5rem;
  font-size: 1rem;
}

.ui-empty-state p {
  margin: 0;
  color: #6b7280;
  font-size: 0.875rem;
}

.ui-empty-state__actions {
  margin-top: 1rem;
}
</style>
