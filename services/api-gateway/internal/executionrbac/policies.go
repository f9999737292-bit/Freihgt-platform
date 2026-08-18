package executionrbac

import "github.com/freight-platform/api-gateway/internal/routeauth"

var executeOrderRoles = map[string]struct{}{
	"PLATFORM_ADMIN":     {},
	"CARRIER_ADMIN":      {},
	"CARRIER_DISPATCHER": {},
}

var readExecutionRoles = map[string]struct{}{
	"PLATFORM_ADMIN":     {},
	"SHIPPER_ADMIN":      {},
	"SHIPPER_LOGIST":     {},
	"FORWARDER_MANAGER":  {},
	"CARRIER_ADMIN":      {},
	"CARRIER_DISPATCHER": {},
}

var startExecutionRoles = map[string]struct{}{
	"PLATFORM_ADMIN":     {},
	"CARRIER_ADMIN":      {},
	"CARRIER_DISPATCHER": {},
}

func CanExecuteOrder(roles []string) bool {
	return routeauth.HasAnyRole(roles, executeOrderRoles)
}

func CanReadExecution(roles []string) bool {
	return routeauth.HasAnyRole(roles, readExecutionRoles)
}

func CanStartExecution(roles []string) bool {
	return routeauth.HasAnyRole(roles, startExecutionRoles)
}
