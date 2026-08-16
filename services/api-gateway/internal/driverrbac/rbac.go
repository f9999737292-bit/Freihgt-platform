package driverrbac

import "github.com/freight-platform/api-gateway/internal/routeauth"

var driverRoles = map[string]struct{}{
	"DRIVER": {},
}

func CanAccessDriverRoutes(roles []string) bool {
	return routeauth.HasAnyRole(roles, driverRoles)
}
