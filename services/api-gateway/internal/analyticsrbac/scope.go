package analyticsrbac

import (
	"errors"
	"strings"

	"github.com/google/uuid"
)

const (
	ScopeTenant  = "TENANT"
	ScopeShipper = "SHIPPER"
	ScopeCarrier = "CARRIER"
)

var ErrScopeUnavailable = errors.New("analytics company scope is not available")

type Scope struct {
	Kind      string
	CompanyID string
}

type Membership struct {
	CompanyID   string
	CompanyType string
	RoleCodes   []string
}

func HasPlatformAdmin(tenantRoles []string) bool {
	for _, role := range tenantRoles {
		if strings.EqualFold(strings.TrimSpace(role), "PLATFORM_ADMIN") {
			return true
		}
	}
	return false
}

func Resolve(tenantRoles []string, memberships []Membership) (Scope, error) {
	if HasPlatformAdmin(tenantRoles) {
		return Scope{Kind: ScopeTenant}, nil
	}
	var chosen *Scope
	seen := make(map[string]string)
	for _, membership := range memberships {
		parsed, err := uuid.Parse(strings.TrimSpace(membership.CompanyID))
		if err != nil || parsed == uuid.Nil {
			continue
		}
		id := parsed.String()
		kind, ok := qualify(membership.CompanyType, membership.RoleCodes)
		if !ok {
			continue
		}
		if previous, exists := seen[id]; exists {
			if previous != kind {
				return Scope{}, ErrScopeUnavailable
			}
			continue
		}
		seen[id] = kind
		if chosen != nil {
			return Scope{}, ErrScopeUnavailable
		}
		chosen = &Scope{Kind: kind, CompanyID: id}
	}
	if chosen == nil {
		return Scope{}, ErrScopeUnavailable
	}
	return *chosen, nil
}

func qualify(companyType string, roles []string) (string, bool) {
	switch strings.ToUpper(strings.TrimSpace(companyType)) {
	case "SHIPPER":
		if hasRole(roles, "SHIPPER_ADMIN", "SHIPPER_LOGIST") {
			return ScopeShipper, true
		}
	case "CARRIER":
		if hasRole(roles, "CARRIER_ADMIN", "CARRIER_DISPATCHER", "CARRIER_ACCOUNTANT") {
			return ScopeCarrier, true
		}
	}
	return "", false
}

func hasRole(roles []string, allowed ...string) bool {
	for _, role := range roles {
		normalized := strings.ToUpper(strings.TrimSpace(role))
		for _, want := range allowed {
			if normalized == want {
				return true
			}
		}
	}
	return false
}
