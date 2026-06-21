<script setup lang="ts">
defineProps<{
  modelValue: string
  label?: string
  type?: string
  placeholder?: string
  required?: boolean
  disabled?: boolean
}>()

defineEmits<{ 'update:modelValue': [value: string] }>()
</script>

<template>
  <label class="ui-input">
    <span v-if="label" class="ui-input__label">{{ label }}</span>
    <input
      class="ui-input__control"
      :type="type || 'text'"
      :value="modelValue"
      :placeholder="placeholder"
      :required="required"
      :disabled="disabled"
      @input="$emit('update:modelValue', ($event.target as HTMLInputElement).value)"
    />
  </label>
</template>

<style scoped>
.ui-input {
  display: flex;
  flex-direction: column;
  gap: 0.375rem;
}

.ui-input__label {
  font-size: 0.875rem;
  font-weight: 500;
  color: var(--color-text);
}

.ui-input__control {
  min-height: 38px;
  padding: 0.5rem 0.75rem;
  border: 1px solid var(--color-border);
  border-radius: var(--radius-md);
  background: var(--color-surface);
  font: inherit;
}

.ui-input__control:focus {
  outline: none;
  border-color: var(--color-primary);
  box-shadow: 0 0 0 3px rgba(37, 99, 235, 0.12);
}
</style>
