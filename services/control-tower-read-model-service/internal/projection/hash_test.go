package projection

import (
	"testing"
)

func TestPayloadSHA256Stable(t *testing.T) {
	t.Parallel()
	payload := []byte(`{"eventId":"11111111-1111-1111-1111-111111111111","eventType":"shipment.created"}`)
	first := PayloadSHA256(payload)
	second := PayloadSHA256(payload)
	if first != second {
		t.Fatalf("hash not stable: %s vs %s", first, second)
	}
	if len(first) != 64 {
		t.Fatalf("expected sha256 hex length 64, got %d", len(first))
	}
}

func TestPayloadSHA256DiffersForDifferentPayloads(t *testing.T) {
	t.Parallel()
	a := PayloadSHA256([]byte(`{"a":1}`))
	b := PayloadSHA256([]byte(`{"a":2}`))
	if a == b {
		t.Fatal("different payloads must produce different hashes")
	}
}
