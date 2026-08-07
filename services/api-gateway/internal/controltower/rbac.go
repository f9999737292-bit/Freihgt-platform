package controltower

var controlTowerAccessRoles = map[string]struct{}{
	"PLATFORM_ADMIN":     {},
	"CARRIER_DISPATCHER": {},
	"SHIPPER_ADMIN":      {},
	"SHIPPER_LOGIST":     {},
	"FORWARDER_MANAGER":  {},
}

func CanAccessControlTower(roles []string) bool {
	for _, role := range roles {
		if _, ok := controlTowerAccessRoles[role]; ok {
			return true
		}
	}
	return false
}
