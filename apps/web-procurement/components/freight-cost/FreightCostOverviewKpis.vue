<script setup lang="ts">
import type {
  FreightCostActor,
  FreightCostSummaryAggregateDTO,
} from '~/types/freightCost'
import {
  getOverviewKpiKeysForActor,
  getOverviewKpiLabelKey,
  getOverviewKpiValue,
} from '~/utils/freightCostWorkspace'
import { formatDecimalMoney, isNullMoney } from '~/utils/freightCostMoney'

const props = defineProps<{
  actor: FreightCostActor
  aggregate: FreightCostSummaryAggregateDTO | null
  mixedCurrency: boolean
  liveUnavailable: boolean
}>()

const { t, locale } = useI18n()

const kpiKeys = computed(() => getOverviewKpiKeysForActor(props.actor))

function displayValue(key: string) {
  if (props.liveUnavailable) return t('freightCosts.unavailable.liveData')
  if (props.mixedCurrency && key !== 'reconciliation_mismatch_count') {
    return t('freightCosts.mixedCurrencyUnavailable')
  }
  const raw = getOverviewKpiValue(props.aggregate, key as Parameters<typeof getOverviewKpiValue>[1])
  if (key === 'reconciliation_mismatch_count') {
    return raw == null ? t('freightCosts.unavailable.money') : String(raw)
  }
  if (isNullMoney(typeof raw === 'string' ? raw : null)) {
    return t('freightCosts.unavailable.money')
  }
  return formatDecimalMoney(String(raw), props.aggregate?.currency_code ?? '', locale.value)
}

function isUnavailable(key: string) {
  if (props.liveUnavailable || props.mixedCurrency) return true
  if (key === 'reconciliation_mismatch_count') return false
  const raw = getOverviewKpiValue(props.aggregate, key as Parameters<typeof getOverviewKpiValue>[1])
  return isNullMoney(typeof raw === 'string' ? raw : null)
}
</script>

<template>
  <div class="overview-kpis" role="group" :aria-label="t('freightCosts.nav.overview')">
    <KpiCard
      v-for="key in kpiKeys"
      :key="key"
      :title="t(getOverviewKpiLabelKey(key))"
      :value="displayValue(key)"
      :unavailable="isUnavailable(key)"
      :helper="mixedCurrency && key !== 'reconciliation_mismatch_count' ? t('freightCosts.mixedCurrencyUnavailable') : undefined"
    />
  </div>
</template>

<style scoped>
.overview-kpis {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(220px, 1fr));
  gap: 1rem;
}
</style>
