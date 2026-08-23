<script setup lang="ts">
import type { FreightCostAnalyticsOpportunityItemDTO } from '~/types/freightCost'
import {
  dataQualityLabelKey,
  formatOpportunityBaselineValue,
  formatOpportunityEstimatedDelta,
  formatOpportunityObservedValue,
  opportunityScopeLabelKey,
  opportunityTypeLabelKey,
} from '~/utils/freightCostIntelligence'

const props = defineProps<{
  items: FreightCostAnalyticsOpportunityItemDTO[]
  liveUnavailable?: boolean
}>()

const { t, locale } = useI18n()

const unavailableLabel = computed(() => t('freightCosts.unavailable.money'))

const columns = computed(() => [
  t('freightCosts.intelligence.columns.opportunityType'),
  t('freightCosts.intelligence.columns.scope'),
  t('freightCosts.intelligence.columns.entityKey'),
  t('freightCosts.intelligence.columns.observedValue'),
  t('freightCosts.intelligence.columns.baselineValue'),
  t('freightCosts.intelligence.columns.estimatedDelta'),
  t('freightCosts.intelligence.columns.sampleSize'),
  t('freightCosts.intelligence.columns.dataQuality'),
  t('freightCosts.intelligence.columns.calculatedAt'),
])

function displayDelta(item: FreightCostAnalyticsOpportunityItemDTO): string {
  if (props.liveUnavailable) return t('freightCosts.unavailable.liveData')
  return formatOpportunityEstimatedDelta(item, locale.value, unavailableLabel.value)
}

function displayObserved(item: FreightCostAnalyticsOpportunityItemDTO): string {
  if (props.liveUnavailable) return t('freightCosts.unavailable.liveData')
  return formatOpportunityObservedValue(item, locale.value, unavailableLabel.value)
}

function displayBaseline(item: FreightCostAnalyticsOpportunityItemDTO): string {
  if (props.liveUnavailable) return t('freightCosts.unavailable.liveData')
  return formatOpportunityBaselineValue(item, locale.value, unavailableLabel.value)
}
</script>

<template>
  <Table :columns="columns">
    <tr v-for="item in items" :key="item.opportunity_id">
      <td>{{ t(opportunityTypeLabelKey(item.type)) }}</td>
      <td>{{ t(opportunityScopeLabelKey(item.scope)) }}</td>
      <td>{{ item.entity_key }}</td>
      <td>{{ displayObserved(item) }}</td>
      <td>{{ displayBaseline(item) }}</td>
      <td>{{ displayDelta(item) }}</td>
      <td>{{ item.sample_size }}</td>
      <td>
        <Badge :status="t(dataQualityLabelKey(item.data_quality))" tone="info" />
      </td>
      <td>{{ item.calculated_at }}</td>
    </tr>
  </Table>
</template>
