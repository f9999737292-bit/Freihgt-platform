package domain

// ShipmentStatusCreated is the database default for new rows; domain create path uses CARRIER_ASSIGNED.
const ShipmentStatusCreated = "CREATED"

var knownShipmentStatuses = map[string]struct{}{
	ShipmentStatusCreated:                   {},
	ShipmentStatusCarrierAssigned:           {},
	ShipmentStatusAcceptedByCarrier:         {},
	ShipmentStatusVehicleAssigned:           {},
	ShipmentStatusDriverAssigned:            {},
	ShipmentStatusPickupSlotBooked:          {},
	ShipmentStatusDeliverySlotBooked:        {},
	ShipmentStatusInPickup:                  {},
	ShipmentStatusLoaded:                    {},
	ShipmentStatusInTransit:                 {},
	ShipmentStatusArrivedAtConsignee:        {},
	ShipmentStatusUnloading:                 {},
	ShipmentStatusDelivered:                 {},
	ShipmentStatusDeliveryConfirmed:         {},
	ShipmentStatusDocumentsCompleted:        {},
	ShipmentStatusReadyForBilling:           {},
	ShipmentStatusIncludedInBillingRegister: {},
	ShipmentStatusFinanciallyClosed:         {},
	ShipmentStatusCancelled:                 {},
}

func IsValidShipmentStatus(status string) bool {
	_, ok := knownShipmentStatuses[status]
	return ok
}
