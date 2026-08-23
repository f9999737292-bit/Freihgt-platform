package domain

import (
	"fmt"
	"strings"
	"unicode"
)

const (
	EquipmentTypeWild = "WILD"

	LaneExclusionNone                  = ""
	LaneExclusionMissingOriginCity     = "MISSING_ORIGIN_CITY"
	LaneExclusionMissingDestinationCity = "MISSING_DESTINATION_CITY"
	LaneExclusionMissingOriginCountry  = "MISSING_ORIGIN_COUNTRY"
	LaneExclusionMissingDestCountry    = "MISSING_DESTINATION_COUNTRY"
	LaneExclusionMissingTransportMode  = "MISSING_TRANSPORT_MODE"
)

type LaneKeyInput struct {
	OriginCountry      string
	OriginCity         string
	DestinationCountry string
	DestinationCity    string
	TransportMode      string
	EquipmentType      string
}

type LaneKeyResult struct {
	LaneKey         string
	Available       bool
	ExclusionReason string
	OriginCountry   string
	OriginCity      string
	DestinationCountry string
	DestinationCity string
	TransportMode   string
	EquipmentType   string
}

// BuildLaneKey derives the canonical directional lane key per ADR-22-004 / ANALYTICS_CONTRACT.
func BuildLaneKey(in LaneKeyInput) LaneKeyResult {
	originCountry := normalizeCountry(in.OriginCountry)
	destCountry := normalizeCountry(in.DestinationCountry)
	originCity := normalizeCity(in.OriginCity)
	destCity := normalizeCity(in.DestinationCity)
	mode := normalizeMode(in.TransportMode)
	equipment := normalizeEquipment(in.EquipmentType)

	result := LaneKeyResult{
		OriginCountry:      originCountry,
		OriginCity:         originCity,
		DestinationCountry: destCountry,
		DestinationCity:    destCity,
		TransportMode:      mode,
		EquipmentType:      equipment,
	}

	switch {
	case originCountry == "":
		result.ExclusionReason = LaneExclusionMissingOriginCountry
		return result
	case destCountry == "":
		result.ExclusionReason = LaneExclusionMissingDestCountry
		return result
	case originCity == "":
		result.ExclusionReason = LaneExclusionMissingOriginCity
		return result
	case destCity == "":
		result.ExclusionReason = LaneExclusionMissingDestinationCity
		return result
	case mode == "":
		result.ExclusionReason = LaneExclusionMissingTransportMode
		return result
	}

	result.LaneKey = fmt.Sprintf("%s:%s->%s:%s|%s|%s",
		originCountry, originCity, destCountry, destCity, mode, equipment)
	result.Available = true
	return result
}

func normalizeCountry(value string) string {
	value = strings.ToUpper(strings.TrimSpace(value))
	if len(value) != 2 {
		return ""
	}
	for _, r := range value {
		if r < 'A' || r > 'Z' {
			return ""
		}
	}
	return value
}

func normalizeCity(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}
	var b strings.Builder
	for _, r := range value {
		if unicode.IsSpace(r) {
			continue
		}
		b.WriteRune(unicode.ToUpper(r))
	}
	return b.String()
}

func normalizeMode(value string) string {
	return strings.ToUpper(strings.TrimSpace(value))
}

func normalizeEquipment(value string) string {
	value = strings.ToUpper(strings.TrimSpace(value))
	if value == "" {
		return EquipmentTypeWild
	}
	return value
}
