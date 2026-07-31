package controltower

import "time"

var knownShipmentStatuses = map[string]struct{}{
	"CARRIER_ASSIGNED":             {},
	"ACCEPTED_BY_CARRIER":          {},
	"VEHICLE_ASSIGNED":             {},
	"DRIVER_ASSIGNED":              {},
	"PICKUP_SLOT_BOOKED":           {},
	"DELIVERY_SLOT_BOOKED":         {},
	"IN_PICKUP":                    {},
	"LOADED":                       {},
	"IN_TRANSIT":                   {},
	"ARRIVED_AT_CONSIGNEE":         {},
	"UNLOADING":                    {},
	"DELIVERED":                    {},
	"DELIVERY_CONFIRMED":           {},
	"DOCUMENTS_COMPLETED":          {},
	"READY_FOR_BILLING":            {},
	"INCLUDED_IN_BILLING_REGISTER": {},
	"FINANCIALLY_CLOSED":           {},
	"CANCELLED":                    {},
}

var prePickupStatuses = map[string]struct{}{
	"CARRIER_ASSIGNED":    {},
	"ACCEPTED_BY_CARRIER": {},
	"VEHICLE_ASSIGNED":    {},
	"DRIVER_ASSIGNED":     {},
	"PICKUP_SLOT_BOOKED":  {},
}

var inTransitStatuses = map[string]struct{}{
	"IN_PICKUP":            {},
	"LOADED":               {},
	"IN_TRANSIT":           {},
	"ARRIVED_AT_CONSIGNEE": {},
	"UNLOADING":            {},
}

var deliveredStatuses = map[string]struct{}{
	"DELIVERED":           {},
	"DELIVERY_CONFIRMED":  {},
	"DOCUMENTS_COMPLETED": {},
}

var terminalStatuses = map[string]struct{}{
	"CANCELLED":          {},
	"FINANCIALLY_CLOSED": {},
}

func IsActiveShipmentStatus(status string) bool {
	_, terminal := terminalStatuses[status]
	return !terminal
}

func IsKnownShipmentStatus(status string) bool {
	_, ok := knownShipmentStatuses[status]
	return ok
}

func slaPriority(status SLAStatus) int {
	switch status {
	case SLAStatusCritical:
		return 5
	case SLAStatusDelayed:
		return 4
	case SLAStatusAtRisk:
		return 3
	case SLAStatusOnTime:
		return 2
	default:
		return 1
	}
}

func pickHigher(current, candidate SLAResult) SLAResult {
	if slaPriority(candidate.Status) >= slaPriority(current.Status) {
		return candidate
	}
	return current
}

func minutesBetween(from, to time.Time) int64 {
	if to.Before(from) {
		return 0
	}
	return int64(to.Sub(from).Minutes())
}

func delayMinutesPast(deadline time.Time, now time.Time) int64 {
	if !now.After(deadline) {
		return 0
	}
	return minutesBetween(deadline, now)
}

func ComputeSLA(input SLAInput) SLAResult {
	now := input.Now
	if now.IsZero() {
		now = time.Now().UTC()
	}

	thresholds := input.Thresholds
	if thresholds.AtRiskMinutes <= 0 {
		thresholds.AtRiskMinutes = 120
	}
	if thresholds.CriticalDelayMinutes <= 0 {
		thresholds.CriticalDelayMinutes = 240
	}
	if thresholds.StaleWarningMinutes <= 0 {
		thresholds.StaleWarningMinutes = 120
	}
	if thresholds.StaleCriticalMinutes <= 0 {
		thresholds.StaleCriticalMinutes = 360
	}

	result := SLAResult{Status: SLAStatusUnknown, Reason: SLAReasonMissingPlannedDates}

	if input.TechnicalProblem {
		return SLAResult{Status: SLAStatusCritical, Reason: SLAReasonTechnicalProblem}
	}

	if input.Status == "CANCELLED" {
		return SLAResult{Status: SLAStatusCritical, Reason: SLAReasonCancelled}
	}

	if input.Status != "" && !IsKnownShipmentStatus(input.Status) {
		return SLAResult{Status: SLAStatusUnknown, Reason: SLAReasonUnknownStatus}
	}

	if input.PlannedPickupAt == nil && input.PlannedDeliveryAt == nil {
		return SLAResult{Status: SLAStatusUnknown, Reason: SLAReasonMissingPlannedDates}
	}

	atRiskWindow := time.Duration(thresholds.AtRiskMinutes) * time.Minute
	criticalDelay := time.Duration(thresholds.CriticalDelayMinutes) * time.Minute
	staleWarning := time.Duration(thresholds.StaleWarningMinutes) * time.Minute
	staleCritical := time.Duration(thresholds.StaleCriticalMinutes) * time.Minute

	if input.LastUpdatedAt != nil && isStaleEligibleStatus(input.Status) {
		staleFor := now.Sub(*input.LastUpdatedAt)
		if staleFor >= staleCritical {
			result = pickHigher(result, SLAResult{
				Status:       SLAStatusCritical,
				Reason:       SLAReasonStaleUpdates,
				DelayMinutes: int64Ptr(minutesBetween(*input.LastUpdatedAt, now)),
			})
		} else if staleFor >= staleWarning {
			result = pickHigher(result, SLAResult{
				Status:       SLAStatusAtRisk,
				Reason:       SLAReasonStaleUpdates,
				DelayMinutes: int64Ptr(minutesBetween(*input.LastUpdatedAt, now)),
			})
		}
	}

	if input.PlannedPickupAt != nil && input.ActualPickupAt == nil && hasStatus(input.Status, prePickupStatuses) {
		if now.After(*input.PlannedPickupAt) {
			delay := delayMinutesPast(*input.PlannedPickupAt, now)
			if now.Sub(*input.PlannedPickupAt) >= criticalDelay {
				result = pickHigher(result, SLAResult{
					Status:       SLAStatusCritical,
					Reason:       SLAReasonPickupOverdue,
					DelayMinutes: int64Ptr(delay),
				})
			} else {
				result = pickHigher(result, SLAResult{
					Status:       SLAStatusDelayed,
					Reason:       SLAReasonPickupOverdue,
					DelayMinutes: int64Ptr(delay),
				})
			}
		} else if input.PlannedPickupAt.Sub(now) <= atRiskWindow {
			result = pickHigher(result, SLAResult{Status: SLAStatusAtRisk, Reason: SLAReasonPickupAtRisk})
		}
	}

	if input.PlannedDeliveryAt != nil && input.ActualDeliveryAt == nil && !hasStatus(input.Status, deliveredStatuses) && input.Status != "READY_FOR_BILLING" {
		if now.After(*input.PlannedDeliveryAt) {
			delay := delayMinutesPast(*input.PlannedDeliveryAt, now)
			if now.Sub(*input.PlannedDeliveryAt) >= criticalDelay {
				result = pickHigher(result, SLAResult{
					Status:       SLAStatusCritical,
					Reason:       SLAReasonDeliveryOverdue,
					DelayMinutes: int64Ptr(delay),
				})
			} else {
				result = pickHigher(result, SLAResult{
					Status:       SLAStatusDelayed,
					Reason:       SLAReasonDeliveryOverdue,
					DelayMinutes: int64Ptr(delay),
				})
			}
		} else if input.PlannedDeliveryAt.Sub(now) <= atRiskWindow && hasStatus(input.Status, inTransitStatuses) {
			result = pickHigher(result, SLAResult{Status: SLAStatusAtRisk, Reason: SLAReasonDeliveryAtRisk})
		}
	}

	if input.ActualDeliveryAt != nil && input.PlannedDeliveryAt != nil {
		if input.ActualDeliveryAt.After(*input.PlannedDeliveryAt) {
			delay := delayMinutesPast(*input.PlannedDeliveryAt, *input.ActualDeliveryAt)
			if input.ActualDeliveryAt.Sub(*input.PlannedDeliveryAt) >= criticalDelay {
				result = pickHigher(result, SLAResult{
					Status:       SLAStatusCritical,
					Reason:       SLAReasonCompletedLate,
					DelayMinutes: int64Ptr(delay),
				})
			} else {
				result = pickHigher(result, SLAResult{
					Status:       SLAStatusDelayed,
					Reason:       SLAReasonCompletedLate,
					DelayMinutes: int64Ptr(delay),
				})
			}
		} else if slaPriority(result.Status) < slaPriority(SLAStatusOnTime) {
			result = SLAResult{Status: SLAStatusOnTime, Reason: SLAReasonCompletedOnTime}
		}
	}

	if slaPriority(result.Status) <= slaPriority(SLAStatusOnTime) {
		switch result.Reason {
		case SLAReasonCompletedOnTime, SLAReasonCompletedLate:
			// preserve completion-specific SLA reasons
		default:
			if hasStatus(input.Status, deliveredStatuses) || input.Status == "READY_FOR_BILLING" || input.ActualPickupAt != nil || input.ActualDeliveryAt != nil {
				result = SLAResult{Status: SLAStatusOnTime, Reason: SLAReasonOnSchedule}
			} else if input.PlannedPickupAt != nil || input.PlannedDeliveryAt != nil {
				result = SLAResult{Status: SLAStatusOnTime, Reason: SLAReasonOnSchedule}
			}
		}
	}

	return result
}

func hasStatus(status string, set map[string]struct{}) bool {
	_, ok := set[status]
	return ok
}

func isStaleEligibleStatus(status string) bool {
	return hasStatus(status, prePickupStatuses) || hasStatus(status, inTransitStatuses)
}

func int64Ptr(v int64) *int64 {
	return &v
}
