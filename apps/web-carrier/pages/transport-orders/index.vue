<script setup lang="ts">
import { Card, CustomerPageHeader, Table } from '@freight-platform/ui'
import PortalState from '../../components/PortalState.vue'
import { viewKindFromError, type PortalViewKind } from '../../domain/viewState'
import type { TransportOrderSummary } from '../../composables/useCarrierApi'

const { t } = useI18n()
const localePath = useLocalePath()
const api = useCarrierApi()
const items = ref<TransportOrderSummary[]>([])
const kind = ref<PortalViewKind>('loading')
const detail = ref('')

async function load() {
  kind.value = 'loading'
  items.value = []
  try {
    const data = await api.listTransportOrders()
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
  if (state === 'empty') return t('carrier.emptyOrders')
  if (state === 'forbidden') return t('carrier.forbidden')
  if (state === 'not_found') return t('carrier.notFound')
  if (state === 'loading') return t('carrier.loading')
  return t('carrier.unavailable')
}
</script>

<template>
  <section data-testid="transport-orders">
    <CustomerPageHeader :title="t('carrier.ordersTitle')" />
    <PortalState v-if="kind !== 'ready'" :kind="kind" :title="titleFor(kind)" :description="detail" />
    <Card v-else>
      <Table :columns="[t('carrier.ordersTitle'), t('carrier.status'), '']">
        <tr v-for="item in items" :key="item.transport_order_id">
          <td>{{ item.transport_order_number }}</td>
          <td>{{ item.transport_order_status }}</td>
          <td>
            <NuxtLink :to="localePath(`/transport-orders/${item.transport_order_id}`)" data-testid="open-order">
              {{ t('carrier.open') }}
            </NuxtLink>
          </td>
        </tr>
      </Table>
    </Card>
  </section>
</template>
