<script setup lang="ts">
import type { Vehicle } from '~/types/shipment'

defineProps<{ vehicle: Vehicle }>()
</script>

<template>
  <UiCard>
    <template #header>
      <h3 class="card-title">{{ $t('shipments.vehicleDetails') }}</h3>
    </template>
    <div class="form-grid form-grid--2">
      <div>
        <span class="text-muted">{{ $t('shipments.plateNumber') }}</span>
        <div>{{ vehicle.plate_number }}</div>
      </div>
      <div>
        <span class="text-muted">{{ $t('shipments.vehicleType') }}</span>
        <div>{{ vehicle.vehicle_type }}</div>
      </div>
      <div>
        <span class="text-muted">{{ $t('transportOrders.equipmentType') }}</span>
        <div>{{ vehicle.equipment_type || '—' }}</div>
      </div>
      <div>
        <span class="text-muted">{{ $t('shipments.capacityWeight') }}</span>
        <div>{{ vehicle.capacity_weight ?? '—' }}</div>
      </div>
      <div>
        <span class="text-muted">{{ $t('shipments.capacityVolume') }}</span>
        <div>{{ vehicle.capacity_volume ?? '—' }}</div>
      </div>
      <div>
        <span class="text-muted">{{ $t('common.status') }}</span>
        <div><VehiclesVehicleStatusBadge :status="vehicle.status" /></div>
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
