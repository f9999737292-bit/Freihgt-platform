<script setup lang="ts">
defineProps<{
  title?: string
  message?: string
}>()

const emit = defineEmits<{
  retry: []
}>()
</script>

<template>
  <div class="api-unavailable">
    <h3>{{ title || $t('common.apiUnavailable') }}</h3>
    <p>{{ message || $t('common.apiUnavailableHint') }}</p>
    <UiButton size="sm" variant="secondary" @click="emit('retry')">
      {{ $t('common.refresh') }}
    </UiButton>
  </div>
</template>

<style scoped>
.api-unavailable {
  padding: 1.5rem;
  border: 1px solid #fde68a;
  border-radius: var(--radius-lg);
  background: #fffbeb;
  color: #92400e;
}

.api-unavailable h3 {
  margin: 0 0 0.5rem;
  color: #78350f;
  font-size: 1rem;
}

.api-unavailable p {
  margin: 0 0 1rem;
  font-size: 0.875rem;
}
</style>
