package rfxrbac

import "github.com/freight-platform/api-gateway/internal/routeauth"

var evaluateRoles = map[string]struct{}{
	"PLATFORM_ADMIN":      {},
	"SHIPPER_ADMIN":       {},
	"PROCUREMENT_MANAGER": {},
}

var approveAwardRoles = map[string]struct{}{
	"PLATFORM_ADMIN":      {},
	"SHIPPER_ADMIN":       {},
	"PROCUREMENT_MANAGER": {},
}

var finalizeAwardRoles = map[string]struct{}{
	"PLATFORM_ADMIN":      {},
	"SHIPPER_ADMIN":       {},
	"PROCUREMENT_MANAGER": {},
}

func CanEvaluate(roles []string) bool {
	return routeauth.HasAnyRole(roles, evaluateRoles)
}

func CanApproveAward(roles []string) bool {
	return routeauth.HasAnyRole(roles, approveAwardRoles)
}

func CanFinalizeAward(roles []string) bool {
	return routeauth.HasAnyRole(roles, finalizeAwardRoles)
}
