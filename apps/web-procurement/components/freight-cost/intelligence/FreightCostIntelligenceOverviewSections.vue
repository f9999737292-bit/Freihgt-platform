<script setup lang="ts">
import type { FreightCostAnalyticsOverviewDTO } from '~/types/freightCost'
import {
  formatAnalyticsMoney,
  formatOpportunityEstimatedDelta,
  getOverviewSummaryKpiKeys,
  getOverviewSummaryKpiValue,
  opportunityTypeLabelKey,
} from '~/utils/freightCostIntelligence'
import { formatDecimalMoney, isNullMoney } from '~/utils/freightCostMoney'

const props = defineProps<{
  overview: FreightCostAnalyticsOverviewDTO
  liveUnavailable?: boolean
}>()

const { t, locale } = useI18n()

const unavailableLabel = computed(() => t('freightCosts.unavailable.money'))
const mixedCurrency = computed(() => props.overview.mixed_currency)

const kpiKeys = computed(() => getOverviewSummaryKpiKeys())

function kpiLabelKey(key: string): string {
  if (key === 'order_count') return 'freightCosts.intelligence.summary.orderCount'
  if (key === 'reconciliation_mismatch_count') return 'freightCosts.kpi.reconciliation_mismatch_count'
  return `freightCosts.kpi.${key}`
}

function displayKpi(key: string): string {
  if (props.liveUnavailable) return t('freightCosts.unavailable.liveData')
  if (mixedCurrency.value && key !== 'reconciliation_mismatch_count' && key !== 'order_count') {
    return t('freightCosts.mixedCurrencyUnavailable')
  }
  const raw = getOverviewSummaryKpiValue(
    props.overview.summary,
    key as Parameters<typeof getOverviewSummaryKpiValue>[1],
    mixedCurrency.value,
  )
  if (key === 'reconciliation_mismatch_count' || key === 'order_count') {
    return raw == null ? unavailableLabel.value : String(raw)
  }
  if (isNullMoney(typeof raw === 'string' ? raw : null)) return unavailableLabel.value
  return formatDecimalMoney(String(raw), props.overview.currency_code, locale.value)
}

function isKpiUnavailable(key: string): boolean {
  if (props.liveUnavailable || mixedCurrency.value) {
    return key !== 'reconciliation_mismatch_count' && key !== 'order_count'
  }
  if (key === 'reconciliation_mismatch_count' || key === 'order_count') return false
  const raw = getOverviewSummaryKpiValue(
    props.overview.summary,
    key as Parameters<typeof getOverviewSummaryKpiValue>[1],
    mixedCurrency.value,
  )
  return isNullMoney(typeof raw === 'string' ? raw : null)
}
</script>

<template>
  <div class="intelligence-overview">
    <section class="intelligence-overview__section">
      <h2>{{ t('freightCosts.intelligence.sections.summary') }}</h2>
      <div class="intelligence-overview__kpis">
        <KpiCard
          v-for="key in kpiKeys"
          :key="key"
          :title="t(kpiLabelKey(key))"
          :value="displayKpi(key)"
          :unavailable="isKpiUnavailable(key)"
          :helper="mixedCurrency && key !== 'reconciliation_mismatch_count' && key !== 'order_count'
            ? t('freightCosts.mixedCurrencyUnavailable')
            : undefined"
        />
      </div>
    </section>

    <section v-if="overview.top_lanes?.length" class="intelligence-overview__section">
      <h2>{{ t('freightCosts.intelligence.sections.topLanes') }}</h2>
      <Table :columns="[
        t('freightCosts.intelligence.columns.lane'),
        t('freightCosts.intelligence.columns.orderCount'),
        t('freightCosts.intelligence.columns.spendTotal'),
      ]">
        <tr v-for="lane in overview.top_lanes" :key="lane.lane_key">
          <td>{{ lane.lane_label }}</td>
          <td>{{ lane.order_count }}</td>
          <td>{{ formatAnalyticsMoney(lane.spend_total, locale, unavailableLabel, mixedCurrency) }}</td>
        </tr>
      </Table>
    </section>

    <section v-if="overview.accessorial" class="intelligence-overview__section">
      <h2>{{ t('freightCosts.intelligence.sections.accessorial') }}</h2>
      <div class="intelligence-overview__kpis">
        <KpiCard
          :title="t('freightCosts.intelligence.summary.accessorialTotal')"
          :value="formatAnalyticsMoney(overview.accessorial.total_amount, locale, unavailableLabel, mixedCurrency)"
          :unavailable="mixedCurrency || isNullMoney(overview.accessorial.total_amount.amount)"
        />
        <KpiCard
          :title="t('freightCosts.intelligence.summary.accessorialOrders')"
          :value="String(overview.accessorial.order_count)"
        />
      </div>
    </section>

    <section v-if="overview.opportunities" class="intelligence-overview__section">
      <h2>{{ t('freightCosts.intelligence.sections.opportunities') }}</h2>
      <KpiCard
        :title="t('freightCosts.intelligence.summary.opportunityCount')"
        :value="String(overview.opportunities.count)"
      />
      <Table
        v-if="overview.opportunities.top_items.length"
        :columns="[
          t('freightCosts.intelligence.columns.opportunityType'),
          t('freightCosts.intelligence.columns.estimatedDelta'),
          t('freightCosts.intelligence.columns.sampleSize'),
        ]"
      >
        <tr v-for="item in overview.opportunities.top_items" :key="item.opportunity_id">
          <td>{{ t(opportunityTypeLabelKey(item.type)) }}</td>
          <td>{{ formatOpportunityEstimatedDelta(item, locale, unavailableLabel) }}</td>
          <td>{{ item.sample_size }}</td>
        </tr>
      </Table>
      <p v-if="overview.opportunities.top_items.length" class="intelligence-overview__hint">
        {{ t('freightCosts.intelligence.hints.backendSavingsOnly') }}
      </p>
    </section>
  </div>
</template>

<style scoped>
.intelligence-overview {
  display: flex;
  flex-direction: column;
  gap: 1.5rem;
}

.intelligence-overview__section h2 {
  margin: 0 0 0.75rem;
  font-size: 1rem;
  font-weight: 600;
}

.intelligence-overview__kpis {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(220px, 1fr));
  gap: 1rem;
}

.intelligence-overview__hint {
  margin: 0.75rem 0 0;
  font-size: 0.8125rem;
  color: var(--color-text-muted);
}
</style>
