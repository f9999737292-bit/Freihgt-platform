<script setup lang="ts">
withDefaults(
  defineProps<{
    to?: string
    type?: 'button' | 'submit'
    variant?: 'primary' | 'secondary'
    disabled?: boolean
  }>(),
  {
    type: 'button',
    variant: 'primary',
    disabled: false,
  },
)
</script>

<template>
  <NuxtLink v-if="to" :to="to" class="ui-button" :class="`ui-button--${variant}`">
    <slot />
  </NuxtLink>
  <button
    v-else
    :type="type"
    class="ui-button"
    :class="`ui-button--${variant}`"
    :disabled="disabled"
  >
    <slot />
  </button>
</template>

<style scoped>
.ui-button {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  padding: 0.625rem 1rem;
  border-radius: 0.375rem;
  font: inherit;
  font-size: 0.875rem;
  font-weight: 500;
  text-decoration: none;
  cursor: pointer;
}

.ui-button--primary {
  border: 1px solid #2563eb;
  background: #2563eb;
  color: #fff;
}

.ui-button--secondary {
  border: 1px solid #d1d5db;
  background: #fff;
  color: #1a1a1a;
}

.ui-button:disabled {
  opacity: 0.7;
  cursor: not-allowed;
}
</style>
