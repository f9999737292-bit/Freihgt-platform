package controltower

var controlTowerAccessRoles = map[string]struct{}{
	"PLATFORM_ADMIN":     {},
	"CARRIER_DISPATCHER": {},
	"SHIPPER_ADMIN":      {},
	"SHIPPER_LOGIST":     {},
	"FORWARDER_MANAGER":  {},
}

func CanAccessControlTower(roles []string) bool {
	return hasAnyRole(roles, controlTowerAccessRoles)
}

func CanAcknowledgeControlTower(roles []string) bool {
	return CanAccessControlTower(roles)
}

func CanAssignControlTower(roles []string) bool {
	return CanAccessControlTower(roles)
}

func CanResolveControlTower(roles []string) bool {
	return CanAccessControlTower(roles)
}

func CanManageException(roles []string) bool {
	return CanAccessControlTower(roles)
}

func hasAnyRole(roles []string, allowed map[string]struct{}) bool {
	for _, role := range roles {
		if _, ok := allowed[role]; ok {
			return true
		}
	}
	return false
}
