<script setup lang="ts">
defineOptions({ inheritAttrs: false })
defineProps<{
  label: string
  name: string
  type?: string
  required?: boolean
}>()
const model = defineModel<string>({ default: '' })
</script>

<template>
  <label class="ui-field">
    <span>{{ label }}</span>
    <input v-model="model" v-bind="$attrs" :name="name" :type="type ?? 'text'" :required="required">
  </label>
</template>

<style scoped>
.ui-field {
  display: flex;
  flex-direction: column;
  gap: 0.35rem;
  font-size: 0.875rem;
}
.ui-field input {
  border: 1px solid #d1d5db;
  border-radius: 6px;
  padding: 0.5rem 0.65rem;
  font: inherit;
}
</style>
