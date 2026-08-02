package domain

import (
	"sort"
	"testing"

	"github.com/freight-platform/statussnapshot"
)

func TestProtocolV1StatusesMatchDomain(t *testing.T) {
	protocol := statussnapshot.ProtocolV1ShipmentStatuses()
	for _, status := range protocol {
		if !IsValidShipmentStatus(status) {
			t.Fatalf("protocol status %q missing from shipment-service domain", status)
		}
	}
	domainStatuses := make([]string, 0, len(knownShipmentStatuses))
	for status := range knownShipmentStatuses {
		if status == ShipmentStatusCreated {
			continue
		}
		domainStatuses = append(domainStatuses, status)
	}
	sort.Strings(domainStatuses)
	if len(domainStatuses) != len(protocol) {
		t.Fatalf("domain exportable statuses=%d protocol=%d", len(domainStatuses), len(protocol))
	}
	for i := range protocol {
		if protocol[i] != domainStatuses[i] {
			t.Fatalf("status mismatch at %d: protocol=%q domain=%q", i, protocol[i], domainStatuses[i])
		}
	}
}
