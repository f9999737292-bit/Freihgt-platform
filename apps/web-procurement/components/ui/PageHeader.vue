<script setup lang="ts">
defineProps<{ title: string; subtitle?: string }>()
</script>

<template>
  <div class="ui-page-header">
    <div>
      <h1>{{ title }}</h1>
      <p v-if="subtitle" class="ui-page-header__subtitle">{{ subtitle }}</p>
    </div>
    <div v-if="$slots.actions" class="ui-page-header__actions">
      <slot name="actions" />
    </div>
  </div>
</template>

<style scoped>
.ui-page-header {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 1rem;
}

.ui-page-header h1 {
  font-size: 1.5rem;
}

.ui-page-header__subtitle {
  margin: 0;
  color: var(--color-text-muted);
}

.ui-page-header__actions {
  display: flex;
  gap: 0.75rem;
  flex-shrink: 0;
}
</style>
