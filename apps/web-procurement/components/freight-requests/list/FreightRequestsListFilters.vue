<script setup lang="ts">
import { FREIGHT_REQUEST_STATUSES, FREIGHT_REQUEST_TYPES } from '~/types/rfx'

const requestType = defineModel<string>('requestType', { default: '' })
const status = defineModel<string>('status', { default: '' })

const emit = defineEmits<{ change: [] }>()

const { t } = useI18n()

const typeOptions = computed(() => [
  { label: t('freightRequests.list.all'), value: '' },
  ...FREIGHT_REQUEST_TYPES.map((value) => ({ label: value, value })),
])

const statusOptions = computed(() => [
  { label: t('freightRequests.list.all'), value: '' },
  ...FREIGHT_REQUEST_STATUSES.map((value) => ({ label: value, value })),
])

function onChange() {
  emit('change')
}
</script>

<template>
  <div class="fr-list-filters">
    <label class="fr-list-filters__field">
      <span class="fr-list-filters__label">{{ $t('freightRequests.list.requestType') }}</span>
      <select v-model="requestType" class="fr-list-filters__select" @change="onChange">
        <option v-for="option in typeOptions" :key="option.value || 'all'" :value="option.value">
          {{ option.label }}
        </option>
      </select>
    </label>

    <label class="fr-list-filters__field">
      <span class="fr-list-filters__label">{{ $t('freightRequests.list.status') }}</span>
      <select v-model="status" class="fr-list-filters__select" @change="onChange">
        <option v-for="option in statusOptions" :key="option.value || 'all'" :value="option.value">
          {{ option.label }}
        </option>
      </select>
    </label>
  </div>
</template>

<style scoped>
.fr-list-filters {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 1rem;
}

.fr-list-filters__field {
  display: flex;
  flex-direction: column;
  gap: 0.375rem;
}

.fr-list-filters__label {
  font-size: 0.8125rem;
  font-weight: 500;
  color: #374151;
}

.fr-list-filters__select {
  width: 100%;
  padding: 0.5rem 0.75rem;
  border: 1px solid #d1d5db;
  border-radius: 0.375rem;
  background: #fff;
  font: inherit;
  font-size: 0.875rem;
}

@media (max-width: 640px) {
  .fr-list-filters {
    grid-template-columns: 1fr;
  }
}
</style>
