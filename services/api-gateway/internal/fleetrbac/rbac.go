package fleetrbac

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
	return hasAnyRole(roles, fleetViewRoles)
}

func CanCreateFleet(roles []string) bool {
	return hasAnyRole(roles, fleetCreateRoles)
}

func CanAssignFleet(roles []string) bool {
	return hasAnyRole(roles, fleetAssignRoles)
}

func hasAnyRole(roles []string, allowed map[string]struct{}) bool {
	for _, role := range roles {
		if _, ok := allowed[role]; ok {
			return true
		}
	}
	return false
}
