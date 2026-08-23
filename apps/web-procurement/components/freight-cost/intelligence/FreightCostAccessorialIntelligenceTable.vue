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
  <Table :columns="columns">
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
  </Table>
</template>
