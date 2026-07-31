<script setup lang="ts">
import { SHIPMENT_STATUSES } from '~/types/shipment'
import type { Company } from '~/types/company'
import type { ControlTowerFilters, SlaStatus } from '~/types/controlTower'

const props = defineProps<{
  filters: ControlTowerFilters
  shipperCompanies: Company[]
  carrierCompanies: Company[]
}>()

const emit = defineEmits<{
  reset: []
  change: []
}>()

const { t } = useI18n()

const statusOptions = computed(() => [
  { label: t('common.all'), value: '' },
  ...SHIPMENT_STATUSES.map((status) => ({ label: status, value: status })),
])

const slaOptions = computed(() => {
  const statuses: SlaStatus[] = ['ON_TIME', 'AT_RISK', 'DELAYED', 'CRITICAL', 'UNKNOWN']
  return [
    { label: t('common.all'), value: '' },
    ...statuses.map((status) => ({
      label: t(`controlTower.sla.${status}`),
      value: status,
    })),
  ]
})

const shipperOptions = computed(() => [
  { label: t('common.all'), value: '' },
  ...props.shipperCompanies.map((company) => ({
    label: company.short_name || company.legal_name,
    value: company.id,
  })),
])

const carrierOptions = computed(() => [
  { label: t('common.all'), value: '' },
  ...props.carrierCompanies.map((company) => ({
    label: company.short_name || company.legal_name,
    value: company.id,
  })),
])

let searchTimer: ReturnType<typeof setTimeout> | undefined

function onSearchInput() {
  clearTimeout(searchTimer)
  searchTimer = setTimeout(() => emit('change'), 350)
}

function onFilterChange() {
  emit('change')
}
</script>

<template>
  <div class="filters-row">
    <UiInput
      :model-value="filters.search"
      :label="$t('controlTower.filters.search')"
      @update:model-value="filters.search = $event; onSearchInput()"
    />
    <UiSelect
      :model-value="filters.status"
      :label="$t('controlTower.filters.status')"
      :options="statusOptions"
      @update:model-value="filters.status = $event; onFilterChange()"
    />
    <UiSelect
      :model-value="filters.slaStatus"
      :label="$t('controlTower.filters.slaStatus')"
      :options="slaOptions"
      @update:model-value="filters.slaStatus = $event; onFilterChange()"
    />
    <UiSelect
      :model-value="filters.shipperCompanyId"
      :label="$t('controlTower.filters.shipper')"
      :options="shipperOptions"
      @update:model-value="filters.shipperCompanyId = $event; onFilterChange()"
    />
    <UiSelect
      :model-value="filters.carrierCompanyId"
      :label="$t('controlTower.filters.carrier')"
      :options="carrierOptions"
      @update:model-value="filters.carrierCompanyId = $event; onFilterChange()"
    />
    <UiInput
      :model-value="filters.date"
      type="date"
      :label="$t('controlTower.filters.date')"
      @update:model-value="filters.date = $event; onFilterChange()"
    />
    <label class="filters-row__checkbox">
      <input
        type="checkbox"
        :checked="filters.criticalOnly"
        @change="filters.criticalOnly = ($event.target as HTMLInputElement).checked; onFilterChange()"
      />
      {{ $t('controlTower.filters.criticalOnly') }}
    </label>
    <UiButton size="sm" variant="secondary" @click="emit('reset')">
      {{ $t('controlTower.filters.reset') }}
    </UiButton>
  </div>
</template>

<style scoped>
.filters-row__checkbox {
  display: inline-flex;
  align-items: center;
  gap: 0.35rem;
  font-size: 0.875rem;
  color: var(--color-text-muted);
  padding-bottom: 0.35rem;
}
</style>
