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

func CanViewRisk(roles []string) bool {
	return CanAccessControlTower(roles)
}

func CanAckRisk(roles []string) bool {
	return CanAccessControlTower(roles)
}

func CanMitigateRisk(roles []string) bool {
	return CanAccessControlTower(roles)
}

func CanViewWorkspace(roles []string) bool {
	return CanAccessControlTower(roles)
}

func CanClaimWork(roles []string) bool {
	return CanAccessControlTower(roles)
}

func CanAssignWork(roles []string) bool {
	return CanAccessControlTower(roles)
}

func CanBulkManageWork(roles []string) bool {
	return CanAccessControlTower(roles)
}

func CanViewTeamWorkload(roles []string) bool {
	return CanAccessControlTower(roles)
}

func CanManageSharedViews(roles []string) bool {
	return CanAccessControlTower(roles)
}

func CanCreateHandoff(roles []string) bool {
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
