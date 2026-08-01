package controltowerreadmodel

var knownShipmentStatuses = map[string]struct{}{
	"CARRIER_ASSIGNED": {}, "ACCEPTED_BY_CARRIER": {}, "VEHICLE_ASSIGNED": {},
	"DRIVER_ASSIGNED": {}, "PICKUP_SLOT_BOOKED": {}, "DELIVERY_SLOT_BOOKED": {},
	"IN_PICKUP": {}, "LOADED": {}, "IN_TRANSIT": {}, "ARRIVED_AT_CONSIGNEE": {},
	"UNLOADING": {}, "DELIVERED": {}, "DELIVERY_CONFIRMED": {},
	"DOCUMENTS_COMPLETED": {}, "READY_FOR_BILLING": {}, "INCLUDED_IN_BILLING_REGISTER": {},
	"FINANCIALLY_CLOSED": {}, "CANCELLED": {},
}

func IsKnownShipmentStatus(status string) bool {
	_, ok := knownShipmentStatuses[status]
	return ok
}
