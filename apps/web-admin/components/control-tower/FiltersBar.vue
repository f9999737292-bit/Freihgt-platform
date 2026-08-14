<script setup lang="ts">
import { SHIPMENT_STATUSES } from '~/types/shipment'
import type { Company } from '~/types/company'
import type { ControlTowerFilters, SlaStatus } from '~/types/controlTower'
import {
  CONTROL_TOWER_BUSINESS_IMPACTS,
  CONTROL_TOWER_ESCALATION_LEVELS,
  CONTROL_TOWER_EVENT_SLA_STATUSES,
  CONTROL_TOWER_EXCEPTION_CATEGORIES,
  CONTROL_TOWER_PREDICTED_EXCEPTION_TYPES,
  CONTROL_TOWER_PRIORITIES,
  CONTROL_TOWER_RISK_LEVELS,
  CONTROL_TOWER_RISK_STATUSES,
  type ControlTowerEventWorkflowStatus,
} from '~/types/controlTower'

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

const workflowStatusOptions = computed(() => {
  const statuses: ControlTowerEventWorkflowStatus[] = ['open', 'acknowledged', 'assigned', 'resolved']
  return [
    { label: t('common.all'), value: '' },
    ...statuses.map((status) => ({
      label: t(`controlTower.events.status.${status}`),
      value: status,
    })),
  ]
})

const priorityOptions = computed(() => [
  { label: t('common.all'), value: '' },
  ...CONTROL_TOWER_PRIORITIES.map((value) => ({
    label: t(`controlTower.exceptions.priorities.${value}`),
    value,
  })),
])

const categoryOptions = computed(() => [
  { label: t('common.all'), value: '' },
  ...CONTROL_TOWER_EXCEPTION_CATEGORIES.map((value) => ({
    label: t(`controlTower.exceptions.categories.${value}`),
    value,
  })),
])

const businessImpactOptions = computed(() => [
  { label: t('common.all'), value: '' },
  ...CONTROL_TOWER_BUSINESS_IMPACTS.map((value) => ({
    label: t(`controlTower.exceptions.businessImpact.${value}`),
    value,
  })),
])

const eventSlaOptions = computed(() => [
  { label: t('common.all'), value: '' },
  ...CONTROL_TOWER_EVENT_SLA_STATUSES.map((value) => ({
    label: t(`controlTower.exceptions.slaStatus.${value}`),
    value,
  })),
])

const escalationOptions = computed(() => [
  { label: t('common.all'), value: '' },
  ...CONTROL_TOWER_ESCALATION_LEVELS.filter((value) => value !== 'none').map((value) => ({
    label: t(`controlTower.exceptions.escalation.${value}`),
    value,
  })),
])

const riskLevelOptions = computed(() => [
  { label: t('common.all'), value: '' },
  ...CONTROL_TOWER_RISK_LEVELS.map((value) => ({
    label: t(`controlTower.risk.levels.${value}`),
    value,
  })),
])

const riskStatusOptions = computed(() => [
  { label: t('common.all'), value: '' },
  ...CONTROL_TOWER_RISK_STATUSES.map((value) => ({
    label: t(`controlTower.risk.status.${value}`),
    value,
  })),
])

const predictedTypeOptions = computed(() => [
  { label: t('common.all'), value: '' },
  ...CONTROL_TOWER_PREDICTED_EXCEPTION_TYPES.map((value) => ({
    label: t(`controlTower.risk.types.${value}`),
    value,
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
  <div class="filters-bar">
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

  <div class="filters-row filters-row--events">
    <UiSelect
      :model-value="filters.eventStatus"
      :label="$t('controlTower.filters.eventStatus')"
      :options="workflowStatusOptions"
      @update:model-value="filters.eventStatus = $event; onFilterChange()"
    />
    <UiSelect
      :model-value="filters.priority"
      :label="$t('controlTower.filters.priority')"
      :options="priorityOptions"
      @update:model-value="filters.priority = $event; onFilterChange()"
    />
    <UiSelect
      :model-value="filters.exceptionCategory"
      :label="$t('controlTower.filters.exceptionCategory')"
      :options="categoryOptions"
      @update:model-value="filters.exceptionCategory = $event; onFilterChange()"
    />
    <UiSelect
      :model-value="filters.businessImpact"
      :label="$t('controlTower.filters.businessImpact')"
      :options="businessImpactOptions"
      @update:model-value="filters.businessImpact = $event; onFilterChange()"
    />
    <UiSelect
      :model-value="filters.eventSlaStatus"
      :label="$t('controlTower.filters.eventSlaStatus')"
      :options="eventSlaOptions"
      @update:model-value="filters.eventSlaStatus = $event; onFilterChange()"
    />
    <UiSelect
      :model-value="filters.escalationLevel"
      :label="$t('controlTower.filters.escalationLevel')"
      :options="escalationOptions"
      @update:model-value="filters.escalationLevel = $event; onFilterChange()"
    />
    <label class="filters-row__checkbox">
      <input
        type="checkbox"
        :checked="filters.unassignedOnly"
        @change="filters.unassignedOnly = ($event.target as HTMLInputElement).checked; onFilterChange()"
      />
      {{ $t('controlTower.filters.unassignedOnly') }}
    </label>
  </div>

  <div class="filters-row filters-row--risks">
    <UiSelect
      :model-value="filters.riskLevel"
      :label="$t('controlTower.filters.riskLevel')"
      :options="riskLevelOptions"
      @update:model-value="filters.riskLevel = $event; onFilterChange()"
    />
    <UiSelect
      :model-value="filters.riskStatus"
      :label="$t('controlTower.filters.riskStatus')"
      :options="riskStatusOptions"
      @update:model-value="filters.riskStatus = $event; onFilterChange()"
    />
    <UiSelect
      :model-value="filters.predictedExceptionType"
      :label="$t('controlTower.filters.predictedExceptionType')"
      :options="predictedTypeOptions"
      @update:model-value="filters.predictedExceptionType = $event; onFilterChange()"
    />
    <label class="filters-row__checkbox">
      <input
        type="checkbox"
        :checked="filters.riskMitigatingOnly"
        @change="filters.riskMitigatingOnly = ($event.target as HTMLInputElement).checked; onFilterChange()"
      />
      {{ $t('controlTower.filters.riskMitigatingOnly') }}
    </label>
    <label class="filters-row__checkbox">
      <input
        type="checkbox"
        :checked="filters.riskNonMitigatingOnly"
        @change="filters.riskNonMitigatingOnly = ($event.target as HTMLInputElement).checked; onFilterChange()"
      />
      {{ $t('controlTower.filters.riskNonMitigatingOnly') }}
    </label>
  </div>
  </div>
</template>

<style scoped>
.filters-bar {
  display: flex;
  flex-direction: column;
  gap: 0;
}

.filters-row {
  display: flex;
  flex-wrap: wrap;
  gap: 0.75rem;
}

.filters-row--events {
  display: flex;
  flex-wrap: wrap;
  gap: 0.75rem;
  margin-top: 0.75rem;
  padding-top: 0.75rem;
  border-top: 1px solid var(--color-border);
}
.filters-row__checkbox {
  display: inline-flex;
  align-items: center;
  gap: 0.35rem;
  font-size: 0.875rem;
  color: var(--color-text-muted);
  padding-bottom: 0.35rem;
}
</style>
