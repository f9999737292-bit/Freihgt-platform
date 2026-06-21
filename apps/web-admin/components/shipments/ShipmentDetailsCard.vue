<script setup lang="ts">
import { formatShipmentDate, type Shipment } from '~/types/shipment'

defineProps<{ shipment: Shipment }>()
</script>

<template>
  <UiCard>
    <template #header>
      <h3 class="card-title">{{ $t('shipments.details') }}</h3>
    </template>
    <div class="form-grid form-grid--2">
      <div>
        <span class="text-muted">{{ $t('shipments.shipmentNumber') }}</span>
        <div>{{ shipment.shipment_number }}</div>
      </div>
      <div>
        <span class="text-muted">{{ $t('common.status') }}</span>
        <div><ShipmentsShipmentStatusBadge :status="shipment.status" /></div>
      </div>
      <div>
        <span class="text-muted">{{ $t('freightRequests.transportOrder') }}</span>
        <div>
          <NuxtLink v-if="shipment.transport_order_id" :to="`/transport-orders/${shipment.transport_order_id}`">
            {{ shipment.transport_order_id }}
          </NuxtLink>
          <span v-else>—</span>
        </div>
      </div>
      <div>
        <span class="text-muted">{{ $t('freightRequests.createdAt') }}</span>
        <div>{{ formatShipmentDate(shipment.created_at) }}</div>
      </div>
      <div>
        <span class="text-muted">{{ $t('companies.updatedAt') }}</span>
        <div>{{ formatShipmentDate(shipment.updated_at) }}</div>
      </div>
      <div>
        <span class="text-muted">{{ $t('shipments.plannedPickup') }}</span>
        <div>{{ formatShipmentDate(shipment.planned_pickup_at) }}</div>
      </div>
      <div>
        <span class="text-muted">{{ $t('shipments.plannedDelivery') }}</span>
        <div>{{ formatShipmentDate(shipment.planned_delivery_at) }}</div>
      </div>
      <div>
        <span class="text-muted">{{ $t('shipments.actualPickup') }}</span>
        <div>{{ formatShipmentDate(shipment.actual_pickup_at) }}</div>
      </div>
      <div>
        <span class="text-muted">{{ $t('shipments.actualDelivery') }}</span>
        <div>{{ formatShipmentDate(shipment.actual_delivery_at) }}</div>
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
