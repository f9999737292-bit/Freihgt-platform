package domain

import (
	"time"

	"github.com/google/uuid"
)

const (
	WorkflowStatusOpen         = "open"
	WorkflowStatusAcknowledged = "acknowledged"
	WorkflowStatusAssigned     = "assigned"
	WorkflowStatusResolved     = "resolved"

	ActionTypeAcknowledged = "acknowledged"
	ActionTypeAssigned     = "assigned"
	ActionTypeReassigned   = "reassigned"
	ActionTypeResolved     = "resolved"
	ActionTypeReopened     = "reopened"

	ResolutionCodeIssueResolved = "issue_resolved"
	ResolutionCodeFalsePositive = "false_positive"
	ResolutionCodeDuplicate     = "duplicate"
	ResolutionCodeCancelled     = "cancelled"
	ResolutionCodeOther         = "other"
)

var validWorkflowStatuses = map[string]struct{}{
	WorkflowStatusOpen:         {},
	WorkflowStatusAcknowledged: {},
	WorkflowStatusAssigned:     {},
	WorkflowStatusResolved:     {},
}

var validResolutionCodes = map[string]struct{}{
	ResolutionCodeIssueResolved: {},
	ResolutionCodeFalsePositive: {},
	ResolutionCodeDuplicate:     {},
	ResolutionCodeCancelled:     {},
	ResolutionCodeOther:         {},
}

type CriticalEventWorkflow struct {
	TenantID             uuid.UUID
	EventID              string
	ShipmentID           uuid.UUID
	EventType            string
	Source               string
	OccurredAt           time.Time
	Status               string
	Version              int
	AcknowledgedAt       *time.Time
	AcknowledgedByUserID *uuid.UUID
	AssignedToUserID     *uuid.UUID
	AssignedByUserID     *uuid.UUID
	AssignedAt           *time.Time
	ResolvedByUserID     *uuid.UUID
	ResolvedAt           *time.Time
	ResolutionCode       *string
	ResolutionComment    *string
	LastReopenedAt       *time.Time
	LastReopenedByUserID *uuid.UUID
	CreatedAt            time.Time
	UpdatedAt            time.Time
}

type CriticalEventAction struct {
	ID          int64
	TenantID    uuid.UUID
	EventID     string
	ActionType  string
	ActorUserID uuid.UUID
	OccurredAt  time.Time
	Metadata    map[string]any
}

type AcknowledgeCriticalEventWorkflowInput struct {
	TenantID   uuid.UUID
	UserID     uuid.UUID
	EventID    string
	ShipmentID uuid.UUID
	EventType  string
	Source     string
	OccurredAt time.Time
}

type AssignCriticalEventInput struct {
	TenantID       uuid.UUID
	ActorUserID    uuid.UUID
	EventID        string
	AssignedToUser uuid.UUID
}

type ResolveCriticalEventInput struct {
	TenantID          uuid.UUID
	ActorUserID       uuid.UUID
	EventID           string
	ResolutionCode    string
	ResolutionComment *string
}

type ReopenCriticalEventInput struct {
	TenantID    uuid.UUID
	ActorUserID uuid.UUID
	EventID     string
}

func ValidWorkflowStatus(status string) bool {
	_, ok := validWorkflowStatuses[status]
	return ok
}

func ValidResolutionCode(code string) bool {
	_, ok := validResolutionCodes[code]
	return ok
}

func CanAssignFromStatus(status string) bool {
	return status == WorkflowStatusAcknowledged || status == WorkflowStatusAssigned
}

func AssignActionType(status string) string {
	if status == WorkflowStatusAssigned {
		return ActionTypeReassigned
	}
	return ActionTypeAssigned
}
