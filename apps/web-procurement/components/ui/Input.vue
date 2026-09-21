<script setup lang="ts">
import { applyInputModel } from '~/utils/inputModel'

defineOptions({ inheritAttrs: false })

const [model, modifiers] = defineModel<string | number | null>({ default: '' })

defineProps<{
  label?: string
  type?: string
  placeholder?: string
  required?: boolean
  disabled?: boolean
}>()

const attrs = useAttrs()

function displayValue(): string {
  if (model.value == null) return ''
  return String(model.value)
}

function onInput(event: Event) {
  model.value = applyInputModel((event.target as HTMLInputElement).value, Boolean(modifiers.number))
}
</script>

<template>
  <label class="ui-input">
    <span v-if="label" class="ui-input__label">{{ label }}</span>
    <input
      class="ui-input__control"
      v-bind="attrs"
      :type="type || 'text'"
      :placeholder="placeholder"
      :required="required"
      :disabled="disabled"
      :value="displayValue()"
      @input="onInput"
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
