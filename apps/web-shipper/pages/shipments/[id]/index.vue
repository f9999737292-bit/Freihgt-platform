<script setup lang="ts">
import { Card, CustomerPageHeader } from '@freight-platform/ui'
import PortalState from '../../../components/PortalState.vue'
import ShipmentFacts from '../../../components/ShipmentFacts.vue'
import type { ShipperShipment } from '../../../domain/shipment'
import { viewKindFromError, type PortalViewKind } from '../../../domain/viewState'

const route = useRoute()
const { t } = useI18n()
const localePath = useLocalePath()
const api = useShipperApi()
const shipment = ref<ShipperShipment | null>(null)
const kind = ref<PortalViewKind>('loading')
const detail = ref('')

async function load() {
  kind.value = 'loading'
  shipment.value = null
  detail.value = ''
  try {
    shipment.value = await api.getShipment(String(route.params.id))
    kind.value = 'ready'
  } catch (error) {
    shipment.value = null
    kind.value = viewKindFromError(error)
    detail.value = error instanceof Error ? error.message : ''
  }
}

onMounted(load)

function titleFor(state: PortalViewKind) {
  if (state === 'forbidden') return t('shipper.forbidden')
  if (state === 'not_found') return t('shipper.notFound')
  if (state === 'unauthorized') return t('shipper.forbidden')
  if (state === 'loading') return t('shipper.loading')
  return t('shipper.unavailable')
}
</script>

<template>
  <section data-testid="shipment-detail">
    <PortalState v-if="kind !== 'ready'" :kind="kind" :title="titleFor(kind)" :description="detail" />
    <template v-else-if="shipment">
      <CustomerPageHeader :title="shipment.shipment_number || t('shipper.shipmentDetail')" />
      <p>
        <NuxtLink :to="localePath(`/shipments/${shipment.id}/tracking`)" data-testid="open-tracking">
          {{ t('shipper.openTracking') }}
        </NuxtLink>
      </p>
      <Card>
        <ShipmentFacts :shipment="shipment" />
      </Card>
    </template>
  </section>
</template>

<style scoped>
a:focus-visible {
  outline: 2px solid #0f3d4c;
  outline-offset: 2px;
}
</style>
