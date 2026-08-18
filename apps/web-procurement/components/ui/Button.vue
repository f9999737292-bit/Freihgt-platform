<script setup lang="ts">
withDefaults(
  defineProps<{
    variant?: 'primary' | 'secondary' | 'ghost' | 'danger'
    size?: 'sm' | 'md'
    type?: 'button' | 'submit' | 'reset'
    disabled?: boolean
    loading?: boolean
  }>(),
  { variant: 'primary', size: 'md', type: 'button', disabled: false, loading: false },
)
</script>

<template>
  <button
    class="ui-button"
    :class="[`ui-button--${variant}`, `ui-button--${size}`]"
    :type="type"
    :disabled="disabled || loading"
  >
    <span v-if="loading" class="ui-button__spinner" />
    <slot />
  </button>
</template>

<style scoped>
.ui-button {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  gap: 0.5rem;
  border: 1px solid transparent;
  border-radius: var(--radius-md);
  font: inherit;
  font-weight: 500;
  cursor: pointer;
  transition: background 0.15s, border-color 0.15s, color 0.15s;
}

.ui-button:disabled {
  opacity: 0.6;
  cursor: not-allowed;
}

.ui-button--md {
  min-height: 38px;
  padding: 0.5rem 1rem;
}

.ui-button--sm {
  min-height: 32px;
  padding: 0.375rem 0.75rem;
  font-size: 0.875rem;
}

.ui-button--primary {
  background: var(--color-primary);
  color: #fff;
}

.ui-button--primary:hover:not(:disabled) {
  background: var(--color-primary-hover);
}

.ui-button--secondary {
  background: var(--color-surface);
  border-color: var(--color-border);
  color: var(--color-text);
}

.ui-button--secondary:hover:not(:disabled) {
  background: var(--color-bg);
}

.ui-button--ghost {
  background: transparent;
  color: var(--color-text);
}

.ui-button--ghost:hover:not(:disabled) {
  background: rgba(26, 35, 50, 0.06);
}

.ui-button--danger {
  background: var(--color-danger);
  color: #fff;
}

.ui-button__spinner {
  width: 14px;
  height: 14px;
  border: 2px solid rgba(255, 255, 255, 0.35);
  border-top-color: #fff;
  border-radius: 50%;
  animation: spin 0.8s linear infinite;
}

@keyframes spin {
  to {
    transform: rotate(360deg);
  }
}
</style>
