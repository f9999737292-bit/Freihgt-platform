<script setup lang="ts">
defineProps<{
  title: string
  subtitle?: string
}>()
</script>

<template>
  <header class="ui-page-header">
    <div>
      <h1>{{ title }}</h1>
      <p v-if="subtitle">{{ subtitle }}</p>
    </div>
    <slot />
  </header>
</template>

<style scoped>
.ui-page-header {
  display: flex;
  justify-content: space-between;
  gap: 1rem;
  align-items: flex-start;
}
.ui-page-header h1 {
  margin: 0;
  font-size: 1.25rem;
}
.ui-page-header p {
  margin: 0.25rem 0 0;
  color: #6b7280;
  font-size: 0.875rem;
}
</style>
