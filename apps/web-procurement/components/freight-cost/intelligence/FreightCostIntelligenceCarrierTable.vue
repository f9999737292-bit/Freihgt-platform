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
  <div class="ui-table-wrap">
    <table class="ui-table">
      <thead>
        <tr>
          <th v-for="column in columns" :key="column">{{ column }}</th>
        </tr>
      </thead>
      <tbody>
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
      </tbody>
    </table>
  </div>
</template>

<style scoped>
.ui-table-wrap {
  overflow-x: auto;
  border: 1px solid var(--color-border);
  border-radius: var(--radius-lg);
  background: var(--color-surface);
}

.ui-table th,
.ui-table td {
  padding: 0.75rem 1rem;
  text-align: left;
  border-bottom: 1px solid var(--color-border);
}

.ui-table th {
  background: #f8fafc;
  font-size: 0.75rem;
  font-weight: 600;
  text-transform: uppercase;
  letter-spacing: 0.04em;
  color: var(--color-text-muted);
}

.ui-table tr:last-child td {
  border-bottom: none;
}
</style>
