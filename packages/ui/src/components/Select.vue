<script setup lang="ts">
defineOptions({ inheritAttrs: false })
defineProps<{
  label: string
  name: string
  options: Array<{ value: string; label: string }>
}>()
const model = defineModel<string>({ default: '' })
</script>

<template>
  <label class="ui-field">
    <span>{{ label }}</span>
    <select v-model="model" v-bind="$attrs" :name="name">
      <option v-for="option in options" :key="option.value" :value="option.value">
        {{ option.label }}
      </option>
    </select>
  </label>
</template>

<style scoped>
.ui-field {
  display: flex;
  flex-direction: column;
  gap: 0.35rem;
  font-size: 0.875rem;
}
.ui-field select {
  border: 1px solid #d1d5db;
  border-radius: 6px;
  padding: 0.5rem 0.65rem;
  font: inherit;
  background: #fff;
}
</style>
