package domain

import (
	"sort"
	"testing"

	"github.com/freight-platform/statussnapshot"
)

func TestProtocolV1StatusesMatchReadModel(t *testing.T) {
	protocol := statussnapshot.ProtocolV1ShipmentStatuses()
	readModel := make([]string, 0, len(allowedShipmentStatuses))
	for status := range allowedShipmentStatuses {
		readModel = append(readModel, status)
	}
	sort.Strings(readModel)
	if len(readModel) != len(protocol) {
		t.Fatalf("read-model statuses=%d protocol=%d", len(readModel), len(protocol))
	}
	for i := range protocol {
		if protocol[i] != readModel[i] {
			t.Fatalf("status mismatch at %d: protocol=%q read-model=%q", i, protocol[i], readModel[i])
		}
	}
}
