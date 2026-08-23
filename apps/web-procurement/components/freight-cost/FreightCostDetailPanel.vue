<script setup lang="ts">
import type { FreightCostDetailVM } from '~/types/freightCost'
import type { FreightCostActor } from '~/types/freightCost'
import { getFreightCostDetailSections } from '~/utils/freightCostWorkspace'
import { formatDecimalMoney, isNullMoney } from '~/utils/freightCostMoney'

const props = defineProps<{
  actor: FreightCostActor
  detail: FreightCostDetailVM | null
  liveUnavailable?: boolean
}>()

const { t, locale } = useI18n()

const sections = computed(() => getFreightCostDetailSections(props.actor))

function money(value: string | null | undefined, currency: string) {
  if (props.liveUnavailable) return t('freightCosts.unavailable.liveData')
  if (isNullMoney(value)) return t('freightCosts.unavailable.money')
  return formatDecimalMoney(value, currency, locale.value)
}
</script>

<template>
  <div v-if="detail" class="detail-panel">
    <section
      v-for="section in sections"
      :key="section.key"
      class="detail-panel__section"
    >
      <h2>{{ t(section.labelKey) }}</h2>
      <template v-if="section.key === 'summary'">
        <p>{{ t('freightCosts.columns.planned') }}: {{ money(detail.summary.planned_amount, detail.summary.currency_code) }}</p>
        <p>{{ t('freightCosts.columns.currentActual') }}: {{ money(detail.summary.current_actual_amount, detail.summary.currency_code) }}</p>
      </template>
      <template v-else-if="section.key === 'planned_snapshot'">
        <p>{{ detail.planned_source ?? t('freightCosts.unavailable.money') }}</p>
      </template>
      <template v-else-if="section.key === 'forecast_exposure'">
        <p>{{ money(detail.summary.forecast_exposure, detail.summary.currency_code) }}</p>
      </template>
      <template v-else-if="section.key === 'variance_drivers'">
        <ul v-if="detail.variance_drivers.length">
          <li v-for="driver in detail.variance_drivers" :key="`${driver.driver_type}-${driver.description}`">
            {{ driver.description }}
          </li>
        </ul>
        <p v-else>{{ t('freightCosts.empty.varianceDrivers') }}</p>
      </template>
      <template v-else-if="section.key === 'reconciliation'">
        <Badge>{{ detail.summary.billing_reconciliation_status ?? t('freightCosts.unavailable.money') }}</Badge>
      </template>
      <template v-else-if="section.key === 'provenance'">
        <p>{{ detail.summary.sources_available.join(', ') || t('freightCosts.unavailable.money') }}</p>
        <p>{{ detail.summary.cost_updated_at }}</p>
      </template>
    </section>
  </div>
</template>

<style scoped>
.detail-panel {
  display: flex;
  flex-direction: column;
  gap: 1rem;
}

.detail-panel__section {
  padding: 1rem;
  border: 1px solid var(--color-border);
  border-radius: var(--radius-lg);
  background: var(--color-surface);
}

.detail-panel__section h2 {
  margin: 0 0 0.75rem;
  font-size: 1rem;
}
</style>
