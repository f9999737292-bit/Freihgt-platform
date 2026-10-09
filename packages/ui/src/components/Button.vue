<script setup lang="ts">
withDefaults(defineProps<{
  type?: 'button' | 'submit'
  variant?: 'primary' | 'ghost'
  disabled?: boolean
}>(), {
  type: 'button',
  variant: 'primary',
  disabled: false,
})
</script>

<template>
  <button :type="type" class="ui-button" :class="`ui-button--${variant}`" :disabled="disabled">
    <slot />
  </button>
</template>

<style scoped>
.ui-button {
  border-radius: 6px;
  border: 1px solid transparent;
  padding: 0.5rem 0.875rem;
  font: inherit;
  cursor: pointer;
}
.ui-button:disabled {
  cursor: not-allowed;
  opacity: 0.6;
}
.ui-button--primary {
  background: #0f3d4c;
  color: #fff;
}
.ui-button--ghost {
  background: #fff;
  border-color: #d1d5db;
  color: #1a1a1a;
}
</style>
