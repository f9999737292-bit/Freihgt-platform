<script setup lang="ts">
defineProps<{
  label: string
}>()
</script>

<template>
  <div class="ui-state" role="status">{{ label }}</div>
</template>

<style scoped>
.ui-state {
  padding: 1.5rem 0.5rem;
  color: #4b5563;
}
</style>
