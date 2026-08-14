package domain

import (
	"time"

	"github.com/google/uuid"
)

const (
	PriorityP1 = "p1"
	PriorityP2 = "p2"
	PriorityP3 = "p3"
	PriorityP4 = "p4"

	CategoryDelay            = "delay"
	CategoryRouteDeviation   = "route_deviation"
	CategoryDocumentIssue    = "document_issue"
	CategoryVehicleIssue     = "vehicle_issue"
	CategoryDriverIssue      = "driver_issue"
	CategorySlotIssue        = "slot_issue"
	CategoryDeliveryIssue    = "delivery_issue"
	CategoryPickupIssue      = "pickup_issue"
	CategoryBillingIssue     = "billing_issue"
	CategoryIntegrationIssue = "integration_issue"
	CategoryDataQuality      = "data_quality"
	CategoryOther            = "other"

	BusinessImpactNone     = "none"
	BusinessImpactLow      = "low"
	BusinessImpactMedium   = "medium"
	BusinessImpactHigh     = "high"
	BusinessImpactCritical = "critical"

	SLAStatusWithinSLA = "within_sla"
	SLAStatusWarning   = "warning"
	SLAStatusBreached  = "breached"
	SLAStatusCompleted = "completed"

	SLAPhaseAcknowledgement = "acknowledgement"
	SLAPhaseAssignment      = "assignment"
	SLAPhaseResolution      = "resolution"

	EscalationNone   = "none"
	EscalationLevel1 = "level_1"
	EscalationLevel2 = "level_2"
	EscalationLevel3 = "level_3"

	ActionTypeExceptionUpdated   = "exception_updated"
	ActionTypeAckSLABreached     = "ack_sla_breached"
	ActionTypeAssignSLABreached  = "assign_sla_breached"
	ActionTypeResolveSLABreached = "resolve_sla_breached"
	ActionTypeEscalationChanged  = "escalation_changed"

	SLAWarningThresholdRatio = 0.20
)

type SLAPolicy struct {
	Acknowledge time.Duration
	Assign      time.Duration
	Resolve     time.Duration
}

var defaultSLAPolicies = map[string]SLAPolicy{
	PriorityP1: {Acknowledge: 5 * time.Minute, Assign: 10 * time.Minute, Resolve: 60 * time.Minute},
	PriorityP2: {Acknowledge: 15 * time.Minute, Assign: 30 * time.Minute, Resolve: 4 * time.Hour},
	PriorityP3: {Acknowledge: 60 * time.Minute, Assign: 120 * time.Minute, Resolve: 24 * time.Hour},
	PriorityP4: {Acknowledge: 240 * time.Minute, Assign: 480 * time.Minute, Resolve: 72 * time.Hour},
}

var validPriorities = map[string]struct{}{
	PriorityP1: {}, PriorityP2: {}, PriorityP3: {}, PriorityP4: {},
}

var validCategories = map[string]struct{}{
	CategoryDelay: {}, CategoryRouteDeviation: {}, CategoryDocumentIssue: {},
	CategoryVehicleIssue: {}, CategoryDriverIssue: {}, CategorySlotIssue: {},
	CategoryDeliveryIssue: {}, CategoryPickupIssue: {}, CategoryBillingIssue: {},
	CategoryIntegrationIssue: {}, CategoryDataQuality: {}, CategoryOther: {},
}

var validBusinessImpacts = map[string]struct{}{
	BusinessImpactNone: {}, BusinessImpactLow: {}, BusinessImpactMedium: {},
	BusinessImpactHigh: {}, BusinessImpactCritical: {},
}

type ExceptionDeadlines struct {
	AcknowledgeDueAt time.Time
	AssignmentDueAt  time.Time
	ResolutionDueAt  time.Time
}

type SLAEvaluation struct {
	Phase            string
	Status           string
	RemainingSeconds *int64
}

type UpdateExceptionInput struct {
	TenantID       uuid.UUID
	ActorUserID    uuid.UUID
	EventID        string
	Priority       *string
	Category       *string
	BusinessImpact *string
}

type EnsureExceptionSeed struct {
	EventID    string
	ShipmentID string
	EventType  string
	Source     string
	OccurredAt time.Time
	Severity   string
}

func ValidPriority(priority string) bool {
	_, ok := validPriorities[priority]
	return ok
}

func ValidExceptionCategory(category string) bool {
	_, ok := validCategories[category]
	return ok
}

func ValidBusinessImpact(impact string) bool {
	_, ok := validBusinessImpacts[impact]
	return ok
}

func SLAPolicyForPriority(priority string) SLAPolicy {
	if policy, ok := defaultSLAPolicies[priority]; ok {
		return policy
	}
	return defaultSLAPolicies[PriorityP3]
}

func DefaultPriorityForSeverity(severity string) string {
	switch severity {
	case "CRITICAL":
		return PriorityP1
	case "WARNING":
		return PriorityP2
	case "INFO":
		return PriorityP4
	default:
		return PriorityP3
	}
}

func DefaultCategoryForEventType(eventType string) string {
	switch eventType {
	case "PICKUP_DELAY":
		return CategoryPickupIssue
	case "DELIVERY_DELAY":
		return CategoryDeliveryIssue
	case "STALE_UPDATES", "NO_GEOLOCATION", "ROUTE_DEVIATION":
		return CategoryDelay
	case "MISSING_DOCUMENTS":
		return CategoryDocumentIssue
	case "SHIPMENT_CANCELLED":
		return CategoryDeliveryIssue
	case "TECHNICAL_PROBLEM", "TECHNICAL_ISSUE":
		return CategoryIntegrationIssue
	default:
		return CategoryOther
	}
}

// CalculateDeadlines derives SLA deadlines from exception activation time and priority.
// Semantics: when priority changes before resolution, unresolved deadlines are
// recalculated from exception_activated_at using the new policy. Completed phases
// retain their historical completion timestamps and are not rewritten.
func CalculateDeadlines(priority string, activatedAt time.Time) ExceptionDeadlines {
	policy := SLAPolicyForPriority(priority)
	base := activatedAt.UTC()
	return ExceptionDeadlines{
		AcknowledgeDueAt: base.Add(policy.Acknowledge),
		AssignmentDueAt:  base.Add(policy.Assign),
		ResolutionDueAt:  base.Add(policy.Resolve),
	}
}

func RecalculateUnresolvedDeadlines(workflow CriticalEventWorkflow, priority string) ExceptionDeadlines {
	deadlines := CalculateDeadlines(priority, workflow.ExceptionActivatedAt)
	if workflow.AcknowledgedAt != nil {
		deadlines.AcknowledgeDueAt = workflow.AcknowledgeDueAt
	}
	if workflow.AssignedAt != nil {
		deadlines.AssignmentDueAt = workflow.AssignmentDueAt
	}
	return deadlines
}

func EvaluateSLA(workflow CriticalEventWorkflow, now time.Time) SLAEvaluation {
	if workflow.Status == WorkflowStatusResolved {
		remaining := int64(0)
		return SLAEvaluation{Phase: SLAPhaseResolution, Status: SLAStatusCompleted, RemainingSeconds: &remaining}
	}

	now = now.UTC()

	switch workflow.Status {
	case WorkflowStatusOpen:
		return evaluateOpenPhase(
			workflow.AcknowledgedAt,
			workflow.ExceptionActivatedAt,
			workflow.AcknowledgeDueAt,
			now,
			SLAPhaseAcknowledgement,
		)
	case WorkflowStatusAcknowledged:
		phaseStart := workflow.ExceptionActivatedAt
		if workflow.AcknowledgedAt != nil {
			phaseStart = workflow.AcknowledgedAt.UTC()
		}
		return evaluateOpenPhase(
			workflow.AssignedAt,
			phaseStart,
			workflow.AssignmentDueAt,
			now,
			SLAPhaseAssignment,
		)
	default:
		phaseStart := workflow.ExceptionActivatedAt
		if workflow.AssignedAt != nil {
			phaseStart = workflow.AssignedAt.UTC()
		}
		return evaluateOpenPhase(
			workflow.ResolvedAt,
			phaseStart,
			workflow.ResolutionDueAt,
			now,
			SLAPhaseResolution,
		)
	}
}

func evaluateOpenPhase(
	completedAt *time.Time,
	phaseStart time.Time,
	dueAt time.Time,
	now time.Time,
	phase string,
) SLAEvaluation {
	if completedAt != nil {
		remaining := int64(0)
		return SLAEvaluation{Phase: phase, Status: SLAStatusCompleted, RemainingSeconds: &remaining}
	}

	remaining := int64(dueAt.Sub(now).Seconds())
	totalWindow := dueAt.Sub(phaseStart).Seconds()
	if totalWindow <= 0 {
		totalWindow = 1
	}

	status := SLAStatusWithinSLA
	if remaining <= 0 {
		status = SLAStatusBreached
	} else if float64(remaining) <= totalWindow*SLAWarningThresholdRatio {
		status = SLAStatusWarning
	}

	return SLAEvaluation{Phase: phase, Status: status, RemainingSeconds: &remaining}
}

func EvaluateEscalation(priority string, sla SLAEvaluation) string {
	if sla.Status == SLAStatusCompleted {
		return EscalationNone
	}
	if sla.Status == SLAStatusBreached {
		if sla.Phase == SLAPhaseResolution && priority == PriorityP1 {
			return EscalationLevel3
		}
		return EscalationLevel2
	}
	if sla.Status == SLAStatusWarning {
		return EscalationLevel1
	}
	return EscalationNone
}

func PriorityRank(priority string) int {
	switch priority {
	case PriorityP1:
		return 1
	case PriorityP2:
		return 2
	case PriorityP3:
		return 3
	case PriorityP4:
		return 4
	default:
		return 5
	}
}

func SLAStatusRank(status string) int {
	switch status {
	case SLAStatusBreached:
		return 1
	case SLAStatusWarning:
		return 2
	case SLAStatusWithinSLA:
		return 3
	default:
		return 4
	}
}

func int64Ptr(v int64) *int64 {
	return &v
}
