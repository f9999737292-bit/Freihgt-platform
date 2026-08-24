<script setup lang="ts">
import type { FreightCostAnalyticsLaneItemDTO } from '~/types/freightCost'
import { formatAnalyticsMoney } from '~/utils/freightCostIntelligence'
import { dataQualityLabelKey } from '~/utils/freightCostIntelligence'

const props = defineProps<{
  items: FreightCostAnalyticsLaneItemDTO[]
  mixedCurrency?: boolean
  liveUnavailable?: boolean
}>()

const { t, locale } = useI18n()

const unavailableLabel = computed(() => t('freightCosts.unavailable.money'))

const columns = computed(() => [
  t('freightCosts.intelligence.columns.lane'),
  t('freightCosts.intelligence.columns.orderCount'),
  t('freightCosts.intelligence.columns.carrierCount'),
  t('freightCosts.columns.planned'),
  t('freightCosts.columns.currentActual'),
  t('freightCosts.columns.finalActual'),
  t('freightCosts.columns.currentVariance'),
  t('freightCosts.intelligence.columns.benchmarkMedian'),
  t('freightCosts.intelligence.columns.benchmarkSample'),
])

function benchmarkMedian(item: FreightCostAnalyticsLaneItemDTO): string {
  if (props.liveUnavailable) return t('freightCosts.unavailable.liveData')
  const quality = String(item.benchmark?.data_quality ?? '').toUpperCase()
  if (quality === 'INSUFFICIENT_SAMPLE' || quality === 'NOT_AVAILABLE') {
    return t('freightCosts.intelligence.states.insufficientSample')
  }
  return formatAnalyticsMoney(item.benchmark?.median, locale.value, unavailableLabel.value, props.mixedCurrency)
}

function moneyCell(money: Parameters<typeof formatAnalyticsMoney>[0]): string {
  if (props.liveUnavailable) return t('freightCosts.unavailable.liveData')
  return formatAnalyticsMoney(money, locale.value, unavailableLabel.value, props.mixedCurrency)
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
        <tr v-for="item in items" :key="item.lane_key" data-testid="freight-intelligence-list-row">
          <td>{{ item.lane_label }}</td>
          <td>{{ item.order_count }}</td>
          <td>{{ item.carrier_count }}</td>
          <td>{{ moneyCell(item.planned_total) }}</td>
          <td>{{ moneyCell(item.current_actual_total) }}</td>
          <td>{{ moneyCell(item.final_actual_total) }}</td>
          <td>{{ moneyCell(item.variance_total) }}</td>
          <td>{{ benchmarkMedian(item) }}</td>
          <td>
            <span>{{ item.benchmark?.sample_size ?? '—' }}</span>
            <Badge
              v-if="item.benchmark?.data_quality && item.benchmark.data_quality !== 'AVAILABLE'"
              :status="t(dataQualityLabelKey(item.benchmark.data_quality))"
              tone="info"
            />
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

td :deep(.ui-badge) {
  margin-left: 0.5rem;
}
</style>
