package analyticsrbac

import "github.com/freight-platform/api-gateway/internal/routeauth"

var readRoles = map[string]struct{}{
	"PLATFORM_ADMIN":      {},
	"PROCUREMENT_MANAGER": {},
	"SHIPPER_ADMIN":       {},
	"SHIPPER_LOGIST":      {},
	"FORWARDER_MANAGER":   {},
	"FINANCE_MANAGER":     {},
	"CARRIER_ADMIN":       {},
	"CARRIER_DISPATCHER":  {},
	"CARRIER_ACCOUNTANT":  {},
}

func AllowRead(roles []string) bool {
	return routeauth.HasAnyRole(roles, readRoles)
}
