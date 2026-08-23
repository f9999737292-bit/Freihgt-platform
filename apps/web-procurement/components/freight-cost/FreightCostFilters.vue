<script setup lang="ts">
import type { FreightCostFilterState } from '~/utils/freightCostWorkspace'
import { activeFreightCostFilterChips } from '~/utils/freightCostWorkspace'

const props = defineProps<{
  modelValue: FreightCostFilterState
}>()

const emit = defineEmits<{
  'update:modelValue': [value: FreightCostFilterState]
}>()

const { t } = useI18n()

const chips = computed(() => activeFreightCostFilterChips(props.modelValue))

function updateField<K extends keyof FreightCostFilterState>(key: K, value: FreightCostFilterState[K]) {
  emit('update:modelValue', { ...props.modelValue, [key]: value })
}

function clearFilters() {
  emit('update:modelValue', {
    from: '',
    to: '',
    date_dimension: '',
    currency: '',
    carrier_id: '',
    origin_location_code: '',
    destination_location_code: '',
    order_status: '',
    settlement_status: '',
    variance_state: '',
    reconciliation_state: '',
    q: '',
  })
}
</script>

<template>
  <section class="freight-cost-filters" aria-label="Freight cost filters">
    <div class="freight-cost-filters__row">
      <Input
        :model-value="modelValue.q"
        :label="t('freightCosts.filters.search')"
        @update:model-value="updateField('q', String($event))"
      />
      <Input
        :model-value="modelValue.from"
        :label="t('freightCosts.filters.from')"
        type="date"
        @update:model-value="updateField('from', String($event))"
      />
      <Input
        :model-value="modelValue.to"
        :label="t('freightCosts.filters.to')"
        type="date"
        @update:model-value="updateField('to', String($event))"
      />
      <Input
        :model-value="modelValue.currency"
        :label="t('freightCosts.filters.currency')"
        @update:model-value="updateField('currency', String($event))"
      />
      <Input
        :model-value="modelValue.carrier_id"
        :label="t('freightCosts.filters.carrier')"
        @update:model-value="updateField('carrier_id', String($event))"
      />
    </div>
    <div v-if="chips.length" class="freight-cost-filters__chips">
      <Badge v-for="chip in chips" :key="chip">{{ chip }}</Badge>
      <Button variant="ghost" type="button" @click="clearFilters">{{ t('freightCosts.filters.clear') }}</Button>
    </div>
  </section>
</template>

<style scoped>
.freight-cost-filters {
  display: flex;
  flex-direction: column;
  gap: 0.75rem;
}

.freight-cost-filters__row {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(180px, 1fr));
  gap: 0.75rem;
}

.freight-cost-filters__chips {
  display: flex;
  flex-wrap: wrap;
  gap: 0.5rem;
  align-items: center;
}
</style>
