package domain

import (
	"testing"

	"github.com/google/uuid"
)

func TestNormalizeBatchEntityIDsDeduplicatesPreservingOrder(t *testing.T) {
	first := uuid.MustParse("11111111-1111-4111-8111-111111111111")
	second := uuid.MustParse("22222222-2222-4222-8222-222222222222")

	normalized := NormalizeBatchEntityIDs([]uuid.UUID{first, second, first, second, first})
	if len(normalized) != 2 {
		t.Fatalf("expected 2 unique ids, got %d", len(normalized))
	}
	if normalized[0] != first || normalized[1] != second {
		t.Fatalf("unexpected order: %#v", normalized)
	}
}

func TestNormalizeBatchEntityIDsEmptySlice(t *testing.T) {
	if got := NormalizeBatchEntityIDs(nil); got != nil {
		t.Fatalf("expected nil, got %#v", got)
	}
}
