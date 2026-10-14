<script setup lang="ts">
import { Card, CustomerPageHeader } from '@freight-platform/ui'
import PortalState from '../../components/PortalState.vue'
import { viewKindFromError, type PortalViewKind } from '../../domain/viewState'
import { companyMayReadDocuments } from '../../domain/document'
import type { TransportOrderSummary } from '../../composables/useCarrierApi'

const route = useRoute()
const { t } = useI18n()
const localePath = useLocalePath()
const api = useCarrierApi()
const office = useCarrierOffice()
const canReadDocuments = computed(() => {
  const selected = office.companies.value.find((item) => item.companyId === office.selectedCompanyId.value)
  return selected ? companyMayReadDocuments(selected.roleCodes) : false
})
const order = ref<TransportOrderSummary | null>(null)
const kind = ref<PortalViewKind>('loading')
const detail = ref('')

async function load() {
  kind.value = 'loading'
  order.value = null
  try {
    order.value = await api.getTransportOrder(String(route.params.id))
    kind.value = 'ready'
  } catch (error) {
    order.value = null
    kind.value = viewKindFromError(error)
    detail.value = error instanceof Error ? error.message : ''
  }
}

onMounted(load)

function titleFor(state: PortalViewKind) {
  if (state === 'forbidden') return t('carrier.forbidden')
  if (state === 'not_found') return t('carrier.notFound')
  if (state === 'loading') return t('carrier.loading')
  return t('carrier.unavailable')
}
</script>

<template>
  <section data-testid="transport-order-detail">
    <PortalState v-if="kind !== 'ready'" :kind="kind" :title="titleFor(kind)" :description="detail" />
    <template v-else-if="order">
      <CustomerPageHeader :title="order.transport_order_number || t('carrier.orderDetail')" />
      <Card>
        <p data-testid="order-status">{{ order.transport_order_status }}</p>
        <p v-if="order.shipment?.status" data-testid="shipment-status">{{ order.shipment.status }}</p>
        <p v-if="order.provenance" data-testid="order-amount">{{ order.provenance.amount }} {{ order.provenance.currency_code }}</p>
        <NuxtLink
          v-if="order.shipment?.id"
          :to="localePath(`/shipments/${order.shipment.id}/tracking`)"
          data-testid="open-tracking"
        >
          {{ t('carrier.openTracking') }}
        </NuxtLink>
        <NuxtLink
          v-if="order.shipment?.id && canReadDocuments"
          :to="localePath(`/shipments/${order.shipment.id}/documents`)"
          data-testid="open-documents"
        >
          {{ t('carrier.openDocuments') }}
        </NuxtLink>
      </Card>
    </template>
  </section>
</template>
