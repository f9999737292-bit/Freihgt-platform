<script setup lang="ts">
defineProps<{
  title: string
  description?: string
}>()
</script>

<template>
  <div class="ui-state" role="status">
    <h2>{{ title }}</h2>
    <p v-if="description">{{ description }}</p>
  </div>
</template>

<style scoped>
.ui-state {
  padding: 1.5rem 0.5rem;
  color: #4b5563;
}
.ui-state h2 {
  margin: 0 0 0.35rem;
  font-size: 1rem;
  color: #1a1a1a;
}
.ui-state p {
  margin: 0;
}
</style>
