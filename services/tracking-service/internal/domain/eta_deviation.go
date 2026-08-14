package domain

import "time"

const (
	DeviationEarlyMinutes  = 15
	DeviationOnTimeMinutes = 15
	DeviationAtRiskMinutes = 30
)

func ProjectedDeviationSeconds(estimated, planned time.Time) int64 {
	return int64(estimated.Sub(planned).Seconds())
}

func ClassifyArrivalProjection(deviationSeconds int64, hasUsableETA bool) string {
	if !hasUsableETA {
		return ArrivalUnknown
	}
	deviation := time.Duration(deviationSeconds) * time.Second
	earlyThreshold := -time.Duration(DeviationEarlyMinutes) * time.Minute
	onTimeUpper := time.Duration(DeviationOnTimeMinutes) * time.Minute
	atRiskUpper := time.Duration(DeviationAtRiskMinutes) * time.Minute

	switch {
	case deviation < earlyThreshold:
		return ArrivalEarly
	case deviation <= onTimeUpper:
		return ArrivalOnTime
	case deviation <= atRiskUpper:
		return ArrivalAtRisk
	default:
		return ArrivalLate
	}
}

func ApplyDeviation(summary *ETATargetSummary, plannedArrivalAt *time.Time, usableETA bool) {
	if summary == nil || plannedArrivalAt == nil || summary.EstimatedArrivalAt == nil || !usableETA {
		if summary != nil {
			summary.ArrivalProjection = ArrivalUnknown
		}
		return
	}
	summary.PlannedArrivalAt = plannedArrivalAt
	dev := ProjectedDeviationSeconds(*summary.EstimatedArrivalAt, plannedArrivalAt.UTC())
	summary.ProjectedDeviationSeconds = &dev
	summary.ArrivalProjection = ClassifyArrivalProjection(dev, usableETA)
}
