<script setup lang="ts">
import type { Shipment } from '~/types/shipment'

defineProps<{
  shipment: Shipment
  companyName: (id?: string | null) => string
}>()
</script>

<template>
  <UiCard>
    <template #header>
      <h3 class="card-title">{{ $t('shipments.parties') }}</h3>
    </template>
    <div class="form-grid form-grid--2">
      <div>
        <span class="text-muted">{{ $t('transportOrders.shipper') }}</span>
        <div>{{ companyName(shipment.shipper_company_id) }}</div>
      </div>
      <div>
        <span class="text-muted">{{ $t('transportOrders.consignee') }}</span>
        <div>{{ companyName(shipment.consignee_company_id) }}</div>
      </div>
      <div>
        <span class="text-muted">{{ $t('shipments.carrier') }}</span>
        <div>{{ companyName(shipment.carrier_company_id) }}</div>
      </div>
      <div>
        <span class="text-muted">{{ $t('shipments.forwarder') }}</span>
        <div>{{ companyName(shipment.forwarder_company_id) || '—' }}</div>
      </div>
    </div>
  </UiCard>
</template>

<style scoped>
.card-title {
  margin: 0;
  font-size: 1rem;
  font-weight: 600;
}
</style>
