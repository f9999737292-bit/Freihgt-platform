package shipmentrbac

import "github.com/freight-platform/api-gateway/internal/routeauth"

var createShipmentRoles = map[string]struct{}{
	"PLATFORM_ADMIN":    {},
	"SHIPPER_ADMIN":     {},
	"SHIPPER_LOGIST":    {},
	"FORWARDER_MANAGER": {},
}

var acceptShipmentRoles = map[string]struct{}{
	"PLATFORM_ADMIN":     {},
	"CARRIER_ADMIN":      {},
	"CARRIER_DISPATCHER": {},
}

var updateShipmentStatusRoles = map[string]struct{}{
	"PLATFORM_ADMIN":     {},
	"CARRIER_ADMIN":      {},
	"CARRIER_DISPATCHER": {},
}

var cancelShipmentRoles = map[string]struct{}{
	"PLATFORM_ADMIN":    {},
	"SHIPPER_ADMIN":     {},
	"SHIPPER_LOGIST":    {},
	"FORWARDER_MANAGER": {},
}

func CanCreateShipment(roles []string) bool {
	return routeauth.HasAnyRole(roles, createShipmentRoles)
}

func CanAcceptShipment(roles []string) bool {
	return routeauth.HasAnyRole(roles, acceptShipmentRoles)
}

func CanUpdateShipmentStatus(roles []string) bool {
	return routeauth.HasAnyRole(roles, updateShipmentStatusRoles)
}

func CanCancelShipment(roles []string) bool {
	return routeauth.HasAnyRole(roles, cancelShipmentRoles)
}
