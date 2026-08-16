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

func CanViewCase(roles []string) bool {
	return CanAccessControlTower(roles)
}

func CanCreateCase(roles []string) bool {
	return CanAccessControlTower(roles)
}

func CanManageCase(roles []string) bool {
	return CanAccessControlTower(roles)
}

func CanAssignCase(roles []string) bool {
	return CanAccessControlTower(roles)
}

func CanAddCaseNote(roles []string) bool {
	return CanAccessControlTower(roles)
}

func CanManageCaseActions(roles []string) bool {
	return CanAccessControlTower(roles)
}

func CanResolveCase(roles []string) bool {
	return CanAccessControlTower(roles)
}

func CanManageCaseParticipants(roles []string) bool {
	return CanAccessControlTower(roles)
}

func CanViewAutomation(roles []string) bool {
	return CanAccessControlTower(roles)
}

func CanManageAutomationRules(roles []string) bool {
	return hasAnyRole(roles, map[string]struct{}{"PLATFORM_ADMIN": {}})
}

func CanManagePlaybooks(roles []string) bool {
	return hasAnyRole(roles, map[string]struct{}{"PLATFORM_ADMIN": {}})
}

func CanViewRecommendations(roles []string) bool {
	return CanAccessControlTower(roles)
}

func CanStartPlaybook(roles []string) bool {
	return CanAccessControlTower(roles)
}

func CanManagePlaybookExecution(roles []string) bool {
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
