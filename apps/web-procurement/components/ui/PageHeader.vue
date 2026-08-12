<script setup lang="ts">
defineProps<{
  title: string
  description?: string
}>()
</script>

<template>
  <header class="ui-page-header">
    <div>
      <h2>{{ title }}</h2>
      <p v-if="description" class="ui-page-header__description">{{ description }}</p>
    </div>
    <div v-if="$slots.actions" class="ui-page-header__actions">
      <slot name="actions" />
    </div>
  </header>
</template>

<style scoped>
.ui-page-header {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 1rem;
  margin-bottom: 1.25rem;
}

.ui-page-header h2 {
  margin: 0;
  font-size: 1.5rem;
}

.ui-page-header__description {
  margin: 0.5rem 0 0;
  color: #6b7280;
  font-size: 0.9375rem;
}

.ui-page-header__actions {
  flex-shrink: 0;
}
</style>
