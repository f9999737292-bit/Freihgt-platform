package auth

// Low-code admin API roles (v0.1). Extend with LOW_CODE_ADMIN / LOW_CODE_PUBLISHER when seeded.
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
