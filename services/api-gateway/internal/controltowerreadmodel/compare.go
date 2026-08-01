package controltowerreadmodel

type ComparisonResult string

const (
	ComparisonMatch                            ComparisonResult = "MATCH"
	ComparisonTotalMismatch                    ComparisonResult = "TOTAL_MISMATCH"
	ComparisonStatusCountMismatch              ComparisonResult = "STATUS_COUNT_MISMATCH"
	ComparisonLegacyUnavailable                ComparisonResult = "LEGACY_UNAVAILABLE"
	ComparisonReadModelUnavailable             ComparisonResult = "READ_MODEL_UNAVAILABLE"
	ComparisonReadModelNotRunning              ComparisonResult = "READ_MODEL_NOT_RUNNING"
	ComparisonLegacyLimitedDataset             ComparisonResult = "LEGACY_LIMITED_DATASET"
	ComparisonLegacyFullAggregateUnavailable   ComparisonResult = "LEGACY_FULL_AGGREGATE_UNAVAILABLE"
	ComparisonLegacyFullAggregateIncomplete    ComparisonResult = "LEGACY_FULL_AGGREGATE_INCOMPLETE"
)

func CompareStatusSummaries(legacy LegacyStatusInput, readModel *RemoteStatusSummary) ComparisonResult {
	if readModel == nil {
		return ComparisonReadModelUnavailable
	}
	if legacy.TotalShipments < 0 || legacy.ByStatus == nil {
		return ComparisonLegacyUnavailable
	}
	if legacy.FullAggregateIncomplete {
		return ComparisonLegacyFullAggregateIncomplete
	}
	if !legacy.FullAggregateAvailable {
		if legacy.LimitedDataset {
			return ComparisonLegacyLimitedDataset
		}
		return ComparisonLegacyFullAggregateUnavailable
	}
	if legacy.TotalShipments != readModel.TotalShipments {
		return ComparisonTotalMismatch
	}
	statuses := map[string]struct{}{}
	for status := range legacy.ByStatus {
		statuses[status] = struct{}{}
	}
	for status := range readModel.ByStatus {
		statuses[status] = struct{}{}
	}
	for status := range statuses {
		if legacy.ByStatus[status] != readModel.ByStatus[status] {
			return ComparisonStatusCountMismatch
		}
	}
	return ComparisonMatch
}
