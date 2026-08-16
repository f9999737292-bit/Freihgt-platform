package domain

import "time"

const (
	SlotWarningBufferMinutes = 15
)

type SlotPolicy struct {
	WarningBufferBeforeEnd time.Duration
	EarlyTolerance         time.Duration
	LateTolerance          time.Duration
}

func DefaultSlotPolicy() SlotPolicy {
	return SlotPolicy{
		WarningBufferBeforeEnd: time.Duration(SlotWarningBufferMinutes) * time.Minute,
		EarlyTolerance:         0,
		LateTolerance:          0,
	}
}

func ApplySlotArrivalAssessment(
	summary *SlotTargetSummary,
	eta ETASnapshot,
	actualArrival *time.Time,
	milestoneCompleted bool,
	policy SlotPolicy,
) {
	if summary == nil {
		return
	}
	summary.ArrivalProjection = SlotArrivalUnknown
	summary.ETARelation = "unknown"
	summary.ProjectedLateBySeconds = nil
	summary.EarlyBySeconds = nil
	summary.MarginSeconds = nil

	if summary.WindowStatus != SlotWindowAvailable || summary.WindowStart == nil || summary.WindowEnd == nil {
		return
	}
	windowStart := summary.WindowStart.UTC()
	windowEnd := summary.WindowEnd.UTC()

	if milestoneCompleted && actualArrival != nil {
		actual := actualArrival.UTC()
		summary.ETARelation = "actual_milestone"
		if actual.After(windowEnd.Add(policy.LateTolerance)) {
			summary.ArrivalProjection = SlotArrivalMissed
			late := int64(actual.Sub(windowEnd).Seconds())
			summary.ProjectedLateBySeconds = &late
			return
		}
		if actual.Before(windowStart.Add(-policy.EarlyTolerance)) {
			summary.ArrivalProjection = SlotArrivalEarly
			early := int64(windowStart.Sub(actual).Seconds())
			summary.EarlyBySeconds = &early
			return
		}
		summary.ArrivalProjection = SlotArrivalCompleted
		return
	}

	if summary.SlotStatus != nil && *summary.SlotStatus == SlotStatusMissed {
		summary.ArrivalProjection = SlotArrivalMissed
		summary.ETARelation = "slot_status"
		return
	}

	if !eta.HasUsableETA || eta.EstimatedArrivalAt == nil {
		return
	}
	if eta.Status == ETAStatusExpired || eta.FreshnessStatus == ETAFreshnessExpired {
		return
	}

	etaTime := eta.EstimatedArrivalAt.UTC()
	summary.ETARelation = "eta_prediction"
	if eta.FreshnessStatus == ETAFreshnessStale {
		summary.ETARelation = "eta_prediction_stale"
	}

	if etaTime.Before(windowStart.Add(-policy.EarlyTolerance)) {
		summary.ArrivalProjection = SlotArrivalEarly
		early := int64(windowStart.Sub(etaTime).Seconds())
		summary.EarlyBySeconds = &early
		return
	}
	if etaTime.After(windowEnd.Add(policy.LateTolerance)) {
		summary.ArrivalProjection = SlotArrivalProjectedMiss
		late := int64(etaTime.Sub(windowEnd).Seconds())
		summary.ProjectedLateBySeconds = &late
		return
	}
	riskThreshold := windowEnd.Add(-policy.WarningBufferBeforeEnd)
	if etaTime.After(riskThreshold) {
		summary.ArrivalProjection = SlotArrivalAtRisk
		margin := int64(windowEnd.Sub(etaTime).Seconds())
		summary.MarginSeconds = &margin
		return
	}
	summary.ArrivalProjection = SlotArrivalOnTime
}

func SlotETAUsableForRisk(eta ETASnapshot) bool {
	if !eta.HasUsableETA || eta.EstimatedArrivalAt == nil {
		return false
	}
	if eta.Status == ETAStatusExpired || eta.FreshnessStatus == ETAFreshnessExpired {
		return false
	}
	if eta.QualityStatus == ETAQualityPoor {
		return false
	}
	return true
}
