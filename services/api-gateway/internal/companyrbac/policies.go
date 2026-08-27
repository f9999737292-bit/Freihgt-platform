package companyrbac

import (
	"strings"

	"github.com/freight-platform/api-gateway/internal/routeauth"
)

const rolePlatformAdmin = "PLATFORM_ADMIN"

type Policy int

const (
	PolicyCreate Policy = iota
	PolicyList
	PolicyRead
	PolicyUpdate
	PolicyDelete
	PolicyReadMembers
	PolicyManageMembers
)

var companyAdminRoles = map[string]struct{}{
	rolePlatformAdmin:   {},
	"SHIPPER_ADMIN":     {},
	"CARRIER_ADMIN":     {},
	"FORWARDER_MANAGER": {},
}

func hasPlatformAdmin(roles []string) bool {
	for _, role := range roles {
		if strings.EqualFold(strings.TrimSpace(role), rolePlatformAdmin) {
			return true
		}
	}
	return false
}

func hasCompanyAdmin(roles []string) bool {
	return routeauth.HasAnyRole(roles, companyAdminRoles)
}

func policyRequiresCompanyID(policy Policy) bool {
	switch policy {
	case PolicyRead, PolicyUpdate, PolicyDelete, PolicyReadMembers, PolicyManageMembers:
		return true
	default:
		return false
	}
}

func policyAllows(policy Policy, companyRoles []string, isPlatformAdmin bool, hasMembership bool) bool {
	switch policy {
	case PolicyCreate, PolicyDelete:
		return isPlatformAdmin
	case PolicyList:
		return true
	case PolicyRead, PolicyReadMembers:
		return isPlatformAdmin || hasMembership
	case PolicyUpdate, PolicyManageMembers:
		if isPlatformAdmin {
			return true
		}
		return hasMembership && hasCompanyAdmin(companyRoles)
	default:
		return false
	}
}

func policyDenyMessage(policy Policy) string {
	switch policy {
	case PolicyCreate:
		return "company creation access denied"
	case PolicyList:
		return "company list access denied"
	case PolicyRead, PolicyReadMembers:
		return "company read access denied"
	case PolicyUpdate, PolicyManageMembers:
		return "company mutation access denied"
	case PolicyDelete:
		return "company deletion access denied"
	default:
		return "company access denied"
	}
}

func policyMetricName(policy Policy) string {
	switch policy {
	case PolicyCreate:
		return "create"
	case PolicyList:
		return "list"
	case PolicyRead:
		return "read"
	case PolicyUpdate:
		return "update"
	case PolicyDelete:
		return "delete"
	case PolicyReadMembers:
		return "read_members"
	case PolicyManageMembers:
		return "manage_members"
	default:
		return "unknown"
	}
}
