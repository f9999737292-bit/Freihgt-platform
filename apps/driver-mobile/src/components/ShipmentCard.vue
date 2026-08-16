<script setup lang="ts">
import type { DriverShipmentSummary } from '@/types/driver'
import { useI18n } from 'vue-i18n'

defineProps<{
  shipment: DriverShipmentSummary
}>()

defineEmits<{
  open: []
}>()

const { t } = useI18n()

function formatWhen(value?: string) {
  if (!value) return '—'
  try {
    return new Intl.DateTimeFormat('ru-RU', {
      day: '2-digit',
      month: '2-digit',
      hour: '2-digit',
      minute: '2-digit',
    }).format(new Date(value))
  } catch {
    return value
  }
}
</script>

<template>
  <ion-card button class="shipment-card" @click="$emit('open')">
    <ion-card-header>
      <ion-card-subtitle>{{ t('shipments.shipmentNumber') }}</ion-card-subtitle>
      <ion-card-title>{{ shipment.shipmentNumber }}</ion-card-title>
    </ion-card-header>
    <ion-card-content>
      <p><strong>{{ t('shipments.status') }}:</strong> {{ shipment.status }}</p>
      <p><strong>{{ t('shipments.pickup') }}:</strong> {{ formatWhen(shipment.plannedPickupAt) }}</p>
      <p><strong>{{ t('shipments.delivery') }}:</strong> {{ formatWhen(shipment.plannedDeliveryAt) }}</p>
      <ion-button expand="block" size="large" class="open-btn">{{ t('shipments.open') }}</ion-button>
    </ion-card-content>
  </ion-card>
</template>

<style scoped>
.shipment-card {
  margin-bottom: 12px;
}
.open-btn {
  margin-top: 12px;
  min-height: 48px;
}
</style>
