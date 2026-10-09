<script setup lang="ts">
defineProps<{
  message: string
}>()
</script>

<template>
  <div class="ui-toast" role="status">{{ message }}</div>
</template>

<style scoped>
.ui-toast {
  position: fixed;
  right: 1rem;
  bottom: 1rem;
  max-width: 24rem;
  padding: 0.75rem 1rem;
  border-radius: 8px;
  background: #111827;
  color: #fff;
  font-size: 0.875rem;
}
</style>
