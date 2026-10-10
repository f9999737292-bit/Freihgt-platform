<script setup lang="ts">
import { displayFact, type ShipperShipment } from '../domain/shipment'

defineProps<{
  shipment: ShipperShipment
}>()

const { t } = useI18n()
</script>

<template>
  <dl class="facts" data-testid="shipment-facts">
    <div>
      <dt>{{ t('shipper.shipmentNumber') }}</dt>
      <dd data-testid="shipment-number">{{ displayFact(shipment.shipment_number) }}</dd>
    </div>
    <div>
      <dt>{{ t('shipper.status') }}</dt>
      <dd data-testid="shipment-status">{{ displayFact(shipment.status) }}</dd>
    </div>
    <div>
      <dt>{{ t('shipper.transportOrderId') }}</dt>
      <dd>{{ displayFact(shipment.transport_order_id) }}</dd>
    </div>
    <div>
      <dt>{{ t('shipper.shipperCompanyId') }}</dt>
      <dd>{{ displayFact(shipment.shipper_company_id) }}</dd>
    </div>
    <div>
      <dt>{{ t('shipper.consigneeCompanyId') }}</dt>
      <dd data-testid="consignee-company">{{ displayFact(shipment.consignee_company_id) }}</dd>
    </div>
    <div>
      <dt>{{ t('shipper.carrierCompanyId') }}</dt>
      <dd data-testid="carrier-company">{{ displayFact(shipment.carrier_company_id) }}</dd>
    </div>
    <div>
      <dt>{{ t('shipper.forwarderCompanyId') }}</dt>
      <dd>{{ displayFact(shipment.forwarder_company_id) }}</dd>
    </div>
    <div>
      <dt>{{ t('shipper.originLocationId') }}</dt>
      <dd>{{ displayFact(shipment.origin_location_id) }}</dd>
    </div>
    <div>
      <dt>{{ t('shipper.destinationLocationId') }}</dt>
      <dd>{{ displayFact(shipment.destination_location_id) }}</dd>
    </div>
    <div>
      <dt>{{ t('shipper.cargoId') }}</dt>
      <dd>{{ displayFact(shipment.cargo_id) }}</dd>
    </div>
    <div>
      <dt>{{ t('shipper.transportMode') }}</dt>
      <dd data-testid="transport-mode">{{ displayFact(shipment.transport_mode) }}</dd>
    </div>
    <div>
      <dt>{{ t('shipper.plannedPickup') }}</dt>
      <dd data-testid="planned-pickup">{{ displayFact(shipment.planned_pickup_at) }}</dd>
    </div>
    <div>
      <dt>{{ t('shipper.plannedDelivery') }}</dt>
      <dd data-testid="planned-delivery">{{ displayFact(shipment.planned_delivery_at) }}</dd>
    </div>
    <div>
      <dt>{{ t('shipper.actualPickup') }}</dt>
      <dd data-testid="actual-pickup">{{ displayFact(shipment.actual_pickup_at) }}</dd>
    </div>
    <div>
      <dt>{{ t('shipper.actualDelivery') }}</dt>
      <dd data-testid="actual-delivery">{{ displayFact(shipment.actual_delivery_at) }}</dd>
    </div>
  </dl>
</template>

<style scoped>
.facts {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(14rem, 1fr));
  gap: 0.75rem 1rem;
  margin: 0;
}
.facts dt {
  color: #6b7280;
  font-size: 0.75rem;
}
.facts dd {
  margin: 0.15rem 0 0;
}
</style>
