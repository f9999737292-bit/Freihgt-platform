<script setup lang="ts">
defineProps<{
  label: string
}>()
</script>

<template>
  <span class="ui-badge">{{ label }}</span>
</template>

<style scoped>
.ui-badge {
  display: inline-block;
  border-radius: 999px;
  background: #e8eef1;
  color: #0f3d4c;
  padding: 0.15rem 0.55rem;
  font-size: 0.75rem;
}
</style>
