package domain

import "strings"

// ParseLaneKeyComponents extracts display components from a canonical lane_key.
func ParseLaneKeyComponents(laneKey string) (originCountry, originCity, destCountry, destCity, mode, equipment string, ok bool) {
	laneKey = strings.TrimSpace(laneKey)
	parts := strings.Split(laneKey, "|")
	if len(parts) != 3 {
		return "", "", "", "", "", "", false
	}
	mode = parts[1]
	equipment = parts[2]
	routeParts := strings.Split(parts[0], "->")
	if len(routeParts) != 2 {
		return "", "", "", "", "", "", false
	}
	originParts := strings.SplitN(routeParts[0], ":", 2)
	destParts := strings.SplitN(routeParts[1], ":", 2)
	if len(originParts) != 2 || len(destParts) != 2 {
		return "", "", "", "", "", "", false
	}
	return originParts[0], originParts[1], destParts[0], destParts[1], mode, equipment, true
}
