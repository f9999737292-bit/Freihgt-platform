package controltowerreadmodel

func Merge(input MergeInput) MergeOutput {
	out := MergeOutput{
		Warnings: []string{},
	}
	legacySummary := legacyFromInput(input.Legacy)

	switch input.Mode {
	case ModeDisabled:
		if input.Legacy.FullAggregateAvailable || input.Legacy.LimitedDataset || input.Legacy.CountedShipments > 0 {
			summary := legacySummary
			out.StatusSummary = &summary
			freshness := &StatusSummaryFreshness{
				Loaded:                true,
				Source:                SourceLegacy,
				LegacyAggregateLoaded: boolPtr(input.Legacy.FullAggregateAvailable),
			}
			if input.Legacy.LimitedDataset {
				freshness.Partial = true
				out.Warnings = append(out.Warnings, WarningLegacyLimited)
			}
			out.StatusSummaryFreshness = freshness
			out.Warnings = dedupeWarnings(out.Warnings)
		}
		return out
	case ModeShadow:
		if input.ReadModelErr != nil || input.ReadModel == nil {
			out.Comparison = ComparisonReadModelUnavailable
			if input.ReadModelErr != nil {
				out.FailureReason = input.ReadModelErr.Reason
			}
		} else if input.RequireConsumerRunning && !input.ReadModel.Freshness.ConsumerRunning {
			out.Comparison = ComparisonReadModelNotRunning
			out.FailureReason = ReasonConsumerNotRunning
		} else {
			out.Comparison = CompareStatusSummaries(input.Legacy, input.ReadModel)
		}
		if input.Legacy.FullAggregateAvailable || input.Legacy.LimitedDataset || input.Legacy.CountedShipments > 0 {
			summary := legacySummary
			out.StatusSummary = &summary
			freshness := &StatusSummaryFreshness{
				Loaded:                true,
				Source:                SourceLegacy,
				LegacyAggregateLoaded: boolPtr(input.Legacy.FullAggregateAvailable),
			}
			if input.Legacy.LimitedDataset {
				freshness.Partial = true
				out.Warnings = append(out.Warnings, WarningLegacyLimited)
			}
			out.StatusSummaryFreshness = freshness
			out.Warnings = dedupeWarnings(out.Warnings)
		}
		return out
	case ModePrimary:
		return mergePrimary(input, legacySummary)
	default:
		return out
	}
}

func mergePrimary(input MergeInput, legacySummary StatusSummary) MergeOutput {
	out := MergeOutput{Warnings: []string{}}

	useFallback := false
	if input.ReadModelErr != nil || input.ReadModel == nil {
		useFallback = true
		out.FailureReason = ReasonUnknown
		if input.ReadModelErr != nil {
			out.FailureReason = input.ReadModelErr.Reason
		}
		out.Warnings = append(out.Warnings, WarningUnavailable, WarningFallbackUsed)
	} else if input.RequireConsumerRunning && !input.ReadModel.Freshness.ConsumerRunning {
		useFallback = true
		out.FailureReason = ReasonConsumerNotRunning
		out.Warnings = append(out.Warnings, WarningConsumerNotRunning, WarningFallbackUsed)
	}

	if useFallback {
		summary := legacySummary
		out.StatusSummary = &summary
		partial := input.Legacy.LimitedDataset
		out.StatusSummaryFreshness = &StatusSummaryFreshness{
			Loaded:                true,
			FallbackUsed:          true,
			Partial:               partial,
			Source:                SourceLegacy,
			LegacyAggregateLoaded: boolPtr(input.Legacy.FullAggregateAvailable),
		}
		if input.Legacy.LimitedDataset {
			out.Warnings = append(out.Warnings, WarningLegacyLimited)
		}
		out.Warnings = dedupeWarnings(out.Warnings)
		return out
	}

	rmSummary := readModelFromInternal(input.ReadModel)
	out.StatusSummary = &rmSummary
	partial := input.ReadModel.IncompleteProjections > 0
	freshness := &StatusSummaryFreshness{
		Loaded:                  true,
		FallbackUsed:            false,
		Partial:                 partial,
		Source:                  SourceReadModel,
		ConsumerRunning:         boolPtr(input.ReadModel.Freshness.ConsumerRunning),
		LastRecordReceivedAt:    input.ReadModel.Freshness.LastRecordReceivedAt,
		LastProjectionAppliedAt: input.ReadModel.Freshness.LastProjectionAppliedAt,
	}
	out.StatusSummaryFreshness = freshness
	if partial {
		out.Warnings = append(out.Warnings, WarningPartial)
	}
	out.Warnings = dedupeWarnings(out.Warnings)
	return out
}

func dedupeWarnings(warnings []string) []string {
	if len(warnings) == 0 {
		return warnings
	}
	order := []string{
		WarningUnavailable,
		WarningConsumerNotRunning,
		WarningFallbackUsed,
		WarningLegacyLimited,
		WarningPartial,
	}
	seen := map[string]struct{}{}
	result := make([]string, 0, len(warnings))
	for _, code := range order {
		for _, w := range warnings {
			if w == code {
				if _, ok := seen[w]; !ok {
					seen[w] = struct{}{}
					result = append(result, w)
				}
			}
		}
	}
	for _, w := range warnings {
		if _, ok := seen[w]; !ok {
			seen[w] = struct{}{}
			result = append(result, w)
		}
	}
	return result
}

func boolPtr(v bool) *bool {
	return &v
}

func AppendUniqueWarnings(existing []string, additions []string) []string {
	return dedupeWarnings(append(existing, additions...))
}
