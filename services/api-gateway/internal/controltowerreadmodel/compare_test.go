package controltowerreadmodel

import "testing"

func TestCompareLegacyLimitedDataset(t *testing.T) {
	legacy := legacyInput(1200, 100, map[string]int64{"IN_TRANSIT": 100}, true)
	rm := readModelPayload(1200, map[string]int64{"IN_TRANSIT": 1200}, 0, true)
	if CompareStatusSummaries(legacy, rm) != ComparisonLegacyLimitedDataset {
		t.Fatal("expected LEGACY_LIMITED_DATASET")
	}
}

func TestCompareExactMatch(t *testing.T) {
	legacy := legacyInput(2, 2, map[string]int64{"IN_TRANSIT": 1, "DELIVERED": 1}, false)
	rm := readModelPayload(2, map[string]int64{"IN_TRANSIT": 1, "DELIVERED": 1}, 0, true)
	if CompareStatusSummaries(legacy, rm) != ComparisonMatch {
		t.Fatal("expected MATCH")
	}
}

func TestCompareTotalMismatch(t *testing.T) {
	legacy := legacyInput(2, 2, map[string]int64{"IN_TRANSIT": 2}, false)
	rm := readModelPayload(3, map[string]int64{"IN_TRANSIT": 3}, 0, true)
	if CompareStatusSummaries(legacy, rm) != ComparisonTotalMismatch {
		t.Fatal("expected TOTAL_MISMATCH")
	}
}

func TestCompareStatusCountMismatch(t *testing.T) {
	legacy := legacyInput(2, 2, map[string]int64{"IN_TRANSIT": 1, "DELIVERED": 1}, false)
	rm := readModelPayload(2, map[string]int64{"IN_TRANSIT": 2, "DELIVERED": 0}, 0, true)
	if CompareStatusSummaries(legacy, rm) != ComparisonStatusCountMismatch {
		t.Fatal("expected STATUS_COUNT_MISMATCH")
	}
}

func TestCompareSeveralStatusMismatches(t *testing.T) {
	legacy := legacyInput(3, 3, map[string]int64{"IN_TRANSIT": 1, "LOADED": 1, "DELIVERED": 1}, false)
	rm := readModelPayload(3, map[string]int64{"IN_TRANSIT": 2, "LOADED": 0, "DELIVERED": 1}, 0, true)
	if CompareStatusSummaries(legacy, rm) != ComparisonStatusCountMismatch {
		t.Fatal("expected STATUS_COUNT_MISMATCH for multiple deltas")
	}
}

func TestCompareStatusOnlyInLegacy(t *testing.T) {
	legacy := legacyInput(2, 2, map[string]int64{"IN_TRANSIT": 1, "LOADED": 1}, false)
	rm := readModelPayload(2, map[string]int64{"IN_TRANSIT": 2}, 0, true)
	if CompareStatusSummaries(legacy, rm) != ComparisonStatusCountMismatch {
		t.Fatal("expected STATUS_COUNT_MISMATCH when legacy has extra status")
	}
}

func TestCompareStatusOnlyInReadModel(t *testing.T) {
	legacy := legacyInput(2, 2, map[string]int64{"IN_TRANSIT": 2}, false)
	rm := readModelPayload(2, map[string]int64{"IN_TRANSIT": 1, "LOADED": 1}, 0, true)
	if CompareStatusSummaries(legacy, rm) != ComparisonStatusCountMismatch {
		t.Fatal("expected STATUS_COUNT_MISMATCH when read-model has extra status")
	}
}

func TestCompareLegacyUnavailable(t *testing.T) {
	legacy := LegacyStatusInput{TotalShipments: -1, ByStatus: nil}
	rm := readModelPayload(1, map[string]int64{"IN_TRANSIT": 1}, 0, true)
	if CompareStatusSummaries(legacy, rm) != ComparisonLegacyUnavailable {
		t.Fatal("expected LEGACY_UNAVAILABLE")
	}
}

func TestCompareReadModelUnavailable(t *testing.T) {
	legacy := legacyInput(1, 1, map[string]int64{"IN_TRANSIT": 1}, false)
	if CompareStatusSummaries(legacy, nil) != ComparisonReadModelUnavailable {
		t.Fatal("expected READ_MODEL_UNAVAILABLE")
	}
}
