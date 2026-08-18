<script setup lang="ts">
defineProps<{ title: string; description?: string }>()
</script>

<template>
  <div class="ui-empty">
    <h3>{{ title }}</h3>
    <p v-if="description">{{ description }}</p>
    <slot />
  </div>
</template>

<style scoped>
.ui-empty {
  padding: 3rem 1.5rem;
  text-align: center;
  color: var(--color-text-muted);
}

.ui-empty h3 {
  color: var(--color-text);
}
</style>
