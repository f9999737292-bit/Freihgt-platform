package auth

// LowCodeAdminRoleCodes lists roles allowed for /v1/low-code/admin/* when
// LOW_CODE_ADMIN_AUTH_ENABLED=true. See docs/LOW_CODE_PERMISSIONS_MATRIX_V0.1.md.
var LowCodeAdminRoleCodes = []string{
	"PLATFORM_ADMIN",
}

func HasLowCodeAdminRole(roleCodes []string) bool {
	if len(roleCodes) == 0 {
		return false
	}
	allowed := make(map[string]struct{}, len(LowCodeAdminRoleCodes))
	for _, code := range LowCodeAdminRoleCodes {
		allowed[code] = struct{}{}
	}
	for _, code := range roleCodes {
		if _, ok := allowed[code]; ok {
			return true
		}
	}
	return false
}
