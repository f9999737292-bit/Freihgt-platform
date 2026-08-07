package controltower

import "github.com/freight-platform/api-gateway/internal/platform/sla"

type SLAStatus = sla.Status

const (
	SLAStatusOnTime   = sla.StatusOnTime
	SLAStatusAtRisk   = sla.StatusAtRisk
	SLAStatusDelayed  = sla.StatusDelayed
	SLAStatusCritical = sla.StatusCritical
	SLAStatusUnknown  = sla.StatusUnknown
)

const (
	SLAReasonMissingPlannedDates = sla.ReasonMissingPlannedDates
	SLAReasonOnSchedule          = sla.ReasonOnSchedule
	SLAReasonPickupAtRisk        = sla.ReasonPickupAtRisk
	SLAReasonPickupOverdue       = sla.ReasonPickupOverdue
	SLAReasonDeliveryAtRisk      = sla.ReasonDeliveryAtRisk
	SLAReasonDeliveryOverdue     = sla.ReasonDeliveryOverdue
	SLAReasonStaleUpdates        = sla.ReasonStaleUpdates
	SLAReasonCancelled           = sla.ReasonCancelled
	SLAReasonTechnicalProblem    = sla.ReasonTechnicalProblem
	SLAReasonCompletedOnTime     = sla.ReasonCompletedOnTime
	SLAReasonCompletedLate       = sla.ReasonCompletedLate
	SLAReasonUnknownStatus       = sla.ReasonUnknownStatus
)

type SLAThresholds = sla.Thresholds
type SLAInput = sla.Input
type SLAResult = sla.Result

func ComputeSLA(input SLAInput) SLAResult {
	return sla.Compute(input)
}

func IsActiveShipmentStatus(status string) bool {
	return sla.IsActiveShipmentStatus(status)
}

func IsKnownShipmentStatus(status string) bool {
	return sla.IsKnownShipmentStatus(status)
}

func IsDeliveredShipmentStatus(status string) bool {
	return sla.IsDeliveredShipmentStatus(status)
}
