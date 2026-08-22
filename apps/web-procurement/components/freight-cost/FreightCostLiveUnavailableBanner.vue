<script setup lang="ts">
defineProps<{ message: string }>()
</script>

<template>
  <div class="live-unavailable-banner" role="status">
    {{ message }}
  </div>
</template>

<style scoped>
.live-unavailable-banner {
  padding: 0.75rem 1rem;
  border: 1px solid #fcd34d;
  border-radius: var(--radius-md);
  background: #fffbeb;
  color: #92400e;
  font-size: 0.875rem;
}
</style>
