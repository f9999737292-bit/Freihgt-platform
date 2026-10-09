<script setup lang="ts">
defineProps<{
  title: string
  description?: string
}>()
</script>

<template>
  <div class="ui-error" role="alert">
    <h2>{{ title }}</h2>
    <p v-if="description">{{ description }}</p>
  </div>
</template>

<style scoped>
.ui-error {
  padding: 1rem;
  border: 1px solid #fecaca;
  border-radius: 8px;
  background: #fef2f2;
  color: #7f1d1d;
}
.ui-error h2 {
  margin: 0 0 0.35rem;
  font-size: 1rem;
}
.ui-error p {
  margin: 0;
}
</style>
