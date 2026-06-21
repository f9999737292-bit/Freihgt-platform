<script setup lang="ts">
defineProps<{ status: string }>()
</script>

<template>
  <span class="shipment-status-badge" :class="`shipment-status-badge--${status.toLowerCase()}`">
    {{ status }}
  </span>
</template>

<style scoped>
.shipment-status-badge {
  display: inline-flex;
  align-items: center;
  padding: 0.125rem 0.5rem;
  border-radius: 999px;
  font-size: 0.75rem;
  font-weight: 600;
  letter-spacing: 0.02em;
  text-transform: uppercase;
}

.shipment-status-badge--carrier_assigned,
.shipment-status-badge--accepted_by_carrier,
.shipment-status-badge--vehicle_assigned,
.shipment-status-badge--driver_assigned {
  background: #dbeafe;
  color: #1e40af;
}

.shipment-status-badge--pickup_slot_booked,
.shipment-status-badge--delivery_slot_booked,
.shipment-status-badge--in_pickup,
.shipment-status-badge--loaded,
.shipment-status-badge--in_transit {
  background: #fef3c7;
  color: #92400e;
}

.shipment-status-badge--arrived_at_consignee,
.shipment-status-badge--unloading,
.shipment-status-badge--delivered,
.shipment-status-badge--delivery_confirmed {
  background: #dcfce7;
  color: #166534;
}

.shipment-status-badge--documents_completed,
.shipment-status-badge--ready_for_billing,
.shipment-status-badge--included_in_billing_register,
.shipment-status-badge--financially_closed {
  background: #ede9fe;
  color: #5b21b6;
}

.shipment-status-badge--cancelled {
  background: #fee2e2;
  color: #991b1b;
}
</style>
