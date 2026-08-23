<script setup lang="ts">
import type { FreightCostActor, FreightCostOrderRowVM } from '~/types/freightCost'
import type { FreightCostTableColumn } from '~/utils/freightCostWorkspace'
import { formatFreightCostDisplayLabel, getPlannedVsActualColumns, shouldShowFreightCostField } from '~/utils/freightCostWorkspace'
import { formatDecimalMoney, isNullMoney } from '~/utils/freightCostMoney'
import { finalityLabelKey, reconciliationLabelKey } from '~/utils/freightCostWorkspace'

const props = defineProps<{
  actor: FreightCostActor
  rows: FreightCostOrderRowVM[]
  loading?: boolean
  liveUnavailable?: boolean
}>()

const { t, locale } = useI18n()

const columns = computed(() => getPlannedVsActualColumns(props.actor))

function cellValue(row: FreightCostOrderRowVM, column: FreightCostTableColumn): string {
  if (props.liveUnavailable) return t('freightCosts.unavailable.liveData')
  const key = column.key as keyof FreightCostOrderRowVM
  if (!shouldShowFreightCostField(key, props.actor)) return t('freightCosts.unavailable.masked')

  const raw = row[key]
  if (key === 'financial_finality') return t(finalityLabelKey(raw as FreightCostOrderRowVM['financial_finality']))
  if (key === 'billing_reconciliation_status') {
    return t(reconciliationLabelKey(raw as FreightCostOrderRowVM['billing_reconciliation_status']))
  }
  if (typeof raw === 'string' && (key === 'order_reference' || key === 'carrier_name')) {
    const label = formatFreightCostDisplayLabel(raw)
    return label || t('freightCosts.unavailable.reference')
  }
  if (typeof raw === 'string' && (key.includes('amount') || key === 'forecast_exposure')) {
    if (isNullMoney(raw)) return t('freightCosts.unavailable.money')
    return formatDecimalMoney(raw, row.currency_code, locale.value)
  }
  return raw == null ? t('freightCosts.unavailable.money') : String(raw)
}

const headerLabels = computed(() => columns.value.map((column) => t(column.labelKey)))
</script>

<template>
  <Table :columns="headerLabels" :loading="loading">
    <tr v-for="row in rows" :key="row.transport_order_id">
      <td v-for="column in columns" :key="column.key">
        <NuxtLink
          v-if="column.key === 'order_reference'"
          :to="`/freight-costs/shipments/${row.transport_order_id}`"
        >
          {{ cellValue(row, column) }}
        </NuxtLink>
        <span v-else>{{ cellValue(row, column) }}</span>
      </td>
    </tr>
  </Table>
</template>
