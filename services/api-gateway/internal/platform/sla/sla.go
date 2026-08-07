package sla

import "time"

type Status string

const (
	StatusOnTime   Status = "ON_TIME"
	StatusAtRisk   Status = "AT_RISK"
	StatusDelayed  Status = "DELAYED"
	StatusCritical Status = "CRITICAL"
	StatusUnknown  Status = "UNKNOWN"
)

const (
	ReasonMissingPlannedDates = "MISSING_PLANNED_DATES"
	ReasonOnSchedule          = "ON_SCHEDULE"
	ReasonPickupAtRisk        = "PICKUP_AT_RISK"
	ReasonPickupOverdue       = "PICKUP_OVERDUE"
	ReasonDeliveryAtRisk      = "DELIVERY_AT_RISK"
	ReasonDeliveryOverdue     = "DELIVERY_OVERDUE"
	ReasonStaleUpdates        = "STALE_UPDATES"
	ReasonCancelled           = "CANCELLED"
	ReasonTechnicalProblem    = "TECHNICAL_PROBLEM"
	ReasonCompletedOnTime     = "COMPLETED_ON_TIME"
	ReasonCompletedLate       = "COMPLETED_LATE"
	ReasonUnknownStatus       = "UNKNOWN_STATUS"
)

type Thresholds struct {
	AtRiskMinutes        int
	CriticalDelayMinutes int
	StaleWarningMinutes  int
	StaleCriticalMinutes int
}

type Input struct {
	Status            string
	PlannedPickupAt   *time.Time
	PlannedDeliveryAt *time.Time
	ActualPickupAt    *time.Time
	ActualDeliveryAt  *time.Time
	LastUpdatedAt     *time.Time
	TechnicalProblem  bool
	Now               time.Time
	Thresholds        Thresholds
}

type Result struct {
	Status       Status
	Reason       string
	DelayMinutes *int64
}

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

func IsDeliveredShipmentStatus(status string) bool {
	return hasStatus(status, deliveredStatuses)
}

func priority(status Status) int {
	switch status {
	case StatusCritical:
		return 5
	case StatusDelayed:
		return 4
	case StatusAtRisk:
		return 3
	case StatusOnTime:
		return 2
	default:
		return 1
	}
}

func pickHigher(current, candidate Result) Result {
	if priority(candidate.Status) >= priority(current.Status) {
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

func Compute(input Input) Result {
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

	result := Result{Status: StatusUnknown, Reason: ReasonMissingPlannedDates}

	if input.TechnicalProblem {
		return Result{Status: StatusCritical, Reason: ReasonTechnicalProblem}
	}

	if input.Status == "CANCELLED" {
		return Result{Status: StatusCritical, Reason: ReasonCancelled}
	}

	if input.Status != "" && !IsKnownShipmentStatus(input.Status) {
		return Result{Status: StatusUnknown, Reason: ReasonUnknownStatus}
	}

	if input.PlannedPickupAt == nil && input.PlannedDeliveryAt == nil {
		return Result{Status: StatusUnknown, Reason: ReasonMissingPlannedDates}
	}

	atRiskWindow := time.Duration(thresholds.AtRiskMinutes) * time.Minute
	criticalDelay := time.Duration(thresholds.CriticalDelayMinutes) * time.Minute
	staleWarning := time.Duration(thresholds.StaleWarningMinutes) * time.Minute
	staleCritical := time.Duration(thresholds.StaleCriticalMinutes) * time.Minute

	if input.LastUpdatedAt != nil && isStaleEligibleStatus(input.Status) {
		staleFor := now.Sub(*input.LastUpdatedAt)
		if staleFor >= staleCritical {
			result = pickHigher(result, Result{
				Status:       StatusCritical,
				Reason:       ReasonStaleUpdates,
				DelayMinutes: int64Ptr(minutesBetween(*input.LastUpdatedAt, now)),
			})
		} else if staleFor >= staleWarning {
			result = pickHigher(result, Result{
				Status:       StatusAtRisk,
				Reason:       ReasonStaleUpdates,
				DelayMinutes: int64Ptr(minutesBetween(*input.LastUpdatedAt, now)),
			})
		}
	}

	if input.PlannedPickupAt != nil && input.ActualPickupAt == nil && hasStatus(input.Status, prePickupStatuses) {
		if now.After(*input.PlannedPickupAt) {
			delay := delayMinutesPast(*input.PlannedPickupAt, now)
			if now.Sub(*input.PlannedPickupAt) >= criticalDelay {
				result = pickHigher(result, Result{
					Status:       StatusCritical,
					Reason:       ReasonPickupOverdue,
					DelayMinutes: int64Ptr(delay),
				})
			} else {
				result = pickHigher(result, Result{
					Status:       StatusDelayed,
					Reason:       ReasonPickupOverdue,
					DelayMinutes: int64Ptr(delay),
				})
			}
		} else if input.PlannedPickupAt.Sub(now) <= atRiskWindow {
			result = pickHigher(result, Result{Status: StatusAtRisk, Reason: ReasonPickupAtRisk})
		}
	}

	if input.PlannedDeliveryAt != nil && input.ActualDeliveryAt == nil && !hasStatus(input.Status, deliveredStatuses) && input.Status != "READY_FOR_BILLING" {
		if now.After(*input.PlannedDeliveryAt) {
			delay := delayMinutesPast(*input.PlannedDeliveryAt, now)
			if now.Sub(*input.PlannedDeliveryAt) >= criticalDelay {
				result = pickHigher(result, Result{
					Status:       StatusCritical,
					Reason:       ReasonDeliveryOverdue,
					DelayMinutes: int64Ptr(delay),
				})
			} else {
				result = pickHigher(result, Result{
					Status:       StatusDelayed,
					Reason:       ReasonDeliveryOverdue,
					DelayMinutes: int64Ptr(delay),
				})
			}
		} else if input.PlannedDeliveryAt.Sub(now) <= atRiskWindow && hasStatus(input.Status, inTransitStatuses) {
			result = pickHigher(result, Result{Status: StatusAtRisk, Reason: ReasonDeliveryAtRisk})
		}
	}

	if input.ActualDeliveryAt != nil && input.PlannedDeliveryAt != nil {
		if input.ActualDeliveryAt.After(*input.PlannedDeliveryAt) {
			delay := delayMinutesPast(*input.PlannedDeliveryAt, *input.ActualDeliveryAt)
			if input.ActualDeliveryAt.Sub(*input.PlannedDeliveryAt) >= criticalDelay {
				result = pickHigher(result, Result{
					Status:       StatusCritical,
					Reason:       ReasonCompletedLate,
					DelayMinutes: int64Ptr(delay),
				})
			} else {
				result = pickHigher(result, Result{
					Status:       StatusDelayed,
					Reason:       ReasonCompletedLate,
					DelayMinutes: int64Ptr(delay),
				})
			}
		} else if priority(result.Status) < priority(StatusOnTime) {
			result = Result{Status: StatusOnTime, Reason: ReasonCompletedOnTime}
		}
	}

	if priority(result.Status) <= priority(StatusOnTime) {
		switch result.Reason {
		case ReasonCompletedOnTime, ReasonCompletedLate:
			// preserve completion-specific SLA reasons
		default:
			if hasStatus(input.Status, deliveredStatuses) || input.Status == "READY_FOR_BILLING" || input.ActualPickupAt != nil || input.ActualDeliveryAt != nil {
				result = Result{Status: StatusOnTime, Reason: ReasonOnSchedule}
			} else if input.PlannedPickupAt != nil || input.PlannedDeliveryAt != nil {
				result = Result{Status: StatusOnTime, Reason: ReasonOnSchedule}
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
