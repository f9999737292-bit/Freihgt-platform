package shipmentevents

var shipmentViewRoles = map[string]struct{}{
	"PLATFORM_ADMIN":     {},
	"SHIPPER_ADMIN":      {},
	"SHIPPER_LOGIST":     {},
	"CARRIER_ADMIN":      {},
	"CARRIER_DISPATCHER": {},
	"FORWARDER_MANAGER":  {},
	"CONSIGNEE_OPERATOR": {},
	"CONSIGNEE_VIEWER":   {},
}

func CanViewShipmentEvents(roles []string) bool {
	for _, role := range roles {
		if _, ok := shipmentViewRoles[role]; ok {
			return true
		}
	}
	return false
}
