<script setup lang="ts">
import type { FreightCostAnalyticsCarrierItemDTO } from '~/types/freightCost'
import {
  dataQualityLabelKey,
  formatAnalyticsMoney,
  formatCarrierAnalyticsName,
} from '~/utils/freightCostIntelligence'

const props = defineProps<{
  items: FreightCostAnalyticsCarrierItemDTO[]
  mixedCurrency?: boolean
  liveUnavailable?: boolean
}>()

const { t, locale } = useI18n()

const unavailableLabel = computed(() => t('freightCosts.unavailable.money'))
const referenceUnavailable = computed(() => t('freightCosts.unavailable.reference'))

const columns = computed(() => [
  t('freightCosts.columns.carrier'),
  t('freightCosts.intelligence.columns.orderCount'),
  t('freightCosts.intelligence.columns.laneCount'),
  t('freightCosts.columns.planned'),
  t('freightCosts.columns.currentActual'),
  t('freightCosts.columns.finalActual'),
  t('freightCosts.columns.currentVariance'),
  t('freightCosts.intelligence.columns.laneNormalizedDelta'),
  t('freightCosts.intelligence.columns.dataQuality'),
])

function moneyCell(money: Parameters<typeof formatAnalyticsMoney>[0]): string {
  if (props.liveUnavailable) return t('freightCosts.unavailable.liveData')
  return formatAnalyticsMoney(money, locale.value, unavailableLabel.value, props.mixedCurrency)
}

function carrierName(item: FreightCostAnalyticsCarrierItemDTO): string {
  if (props.liveUnavailable) return t('freightCosts.unavailable.liveData')
  return formatCarrierAnalyticsName(item.carrier_name, referenceUnavailable.value)
}
</script>

<template>
  <Table :columns="columns">
    <tr v-for="item in items" :key="item.carrier_company_id">
      <td>{{ carrierName(item) }}</td>
      <td>{{ item.order_count }}</td>
      <td>{{ item.lane_count }}</td>
      <td>{{ moneyCell(item.planned_total) }}</td>
      <td>{{ moneyCell(item.current_actual_total) }}</td>
      <td>{{ moneyCell(item.final_actual_total) }}</td>
      <td>{{ moneyCell(item.variance_total) }}</td>
      <td>{{ moneyCell(item.lane_normalized_delta) }}</td>
      <td>
        <Badge :status="t(dataQualityLabelKey(item.data_quality))" tone="info" />
      </td>
    </tr>
  </Table>
</template>
