<script setup lang="ts">
import type { FreightCostAnalyticsAccessorialItemDTO } from '~/types/freightCost'
import { accessorialCategoryLabelKey } from '~/utils/freightCostWorkspace'
import {
  dataQualityLabelKey,
  formatAnalyticsMoney,
  formatAnalyticsRatio,
} from '~/utils/freightCostIntelligence'

const props = defineProps<{
  items: FreightCostAnalyticsAccessorialItemDTO[]
  mixedCurrency?: boolean
  liveUnavailable?: boolean
}>()

const { t, locale } = useI18n()

const unavailableLabel = computed(() => t('freightCosts.unavailable.money'))

const columns = computed(() => [
  t('freightCosts.intelligence.columns.category'),
  t('freightCosts.intelligence.columns.totalAmount'),
  t('freightCosts.intelligence.columns.orderCount'),
  t('freightCosts.intelligence.columns.lineCount'),
  t('freightCosts.intelligence.columns.shareOfSpend'),
  t('freightCosts.intelligence.columns.accessorialRate'),
  t('freightCosts.intelligence.columns.dataQuality'),
])

function moneyCell(money: Parameters<typeof formatAnalyticsMoney>[0]): string {
  if (props.liveUnavailable) return t('freightCosts.unavailable.liveData')
  return formatAnalyticsMoney(money, locale.value, unavailableLabel.value, props.mixedCurrency)
}

function categoryLabel(category: string): string {
  const key = accessorialCategoryLabelKey(category as Parameters<typeof accessorialCategoryLabelKey>[0])
  return t(key)
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
        <tr v-for="item in items" :key="item.normalized_category">
          <td>{{ categoryLabel(item.normalized_category) }}</td>
          <td>{{ moneyCell(item.total_amount) }}</td>
          <td>{{ item.order_count }}</td>
          <td>{{ item.line_count }}</td>
          <td>{{ formatAnalyticsRatio(item.share_of_spend, locale, unavailableLabel) }}</td>
          <td>{{ formatAnalyticsRatio(item.accessorial_order_rate, locale, unavailableLabel) }}</td>
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
