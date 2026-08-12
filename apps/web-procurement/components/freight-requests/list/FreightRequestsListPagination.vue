<script setup lang="ts">
defineProps<{
  total: number
  canGoPrev: boolean
  canGoNext: boolean
}>()

const emit = defineEmits<{
  prev: []
  next: []
}>()
</script>

<template>
  <div class="fr-list-pagination">
    <span class="fr-list-pagination__total">{{ total }}</span>
    <div class="fr-list-pagination__actions">
      <UiButton variant="secondary" :disabled="!canGoPrev" @click="emit('prev')">
        ←
      </UiButton>
      <UiButton variant="secondary" :disabled="!canGoNext" @click="emit('next')">
        →
      </UiButton>
    </div>
  </div>
</template>

<style scoped>
.fr-list-pagination {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 1rem 1.25rem;
  border-top: 1px solid #e5e7eb;
}

.fr-list-pagination__total {
  font-size: 0.875rem;
  color: #6b7280;
}

.fr-list-pagination__actions {
  display: flex;
  gap: 0.5rem;
}
</style>
