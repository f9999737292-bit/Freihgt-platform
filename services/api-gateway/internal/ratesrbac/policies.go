package ratesrbac

import (
	"strings"

	"github.com/freight-platform/api-gateway/internal/routeauth"
)

const rolePlatformAdmin = "PLATFORM_ADMIN"

type Policy int

const (
	PolicyRead Policy = iota
	PolicyCreateContract
	PolicyEditContract
	PolicyContractLifecycle
	PolicyCreateRateCard
	PolicyEditDraftRateVersion
	PolicyActivateRateVersion
	PolicyEditDraftRate
	PolicySimulate
)

var readRoles = map[string]struct{}{
	rolePlatformAdmin:    {},
	"PROCUREMENT_MANAGER": {},
	"SHIPPER_ADMIN":       {},
	"SHIPPER_LOGIST":      {},
	"FORWARDER_MANAGER":   {},
	"CARRIER_ADMIN":       {},
	"CARRIER_DISPATCHER":  {},
	"CARRIER_ACCOUNTANT":  {},
}

var mutateRoles = map[string]struct{}{
	rolePlatformAdmin:    {},
	"PROCUREMENT_MANAGER": {},
	"SHIPPER_ADMIN":       {},
	"FORWARDER_MANAGER":   {},
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

func allowMutate(companyRoles []string, actorKind string, isPlatformAdmin bool) bool {
	if actorKind != "BUYER" {
		return false
	}
	if isPlatformAdmin {
		return true
	}
	return routeauth.HasAnyRole(companyRoles, mutateRoles)
}

func policyAllows(policy Policy, companyRoles []string, actorKind string, isPlatformAdmin bool) bool {
	switch policy {
	case PolicyRead, PolicySimulate:
		return allowRead(companyRoles, isPlatformAdmin)
	case PolicyCreateContract, PolicyEditContract, PolicyContractLifecycle,
		PolicyCreateRateCard, PolicyEditDraftRateVersion, PolicyActivateRateVersion, PolicyEditDraftRate:
		return allowMutate(companyRoles, actorKind, isPlatformAdmin)
	default:
		return false
	}
}

func policyDenyMessage(policy Policy) string {
	switch policy {
	case PolicyRead:
		return "contract rate read access denied"
	case PolicySimulate:
		return "contract rate simulation access denied"
	case PolicyCreateContract:
		return "contract create access denied"
	case PolicyEditContract:
		return "contract edit access denied"
	case PolicyContractLifecycle:
		return "contract lifecycle access denied"
	case PolicyCreateRateCard:
		return "rate card create access denied"
	case PolicyEditDraftRateVersion:
		return "rate version edit access denied"
	case PolicyActivateRateVersion:
		return "rate version activation access denied"
	case PolicyEditDraftRate:
		return "rate line/component edit access denied"
	default:
		return "contract rate access denied"
	}
}

func policyMetricName(policy Policy) string {
	switch policy {
	case PolicyRead:
		return "read"
	case PolicySimulate:
		return "simulate"
	case PolicyCreateContract:
		return "create_contract"
	case PolicyEditContract:
		return "edit_contract"
	case PolicyContractLifecycle:
		return "contract_lifecycle"
	case PolicyCreateRateCard:
		return "create_rate_card"
	case PolicyEditDraftRateVersion:
		return "edit_rate_version"
	case PolicyActivateRateVersion:
		return "activate_rate_version"
	case PolicyEditDraftRate:
		return "edit_rate"
	default:
		return "unknown"
	}
}
