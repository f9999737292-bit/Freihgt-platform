<script setup lang="ts">
defineProps<{
  title: string
  value: string
  helper?: string
  unavailable?: boolean
}>()
</script>

<template>
  <article
    class="kpi-card"
    role="group"
    :aria-label="title"
  >
    <h3 class="kpi-card__title">{{ title }}</h3>
    <p
      class="kpi-card__value"
      :class="{ 'kpi-card__value--unavailable': unavailable }"
      :aria-label="unavailable ? helper || title : value"
    >
      {{ value }}
    </p>
    <p v-if="helper" class="kpi-card__helper">{{ helper }}</p>
  </article>
</template>

<style scoped>
.kpi-card {
  background: var(--color-surface);
  border: 1px solid var(--color-border);
  border-radius: var(--radius-lg);
  padding: 1rem 1.25rem;
  box-shadow: var(--shadow-sm);
}

.kpi-card__title {
  margin: 0 0 0.5rem;
  font-size: 0.875rem;
  font-weight: 500;
  color: var(--color-text-muted);
}

.kpi-card__value {
  margin: 0;
  font-size: 1.5rem;
  font-weight: 600;
  color: var(--color-text);
}

.kpi-card__value--unavailable {
  color: var(--color-text-muted);
}

.kpi-card__helper {
  margin: 0.5rem 0 0;
  font-size: 0.8125rem;
  color: var(--color-text-muted);
}
</style>
