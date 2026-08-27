package domain

// Company authorization contract (Wave 1R1).
//
// Route matrix — public /v1/companies handlers:
//
//	POST   /companies              AUTH_REQUIRED  TENANT_FROM_JWT  ROLE=PLATFORM_ADMIN  DENY=403
//	GET    /companies              AUTH_REQUIRED  TENANT_FROM_JWT  MEMBERSHIP_SCOPED|PLATFORM_ADMIN  DENY=403/empty
//	GET    /companies/{id}         AUTH_REQUIRED  TENANT_MATCH     MEMBERSHIP|PLATFORM_ADMIN       DENY=404
//	PATCH  /companies/{id}         AUTH_REQUIRED  TENANT_MATCH     COMPANY_ADMIN|PLATFORM_ADMIN    DENY=403
//	DELETE /companies/{id}         AUTH_REQUIRED  TENANT_MATCH     PLATFORM_ADMIN                  DENY=403
//	GET    /companies/{id}/members AUTH_REQUIRED  TENANT_MATCH     MEMBERSHIP|PLATFORM_ADMIN       DENY=403
//	POST   /companies/{id}/members AUTH_REQUIRED  TENANT_MATCH     COMPANY_ADMIN|PLATFORM_ADMIN    DENY=403
//	PATCH  /companies/{id}/members/{mid}  same as POST members
//	DELETE /companies/{id}/members/{mid}  same as POST members
//
// Client-supplied tenant_id (query/body) is never authorization proof.

const RolePlatformAdmin = "PLATFORM_ADMIN"

var CompanyAdminRoles = map[string]struct{}{
	RolePlatformAdmin:   {},
	"SHIPPER_ADMIN":     {},
	"CARRIER_ADMIN":     {},
	"FORWARDER_MANAGER": {},
}

func HasCompanyAdminRole(codes []string) bool {
	for _, code := range codes {
		if _, ok := CompanyAdminRoles[code]; ok {
			return true
		}
	}
	return false
}

func HasPlatformAdminRole(codes []string) bool {
	for _, code := range codes {
		if code == RolePlatformAdmin {
			return true
		}
	}
	return false
}
