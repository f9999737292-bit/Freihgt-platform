package domain

import (
	"fmt"
	"strings"
)

// BuildLaneLabel derives a canonical non-localized display label from lane components.
// Lane identity remains lane_key; this is display-only.
func BuildLaneLabel(originCity, destinationCity, transportMode, equipmentType string) string {
	origin := normalizeCity(originCity)
	dest := normalizeCity(destinationCity)
	mode := normalizeMode(transportMode)
	equipment := normalizeEquipment(equipmentType)
	if origin == "" || dest == "" || mode == "" {
		return ""
	}
	return fmt.Sprintf("%s → %s (%s / %s)", origin, dest, mode, equipment)
}

// ResolveCompanyDisplayName prefers short_name over legal_name per v2.2D enrichment policy.
func ResolveCompanyDisplayName(shortName, legalName *string) string {
	if shortName != nil {
		if trimmed := stringsTrim(*shortName); trimmed != "" {
			return trimmed
		}
	}
	if legalName != nil {
		return stringsTrim(*legalName)
	}
	return ""
}

func stringsTrim(value string) string {
	return strings.TrimSpace(value)
}
