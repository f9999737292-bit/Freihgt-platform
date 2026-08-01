package controltowerreadmodel

import "testing"

func legacyInput(total int64, counted int64, byStatus map[string]int64, limited bool) LegacyStatusInput {
	return LegacyStatusInput{
		TotalShipments:         total,
		CountedShipments:       counted,
		ByStatus:               byStatus,
		LimitedDataset:         limited,
		FullAggregateAvailable: !limited,
	}
}

func readModelPayload(total int64, byStatus map[string]int64, incomplete int64, consumerRunning bool) *RemoteStatusSummary {
	return &RemoteStatusSummary{
		TotalShipments:        total,
		ByStatus:              byStatus,
		IncompleteProjections: incomplete,
		Freshness:             FreshnessSnapshot{ConsumerRunning: consumerRunning},
	}
}

func TestMergeDisabledLimitedLegacyReturnsSummary(t *testing.T) {
	out := Merge(MergeInput{
		Mode:   ModeDisabled,
		Legacy: legacyInput(1200, 100, map[string]int64{"IN_TRANSIT": 20, "DELIVERED": 80}, true),
	})
	if out.StatusSummary == nil {
		t.Fatal("expected limited legacy summary in disabled mode")
	}
	if !out.StatusSummary.LimitedDataset || out.StatusSummary.CountedShipments != 100 {
		t.Fatalf("summary=%+v", out.StatusSummary)
	}
	if !containsWarning(out.Warnings, WarningLegacyLimited) {
		t.Fatalf("warnings=%v", out.Warnings)
	}
}

func TestMergeDisabledFullLegacyReturnsSummary(t *testing.T) {
	out := Merge(MergeInput{
		Mode:   ModeDisabled,
		Legacy: legacyInput(10, 10, map[string]int64{"IN_TRANSIT": 10}, false),
	})
	if out.StatusSummary == nil {
		t.Fatal("expected full legacy summary in disabled mode")
	}
	if out.StatusSummary.LimitedDataset || out.StatusSummary.CountedShipments != 10 {
		t.Fatalf("summary=%+v", out.StatusSummary)
	}
	if out.StatusSummaryFreshness == nil || out.StatusSummaryFreshness.LegacyAggregateLoaded == nil || !*out.StatusSummaryFreshness.LegacyAggregateLoaded {
		t.Fatalf("freshness=%+v", out.StatusSummaryFreshness)
	}
}

func TestMergeShadowKeepsLegacyInOutput(t *testing.T) {
	out := Merge(MergeInput{
		Mode:      ModeShadow,
		Legacy:    legacyInput(2, 2, map[string]int64{"IN_TRANSIT": 2}, false),
		ReadModel: readModelPayload(2, map[string]int64{"IN_TRANSIT": 2}, 0, true),
	})
	if out.StatusSummary == nil || out.StatusSummary.Source != SourceLegacy {
		t.Fatalf("shadow mode must expose legacy status summary, got %+v", out.StatusSummary)
	}
	if out.Comparison != ComparisonMatch {
		t.Fatalf("comparison=%q want MATCH", out.Comparison)
	}
}

func TestMergeShadowLimitedLegacyComparison(t *testing.T) {
	out := Merge(MergeInput{
		Mode:      ModeShadow,
		Legacy:    legacyInput(1200, 100, map[string]int64{"IN_TRANSIT": 100}, true),
		ReadModel: readModelPayload(1200, map[string]int64{"IN_TRANSIT": 1200}, 0, true),
	})
	if out.Comparison != ComparisonLegacyLimitedDataset {
		t.Fatalf("comparison=%q want LEGACY_LIMITED_DATASET", out.Comparison)
	}
}

func TestMergePrimaryUsesReadModel(t *testing.T) {
	out := Merge(MergeInput{
		Mode:                   ModePrimary,
		Legacy:                 legacyInput(1200, 100, map[string]int64{"DELIVERED": 100}, true),
		ReadModel:              readModelPayload(2, map[string]int64{"IN_TRANSIT": 2}, 0, true),
		RequireConsumerRunning: true,
	})
	if out.StatusSummary == nil || out.StatusSummary.Source != SourceReadModel {
		t.Fatalf("expected read-model source, got %+v", out.StatusSummary)
	}
	if out.StatusSummary.LimitedDataset {
		t.Fatal("read-model success must not inherit legacy limited marker")
	}
	if out.StatusSummaryFreshness == nil || out.StatusSummaryFreshness.FallbackUsed || out.StatusSummaryFreshness.Partial {
		t.Fatalf("expected successful freshness, got %+v", out.StatusSummaryFreshness)
	}
}

func TestMergePrimaryFallbackOnError(t *testing.T) {
	out := Merge(MergeInput{
		Mode:                   ModePrimary,
		Legacy:                 legacyInput(3, 3, map[string]int64{"LOADED": 3}, false),
		ReadModelErr:           &DependencyError{Reason: ReasonTimeout},
		RequireConsumerRunning: true,
	})
	if out.StatusSummary == nil || out.StatusSummary.Source != SourceLegacy {
		t.Fatalf("expected legacy fallback, got %+v", out.StatusSummary)
	}
	if !containsWarning(out.Warnings, WarningUnavailable) || !containsWarning(out.Warnings, WarningFallbackUsed) {
		t.Fatalf("warnings=%v", out.Warnings)
	}
}

func TestMergePrimaryLimitedFallbackWarnings(t *testing.T) {
	out := Merge(MergeInput{
		Mode:                   ModePrimary,
		Legacy:                 legacyInput(1200, 100, map[string]int64{"IN_TRANSIT": 100}, true),
		ReadModelErr:           &DependencyError{Reason: ReasonNon2XX},
		RequireConsumerRunning: true,
	})
	if !out.StatusSummary.LimitedDataset || !out.StatusSummaryFreshness.Partial {
		t.Fatalf("expected limited fallback markers, summary=%+v freshness=%+v", out.StatusSummary, out.StatusSummaryFreshness)
	}
	for _, code := range []string{WarningUnavailable, WarningFallbackUsed, WarningLegacyLimited} {
		if !containsWarning(out.Warnings, code) {
			t.Fatalf("missing %s in %v", code, out.Warnings)
		}
	}
}

func TestMergePrimaryConsumerNotRunningFallback(t *testing.T) {
	out := Merge(MergeInput{
		Mode:                   ModePrimary,
		Legacy:                 legacyInput(1, 1, map[string]int64{"IN_TRANSIT": 1}, false),
		ReadModel:              readModelPayload(1, map[string]int64{"IN_TRANSIT": 1}, 0, false),
		RequireConsumerRunning: true,
	})
	if out.StatusSummary.Source != SourceLegacy {
		t.Fatalf("expected legacy fallback when consumer not running")
	}
}

func TestMergePrimaryPartialUsesReadModelWithWarning(t *testing.T) {
	out := Merge(MergeInput{
		Mode:                   ModePrimary,
		Legacy:                 legacyInput(2, 2, map[string]int64{"IN_TRANSIT": 2}, false),
		ReadModel:              readModelPayload(2, map[string]int64{"IN_TRANSIT": 2}, 1, true),
		RequireConsumerRunning: true,
	})
	if out.StatusSummary.Source != SourceReadModel {
		t.Fatal("partial projection should still use read-model")
	}
	if !containsWarning(out.Warnings, WarningPartial) {
		t.Fatalf("warnings=%v", out.Warnings)
	}
	if containsWarning(out.Warnings, WarningLegacyLimited) {
		t.Fatal("read-model partial must not include legacy limited warning")
	}
}

func TestMergeWarningsDedupedAndOrdered(t *testing.T) {
	warnings := dedupeWarnings([]string{
		WarningPartial,
		WarningLegacyLimited,
		WarningUnavailable,
		WarningFallbackUsed,
		WarningUnavailable,
	})
	if len(warnings) != 4 {
		t.Fatalf("deduped len=%d want 4: %v", len(warnings), warnings)
	}
	if warnings[0] != WarningUnavailable || warnings[1] != WarningFallbackUsed || warnings[2] != WarningLegacyLimited || warnings[3] != WarningPartial {
		t.Fatalf("order=%v", warnings)
	}
}

func containsWarning(warnings []string, code string) bool {
	for _, w := range warnings {
		if w == code {
			return true
		}
	}
	return false
}
