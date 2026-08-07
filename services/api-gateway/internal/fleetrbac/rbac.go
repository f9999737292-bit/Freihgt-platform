package fleetrbac

import "github.com/freight-platform/api-gateway/internal/routeauth"

var fleetViewRoles = map[string]struct{}{
	"PLATFORM_ADMIN":     {},
	"CARRIER_ADMIN":      {},
	"CARRIER_DISPATCHER": {},
}

var fleetCreateRoles = map[string]struct{}{
	"PLATFORM_ADMIN": {},
	"CARRIER_ADMIN":  {},
}

var fleetAssignRoles = map[string]struct{}{
	"PLATFORM_ADMIN":     {},
	"CARRIER_ADMIN":      {},
	"CARRIER_DISPATCHER": {},
}

func CanViewFleet(roles []string) bool {
	return routeauth.HasAnyRole(roles, fleetViewRoles)
}

func CanCreateFleet(roles []string) bool {
	return routeauth.HasAnyRole(roles, fleetCreateRoles)
}

func CanAssignFleet(roles []string) bool {
	return routeauth.HasAnyRole(roles, fleetAssignRoles)
}
