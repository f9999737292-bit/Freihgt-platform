<script setup lang="ts">
import { Button, Card, CustomerPageHeader, Input, Table } from '@freight-platform/ui'
import PortalState from '../../components/PortalState.vue'
import { displayFact, type ShipperShipment } from '../../domain/shipment'
import { viewKindFromError, type PortalViewKind } from '../../domain/viewState'

const { t } = useI18n()
const localePath = useLocalePath()
const api = useShipperApi()
const items = ref<ShipperShipment[]>([])
const kind = ref<PortalViewKind>('loading')
const detail = ref('')
const statusFilter = ref('')

async function load() {
  kind.value = 'loading'
  items.value = []
  detail.value = ''
  try {
    const data = await api.listShipments(statusFilter.value)
    items.value = data.items ?? []
    kind.value = items.value.length === 0 ? 'empty' : 'ready'
  } catch (error) {
    items.value = []
    kind.value = viewKindFromError(error)
    detail.value = error instanceof Error ? error.message : ''
  }
}

onMounted(load)

function titleFor(state: PortalViewKind) {
  if (state === 'empty') return t('shipper.emptyShipments')
  if (state === 'forbidden') return t('shipper.forbidden')
  if (state === 'not_found') return t('shipper.notFound')
  if (state === 'unauthorized') return t('shipper.forbidden')
  if (state === 'loading') return t('shipper.loading')
  return t('shipper.unavailable')
}
</script>

<template>
  <section data-testid="shipment-inbox">
    <CustomerPageHeader :title="t('shipper.shipmentsTitle')" />
    <form class="filter" data-testid="status-filter-form" @submit.prevent="load">
      <Input v-model="statusFilter" name="status" data-testid="status-filter" :label="t('shipper.statusFilter')" />
      <Button type="submit" data-testid="apply-status">{{ t('shipper.apply') }}</Button>
    </form>
    <PortalState v-if="kind !== 'ready'" :kind="kind" :title="titleFor(kind)" :description="detail" />
    <Card v-else>
      <Table
        :columns="[
          t('shipper.shipmentNumber'),
          t('shipper.status'),
          t('shipper.carrierCompanyId'),
          t('shipper.consigneeCompanyId'),
          t('shipper.transportMode'),
          t('shipper.plannedPickup'),
          t('shipper.plannedDelivery'),
          t('shipper.actualPickup'),
          t('shipper.actualDelivery'),
          '',
        ]"
      >
        <tr v-for="item in items" :key="item.id">
          <td>{{ displayFact(item.shipment_number) }}</td>
          <td>{{ displayFact(item.status) }}</td>
          <td>{{ displayFact(item.carrier_company_id) }}</td>
          <td>{{ displayFact(item.consignee_company_id) }}</td>
          <td>{{ displayFact(item.transport_mode) }}</td>
          <td>{{ displayFact(item.planned_pickup_at) }}</td>
          <td>{{ displayFact(item.planned_delivery_at) }}</td>
          <td>{{ displayFact(item.actual_pickup_at) }}</td>
          <td>{{ displayFact(item.actual_delivery_at) }}</td>
          <td>
            <NuxtLink :to="localePath(`/shipments/${item.id}`)" data-testid="open-shipment">
              {{ t('shipper.open') }}
            </NuxtLink>
          </td>
        </tr>
      </Table>
    </Card>
  </section>
</template>

<style scoped>
.filter {
  display: flex;
  align-items: flex-end;
  gap: 0.75rem;
  max-width: 24rem;
  margin-bottom: 1rem;
}
</style>
