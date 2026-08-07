package statussnapshot

import "sort"

// ProtocolV1ShipmentStatuses returns the sorted allowlist used by snapshot protocol v1.
func ProtocolV1ShipmentStatuses() []string {
	statuses := make([]string, 0, len(KnownShipmentStatuses))
	for status := range KnownShipmentStatuses {
		statuses = append(statuses, status)
	}
	sort.Strings(statuses)
	return statuses
}
