<script setup lang="ts">
import { formatShipmentDate } from '~/types/shipment'
import type { ControlTowerShipment } from '~/types/controlTower'

defineProps<{
  rows: ControlTowerShipment[]
  loading?: boolean
}>()

const { t } = useI18n()

const columns = computed(() => [
  t('controlTower.table.shipmentNumber'),
  t('controlTower.table.transportOrder'),
  t('controlTower.table.shipper'),
  t('controlTower.table.carrier'),
  t('controlTower.table.route'),
  t('controlTower.table.plannedPickup'),
  t('controlTower.table.plannedDelivery'),
  t('controlTower.table.status'),
  t('controlTower.table.slaStatus'),
  t('controlTower.table.lastEvent'),
  t('controlTower.table.lastUpdated'),
  t('common.actions'),
])
</script>

<template>
  <UiTable :columns="columns" :loading="loading">
    <tr v-for="row in rows" :key="row.id">
      <td>
        <NuxtLink :to="`/shipments/${row.id}`">{{ row.shipmentNumber }}</NuxtLink>
      </td>
      <td>
        <NuxtLink
          v-if="row.transportOrderId"
          :to="`/transport-orders/${row.transportOrderId}`"
        >
          {{ row.transportOrderNumber || row.transportOrderId }}
        </NuxtLink>
        <span v-else>—</span>
      </td>
      <td>{{ row.shipperName || '—' }}</td>
      <td>{{ row.carrierName || '—' }}</td>
      <td>{{ row.route || '—' }}</td>
      <td>{{ formatShipmentDate(row.plannedPickupAt) }}</td>
      <td>{{ formatShipmentDate(row.plannedDeliveryAt) }}</td>
      <td><ShipmentsShipmentStatusBadge :status="row.status" /></td>
      <td><ControlTowerSlaStatusBadge :status="row.slaStatus" /></td>
      <td>{{ row.lastEvent || '—' }}</td>
      <td>{{ formatShipmentDate(row.lastUpdatedAt) }}</td>
      <td class="actions-cell">
        <NuxtLink :to="`/shipments/${row.id}`">{{ $t('controlTower.actions.openShipment') }}</NuxtLink>
        <NuxtLink
          v-if="row.transportOrderId"
          :to="`/transport-orders/${row.transportOrderId}`"
        >
          {{ $t('controlTower.actions.openOrder') }}
        </NuxtLink>
        <!-- TODO: dedicated shipment event history route when available -->
        <NuxtLink :to="`/shipments/${row.id}#events`">
          {{ $t('controlTower.actions.eventHistory') }}
        </NuxtLink>
      </td>
    </tr>
  </UiTable>
</template>

<style scoped>
.actions-cell {
  display: flex;
  flex-direction: column;
  gap: 0.25rem;
  font-size: 0.8125rem;
}
</style>
