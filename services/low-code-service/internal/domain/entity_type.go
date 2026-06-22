package domain

import (
	"strings"

	apperrors "github.com/freight-platform/low-code-service/internal/platform/errors"
)

var allowedEntityTypes = map[string]struct{}{
	"TRANSPORT_ORDER":  {},
	"RFX":              {},
	"FREIGHT_REQUEST":  {},
	"BID":              {},
	"SHIPMENT":         {},
	"DOCUMENT":         {},
	"BILLING_REGISTER": {},
	"COMPANY_PROFILE":  {},
	"DRIVER_TASK":      {},
}

func ValidateEntityType(entityType string) error {
	entityType = strings.TrimSpace(entityType)
	if entityType == "" {
		return nil
	}
	if _, ok := allowedEntityTypes[entityType]; !ok {
		return apperrors.Validation("invalid entity_type", map[string]any{
			"entity_type": entityType,
			"allowed":     AllowedEntityTypes(),
		})
	}
	return nil
}

func AllowedEntityTypes() []string {
	return []string{
		"TRANSPORT_ORDER",
		"RFX",
		"FREIGHT_REQUEST",
		"BID",
		"SHIPMENT",
		"DOCUMENT",
		"BILLING_REGISTER",
		"COMPANY_PROFILE",
		"DRIVER_TASK",
	}
}
