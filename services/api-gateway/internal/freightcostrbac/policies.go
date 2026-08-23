package freightcostrbac

import (
	"strings"

	"github.com/freight-platform/api-gateway/internal/routeauth"
)

const rolePlatformAdmin = "PLATFORM_ADMIN"

type Policy int

const (
	PolicyRead Policy = iota
	PolicyBuyerAnalytics
)

var buyerAnalyticsRoles = map[string]struct{}{
	rolePlatformAdmin:     {},
	"PROCUREMENT_MANAGER": {},
	"SHIPPER_ADMIN":       {},
	"SHIPPER_LOGIST":      {},
	"FORWARDER_MANAGER":   {},
	"FINANCE_MANAGER":     {},
}

var readRoles = map[string]struct{}{
	rolePlatformAdmin:     {},
	"PROCUREMENT_MANAGER": {},
	"SHIPPER_ADMIN":       {},
	"SHIPPER_LOGIST":      {},
	"FORWARDER_MANAGER":   {},
	"FINANCE_MANAGER":     {},
	"CARRIER_ADMIN":       {},
	"CARRIER_DISPATCHER":  {},
	"CARRIER_ACCOUNTANT":  {},
}

func hasPlatformAdmin(roles []string) bool {
	for _, role := range roles {
		if strings.EqualFold(strings.TrimSpace(role), rolePlatformAdmin) {
			return true
		}
	}
	return false
}

func allowRead(companyRoles []string, isPlatformAdmin bool) bool {
	if isPlatformAdmin {
		return true
	}
	return routeauth.HasAnyRole(companyRoles, readRoles)
}

func allowBuyerAnalytics(companyRoles []string, isPlatformAdmin bool) bool {
	if isPlatformAdmin {
		return true
	}
	return routeauth.HasAnyRole(companyRoles, buyerAnalyticsRoles)
}

func policyAllows(policy Policy, companyRoles []string, _ string, isPlatformAdmin bool) bool {
	switch policy {
	case PolicyRead:
		return allowRead(companyRoles, isPlatformAdmin)
	case PolicyBuyerAnalytics:
		return allowBuyerAnalytics(companyRoles, isPlatformAdmin)
	default:
		return false
	}
}

func policyDenyMessage(policy Policy) string {
	switch policy {
	case PolicyRead:
		return "freight cost read access denied"
	case PolicyBuyerAnalytics:
		return "freight cost buyer analytics access denied"
	default:
		return "freight cost access denied"
	}
}

func policyMetricName(policy Policy) string {
	switch policy {
	case PolicyRead:
		return "read"
	case PolicyBuyerAnalytics:
		return "buyer_analytics"
	default:
		return "unknown"
	}
}
