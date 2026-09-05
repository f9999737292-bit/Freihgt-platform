<script setup lang="ts">
withDefaults(
  defineProps<{
    modelValue: string
    label?: string
    options: Array<{ label: string; value: string }>
    placeholder?: string
    disabled?: boolean
    loading?: boolean
    required?: boolean
  }>(),
  {
    placeholder: '',
    disabled: false,
    loading: false,
    required: false,
  },
)

defineEmits<{ 'update:modelValue': [value: string] }>()
</script>

<template>
  <label class="ui-select">
    <span v-if="label" class="ui-select__label">{{ label }}</span>
    <select
      class="ui-select__control"
      :class="{ 'ui-select__control--loading': loading }"
      :value="modelValue"
      :disabled="disabled || loading"
      :required="required"
      @change="$emit('update:modelValue', ($event.target as HTMLSelectElement).value)"
    >
      <option v-if="placeholder" value="" disabled hidden>
        {{ placeholder }}
      </option>
      <option v-for="option in options" :key="option.value" :value="option.value">
        {{ option.label }}
      </option>
    </select>
  </label>
</template>

<style scoped>
.ui-select {
  display: flex;
  flex-direction: column;
  gap: 0.375rem;
}

.ui-select__label {
  font-size: 0.875rem;
  font-weight: 500;
}

.ui-select__control {
  min-height: 38px;
  padding: 0.5rem 0.75rem;
  border: 1px solid var(--color-border);
  border-radius: var(--radius-md);
  background: var(--color-surface);
  font: inherit;
}

.ui-select__control:disabled {
  opacity: 0.7;
  cursor: not-allowed;
}

.ui-select__control--loading {
  color: var(--color-text-muted);
}
</style>
