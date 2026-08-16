package domain

import (
	"strings"

	"github.com/google/uuid"

	apperrors "github.com/freight-platform/rfx-service/internal/platform/errors"
)

// ResolveBuyerCompanyID validates requested owner/shipper company against verified memberships.
// The request value is a selector only; membership is the authority.
func ResolveBuyerCompanyID(requested uuid.UUID, memberships []uuid.UUID) (uuid.UUID, error) {
	if len(memberships) == 0 {
		return uuid.Nil, apperrors.Forbidden("buyer company membership is required")
	}
	if requested != uuid.Nil {
		for _, id := range memberships {
			if id == requested {
				return id, nil
			}
		}
		return uuid.Nil, apperrors.Forbidden("owner_company_id does not match authenticated membership")
	}
	if len(memberships) == 1 {
		return memberships[0], nil
	}
	return uuid.Nil, apperrors.Validation("owner_company_id is required when user belongs to multiple buyer companies", map[string]any{"field": "owner_company_id"})
}

func ContainsCompanyID(memberships []uuid.UUID, companyID uuid.UUID) bool {
	for _, id := range memberships {
		if id == companyID {
			return true
		}
	}
	return false
}

func HasBuyerRole(roleCodes []string) bool {
	for _, code := range roleCodes {
		switch strings.ToUpper(strings.TrimSpace(code)) {
		case "PLATFORM_ADMIN", "PROCUREMENT_MANAGER", "SHIPPER_ADMIN", "SHIPPER_LOGIST", "FORWARDER_MANAGER":
			return true
		}
	}
	return false
}
