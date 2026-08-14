package domain

import (
	"time"

	"github.com/google/uuid"
)

const (
	WorkItemTypeException = "exception"
	WorkItemTypeRisk      = "risk"

	UrgencyCritical = "critical"
	UrgencyHigh     = "high"
	UrgencyNormal   = "normal"
	UrgencyLow      = "low"

	ViewScopePrivate = "private"
	ViewScopeShared  = "shared"

	FilterSchemaVersion = 1

	BulkActionMaxBatch    = 100
	WorkItemsMaxPageLimit = 100

	ActionClaimed    = "claimed"
	ActionUnassigned = "unassigned"

	ActionRiskClaimed    = "risk_claimed"
	ActionRiskAssigned   = "risk_assigned"
	ActionRiskReassigned = "risk_reassigned"
	ActionRiskUnassigned = "risk_unassigned"

	HandoffOutcomeTransferred = "transferred"
	HandoffOutcomeFailed      = "failed"
)

type WorkItem struct {
	ID                 string
	ItemType           string
	SourceID           string
	ShipmentID         uuid.UUID
	TenantID           uuid.UUID
	Title              string
	Summary            string
	WorkflowStatus     string
	Priority           *string
	BusinessImpact     *string
	ExceptionCategory  *string
	SLAStatus          *string
	SLAPhase           *string
	SLADueAt           *time.Time
	RiskLevel          *string
	RiskScore          *int
	RiskStatus         *string
	PredictedType      *string
	EscalationLevel    *string
	Urgency            string
	OwnerUserID        *uuid.UUID
	CreatedAt          time.Time
	UpdatedAt          time.Time
	ThreatenedDeadline *time.Time
	AvailableActions   []string
	LinkedRiskKey      *string
	LinkedEventID      *string
	EventType          *string
}

type WorkItemFilter struct {
	ItemType          string
	WorkflowStatus    string
	Priority          string
	BusinessImpact    string
	SLAStatus         string
	EscalationLevel   string
	RiskLevel         string
	RiskStatus        string
	PredictedType     string
	ExceptionCategory string
	OwnerUserID       *uuid.UUID
	UnassignedOnly    bool
	MyWorkOnly        bool
	IncludeCompleted  bool
	Search            string
	Preset            string
	Page              int
	Limit             int
}

type WorkItemPage struct {
	Items   []WorkItem
	Page    int
	Limit   int
	Total   int
	HasNext bool
}

type SavedView struct {
	ID                  uuid.UUID
	TenantID            uuid.UUID
	OwnerUserID         uuid.UUID
	Name                string
	Scope               string
	FilterSchemaVersion int
	Filters             map[string]any
	Sort                map[string]any
	IsDefault           bool
	CreatedAt           time.Time
	UpdatedAt           time.Time
}

type ShiftHandoff struct {
	ID         uuid.UUID
	TenantID   uuid.UUID
	FromUserID uuid.UUID
	ToUserID   *uuid.UUID
	Title      *string
	Note       *string
	CreatedAt  time.Time
	Items      []ShiftHandoffItem
}

type ShiftHandoffItem struct {
	ID         uuid.UUID
	ItemType   string
	SourceID   string
	ShipmentID *uuid.UUID
	Outcome    string
	ErrorCode  *string
}

type BulkActionItem struct {
	ItemType string
	ItemID   string
}

type BulkActionResult struct {
	ItemType string
	ItemID   string
	Success  bool
	Error    *string
}

type BulkActionOutcome struct {
	Requested int
	Succeeded int
	Failed    int
	Results   []BulkActionResult
}

type WorkloadSummary struct {
	UserID         uuid.UUID
	ActiveWork     int
	Unacknowledged int
	P1             int
	P2             int
	SLABreached    int
	SLAWarning     int
	CriticalRisks  int
	HighRisks      int
}

type WorkspaceKPI struct {
	MyActiveWork     int
	MyCriticalWork   int
	UnassignedWork   int
	TeamActiveWork   int
	SLABreachedWork  int
	SLAWarningWork   int
	CriticalRiskWork int
}

type WorkItemTimelineEntry struct {
	Source      string
	ActionType  string
	ActorUserID *uuid.UUID
	OccurredAt  time.Time
	Metadata    map[string]any
}

func WorkItemKey(itemType, sourceID string) string {
	return itemType + ":" + sourceID
}
