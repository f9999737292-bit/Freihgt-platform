package domain

import (
	"time"

	"github.com/google/uuid"
)

const (
	CaseFilterSchemaVersion = 2

	CaseStatusOpen           = "open"
	CaseStatusInvestigating  = "investigating"
	CaseStatusActionRequired = "action_required"
	CaseStatusMonitoring     = "monitoring"
	CaseStatusResolved       = "resolved"
	CaseStatusClosed         = "closed"

	CaseSeverityCritical = "critical"
	CaseSeverityHigh     = "high"
	CaseSeverityMedium   = "medium"
	CaseSeverityLow      = "low"

	CaseLinkShipment       = "shipment"
	CaseLinkTransportOrder = "transport_order"
	CaseLinkException      = "exception"
	CaseLinkRisk           = "risk"
	CaseLinkWorkItem       = "work_item"

	ParticipantRoleOwner        = "owner"
	ParticipantRoleCollaborator = "collaborator"
	ParticipantRoleObserver     = "observer"

	NoteVisibilityInternal = "internal"

	ActionItemStatusOpen       = "open"
	ActionItemStatusInProgress = "in_progress"
	ActionItemStatusDone       = "done"
	ActionItemStatusCancelled  = "cancelled"

	CaseEventSourceCase       = "CASE"
	CaseEventSourceNote       = "NOTE"
	CaseEventSourceActionItem = "ACTION_ITEM"
	CaseEventSourceDecision   = "DECISION"
	CaseEventSourceSystem     = "SYSTEM"

	WorkspaceScopeWorkItems = "work_items"
	WorkspaceScopeCases     = "cases"

	CasesMaxPageLimit = 100
)

var ActiveCaseStatuses = map[string]struct{}{
	CaseStatusOpen: {}, CaseStatusInvestigating: {}, CaseStatusActionRequired: {},
	CaseStatusMonitoring: {},
}

var CaseResolutionCodes = []string{
	"operational_issue_resolved", "risk_cleared", "shipment_replanned",
	"carrier_action_completed", "customer_accepted", "duplicate_case",
	"false_positive", "cancelled", "other",
}

type OperationalCase struct {
	ID                uuid.UUID
	TenantID          uuid.UUID
	Reference         string
	Title             string
	Summary           string
	Status            string
	DerivedSeverity   string
	EffectiveSeverity string
	SeverityOverride  bool
	OwnerUserID       *uuid.UUID
	CreatedByUserID   uuid.UUID
	ResolutionCode    *string
	ResolutionSummary *string
	Version           int64
	LastActivityAt    time.Time
	CreatedAt         time.Time
	UpdatedAt         time.Time
	ResolvedAt        *time.Time
	ClosedAt          *time.Time
}

type CaseLink struct {
	ID             uuid.UUID
	TenantID       uuid.UUID
	CaseID         uuid.UUID
	EntityType     string
	EntityID       string
	LinkedAt       time.Time
	LinkedByUserID uuid.UUID
}

type CaseParticipant struct {
	CaseID        uuid.UUID
	TenantID      uuid.UUID
	UserID        uuid.UUID
	Role          string
	AddedAt       time.Time
	AddedByUserID uuid.UUID
}

type CaseNote struct {
	ID           uuid.UUID
	TenantID     uuid.UUID
	CaseID       uuid.UUID
	AuthorUserID uuid.UUID
	Body         string
	Visibility   string
	CreatedAt    time.Time
	UpdatedAt    time.Time
	EditedAt     *time.Time
	DeletedAt    *time.Time
	MentionedIDs []uuid.UUID
}

type CaseActionItem struct {
	ID              uuid.UUID
	TenantID        uuid.UUID
	CaseID          uuid.UUID
	Title           string
	Description     string
	Status          string
	AssigneeUserID  *uuid.UUID
	DueAt           *time.Time
	CreatedByUserID uuid.UUID
	CreatedAt       time.Time
	UpdatedAt       time.Time
	CompletedAt     *time.Time
}

type CaseDecision struct {
	ID              uuid.UUID
	TenantID        uuid.UUID
	CaseID          uuid.UUID
	Decision        string
	Rationale       string
	DecidedByUserID uuid.UUID
	DecidedAt       time.Time
}

type CaseEvent struct {
	ID          int64
	TenantID    uuid.UUID
	CaseID      uuid.UUID
	Source      string
	ActionType  string
	ActorUserID *uuid.UUID
	OccurredAt  time.Time
	Metadata    map[string]any
}

type CaseHealth struct {
	HasSLABreach             bool
	HasSLAWarning            bool
	NearestSLADueAt          *time.Time
	HighestExceptionPriority *string
	HighestRiskLevel         *string
	OpenActionCount          int
	OverdueActionCount       int
	NearestActionDueAt       *time.Time
	ActiveWorkItemCount      int
	ActiveExceptionCount     int
	ActiveRiskCount          int
}

type CaseListFilter struct {
	Status            string
	Severity          string
	OwnerUserID       *uuid.UUID
	ParticipantUserID *uuid.UUID
	ShipmentID        *uuid.UUID
	Search            string
	MyCases           bool
	Unassigned        bool
	HasOpenActions    bool
	HasSLABreach      bool
	HasSLAWarning     bool
	HasCriticalRisk   bool
	OverdueActions    bool
	IncludeClosed     bool
	Preset            string
	Page              int
	Limit             int
}

type CasePage struct {
	Items   []OperationalCase
	Page    int
	Limit   int
	Total   int
	HasNext bool
}

type CaseKPI struct {
	OpenCases               int
	MyOpenCases             int
	CriticalCases           int
	UnassignedCases         int
	CasesWithSLABreach      int
	CasesWithSLAWarning     int
	CasesWithOverdueActions int
	SlaAtRiskCases          int
	ResolvedCases           int
}

type CreateCaseInput struct {
	Title              string
	Summary            string
	Severity           string
	OwnerUserID        *uuid.UUID
	ShipmentIDs        []uuid.UUID
	WorkItems          []BulkActionItem
	ParticipantUserIDs []uuid.UUID
}

type ActiveCaseRef struct {
	CaseID    uuid.UUID
	Reference string
	Title     string
	Status    string
}
