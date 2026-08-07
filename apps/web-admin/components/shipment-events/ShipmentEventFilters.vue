<script setup lang="ts">
import type { ShipmentEventQueryFilters } from '~/types/shipmentEvent'

const props = defineProps<{
  modelValue: ShipmentEventQueryFilters
  disabled?: boolean
}>()

const emit = defineEmits<{
  'update:modelValue': [ShipmentEventQueryFilters]
  apply: []
  reset: []
}>()

const local = reactive({ ...props.modelValue })

watch(
  () => props.modelValue,
  (value) => Object.assign(local, value),
  { deep: true },
)

function apply() {
  emit('update:modelValue', { ...local })
  emit('apply')
}

function reset() {
  emit('reset')
}
</script>

<template>
  <UiCard class="filters-card">
    <template #header>
      <h3 class="card-title">{{ $t('shipmentEvents.filters.title') }}</h3>
    </template>

    <div class="filters-grid">
      <label>
        <span>{{ $t('shipmentEvents.filters.type') }}</span>
        <input v-model="local.type" type="text" :disabled="disabled" />
      </label>
      <label>
        <span>{{ $t('shipmentEvents.filters.category') }}</span>
        <select v-model="local.category" :disabled="disabled">
          <option value="">{{ $t('common.all') }}</option>
          <option v-for="category in ['SHIPMENT','OPERATION','DOCUMENT','SLA','BILLING','TECHNICAL','GEOLOCATION','SYSTEM']" :key="category" :value="category">
            {{ $t(`shipmentEvents.categories.${category}`) }}
          </option>
        </select>
      </label>
      <label>
        <span>{{ $t('shipmentEvents.filters.source') }}</span>
        <select v-model="local.source" :disabled="disabled">
          <option value="">{{ $t('common.all') }}</option>
          <option v-for="source in ['SHIPMENT_STATE','SLA_CALCULATOR','DOCUMENT_STATE','BILLING_STATE']" :key="source" :value="source">
            {{ $t(`shipmentEvents.sources.${source}`) }}
          </option>
        </select>
      </label>
      <label>
        <span>{{ $t('shipmentEvents.filters.severity') }}</span>
        <select v-model="local.severity" :disabled="disabled">
          <option value="">{{ $t('common.all') }}</option>
          <option v-for="severity in ['INFO','WARNING','CRITICAL']" :key="severity" :value="severity">
            {{ $t(`shipmentEvents.severities.${severity}`) }}
          </option>
        </select>
      </label>
      <label>
        <span>{{ $t('shipmentEvents.filters.dateFrom') }}</span>
        <input v-model="local.date_from" type="date" :disabled="disabled" />
      </label>
      <label>
        <span>{{ $t('shipmentEvents.filters.dateTo') }}</span>
        <input v-model="local.date_to" type="date" :disabled="disabled" />
      </label>
      <label>
        <span>{{ $t('shipmentEvents.filters.derived') }}</span>
        <select v-model="local.derived" :disabled="disabled">
          <option value="">{{ $t('common.all') }}</option>
          <option value="true">{{ $t('shipmentEvents.filters.derivedOnly') }}</option>
          <option value="false">{{ $t('shipmentEvents.filters.canonicalOnly') }}</option>
        </select>
      </label>
      <label>
        <span>{{ $t('shipmentEvents.filters.order') }}</span>
        <select v-model="local.order" :disabled="disabled">
          <option value="desc">{{ $t('shipmentEvents.filters.newestFirst') }}</option>
          <option value="asc">{{ $t('shipmentEvents.filters.oldestFirst') }}</option>
        </select>
      </label>
    </div>

    <div class="filters-actions">
      <UiButton size="sm" :disabled="disabled" @click="apply">{{ $t('shipmentEvents.filters.apply') }}</UiButton>
      <UiButton size="sm" variant="secondary" :disabled="disabled" @click="reset">{{ $t('shipmentEvents.filters.reset') }}</UiButton>
    </div>
  </UiCard>
</template>

<style scoped>
.filters-grid {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(180px, 1fr));
  gap: 0.75rem;
}

label {
  display: flex;
  flex-direction: column;
  gap: 0.25rem;
  font-size: 0.8125rem;
}

input,
select {
  padding: 0.375rem 0.5rem;
  border: 1px solid var(--color-border);
  border-radius: var(--radius-md);
}

.filters-actions {
  display: flex;
  gap: 0.5rem;
  margin-top: 1rem;
}
</style>
