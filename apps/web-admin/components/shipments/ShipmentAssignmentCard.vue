<script setup lang="ts">
import type { Driver, Shipment, Vehicle } from '~/types/shipment'
import { canAssignDriver, canAssignVehicle } from '~/types/shipment'

const props = defineProps<{
  shipment: Shipment
  driver?: Driver | null
  vehicle?: Vehicle | null
  loadingDriver?: boolean
  loadingVehicle?: boolean
}>()

const emit = defineEmits<{
  assignDriver: []
  assignVehicle: []
}>()

const showAssignDriver = computed(() => canAssignDriver(props.shipment.status))
const showAssignVehicle = computed(() => canAssignVehicle(props.shipment.status))
</script>

<template>
  <UiCard>
    <template #header>
      <h3 class="card-title">{{ $t('shipments.assignment') }}</h3>
    </template>
    <div class="form-grid form-grid--2">
      <div>
        <span class="text-muted">{{ $t('shipments.driver') }}</span>
        <div v-if="loadingDriver" class="text-muted">{{ $t('common.loading') }}</div>
        <div v-else-if="driver">
          <div>{{ driver.full_name }}</div>
          <div class="text-sm text-muted">{{ driver.phone || driver.license_number || driver.id }}</div>
          <DriversDriverStatusBadge :status="driver.status" />
        </div>
        <div v-else>—</div>
        <UiButton
          v-if="showAssignDriver"
          size="sm"
          class="assign-btn"
          @click="emit('assignDriver')"
        >
          {{ $t('shipments.assignDriver') }}
        </UiButton>
      </div>

      <div>
        <span class="text-muted">{{ $t('shipments.vehicle') }}</span>
        <div v-if="loadingVehicle" class="text-muted">{{ $t('common.loading') }}</div>
        <div v-else-if="vehicle">
          <div>{{ vehicle.plate_number }}</div>
          <div class="text-sm text-muted">{{ vehicle.vehicle_type }} / {{ vehicle.equipment_type || '—' }}</div>
          <VehiclesVehicleStatusBadge :status="vehicle.status" />
        </div>
        <div v-else>—</div>
        <UiButton
          v-if="showAssignVehicle"
          size="sm"
          class="assign-btn"
          @click="emit('assignVehicle')"
        >
          {{ $t('shipments.assignVehicle') }}
        </UiButton>
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

.assign-btn {
  margin-top: 0.75rem;
}
</style>
