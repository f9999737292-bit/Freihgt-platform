package domain

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
)

const (
	AutomationConditionSchemaVersion = 1
	MaxConditionsPerRule             = 25
	MaxConditionNesting              = 2
	MaxPlaybookSteps                 = 50
	MaxRuleNameLength                = 128
	MaxPlaybookNameLength            = 128
)

// Rule statuses.
const (
	RuleStatusDraft    = "draft"
	RuleStatusActive   = "active"
	RuleStatusDisabled = "disabled"
	RuleStatusRetired  = "retired"
)

// Execution modes.
const (
	ExecutionModeObserve     = "observe"
	ExecutionModeRecommend   = "recommend"
	ExecutionModeGuardedAuto = "guarded_auto"
)

// Playbook statuses.
const (
	PlaybookStatusDraft   = "draft"
	PlaybookStatusActive  = "active"
	PlaybookStatusRetired = "retired"
)

// Playbook step types.
const (
	StepTypeInstruction    = "instruction"
	StepTypeChecklist      = "checklist"
	StepTypeOperatorAction = "operator_action"
	StepTypeSystemAction   = "system_action"
)

// Recommendation statuses.
const (
	RecommendationStatusPending   = "pending"
	RecommendationStatusAccepted  = "accepted"
	RecommendationStatusDismissed = "dismissed"
	RecommendationStatusExpired   = "expired"
	RecommendationStatusCompleted = "completed"
)

// Execution statuses.
const (
	ExecutionStatusNotStarted = "not_started"
	ExecutionStatusInProgress = "in_progress"
	ExecutionStatusCompleted  = "completed"
	ExecutionStatusCancelled  = "cancelled"
)

// Execution step statuses.
const (
	ExecutionStepStatusPending    = "pending"
	ExecutionStepStatusInProgress = "in_progress"
	ExecutionStepStatusDone       = "done"
	ExecutionStepStatusSkipped    = "skipped"
)

// Dismiss reasons.
const (
	DismissReasonNotRelevant    = "not_relevant"
	DismissReasonAlreadyHandled = "already_handled"
	DismissReasonDuplicate      = "duplicate"
	DismissReasonFalsePositive  = "false_positive"
	DismissReasonOther          = "other"
)

// Actor types for audit.
const (
	ActorTypeUser   = "user"
	ActorTypeSystem = "system"
)

// Controlled trigger types.
var AllowedTriggerTypes = map[string]struct{}{
	"risk_created":                {},
	"risk_level_changed":          {},
	"exception_created":           {},
	"exception_priority_changed":  {},
	"sla_warning":                 {},
	"sla_breached":                {},
	"tracking_stale":              {},
	"tracking_lost":               {},
	"eta_at_risk":                 {},
	"eta_projected_late":          {},
	"slot_at_risk":                {},
	"slot_projected_miss":         {},
	"slot_actual_missed":          {},
	"work_item_created":           {},
	"work_item_unassigned":        {},
	"case_created":                {},
	"case_status_changed":         {},
}

type AutomationRule struct {
	ID                     uuid.UUID
	TenantID               uuid.UUID
	Name                   string
	Description            string
	Status                 string
	TriggerType            string
	Conditions             ConditionGroup
	ConditionSchemaVersion int
	PlaybookID             *uuid.UUID
	ExecutionMode          string
	Priority               int
	Version                int
	CreatedByUserID        uuid.UUID
	UpdatedByUserID        uuid.UUID
	CreatedAt              time.Time
	UpdatedAt              time.Time
}

type OperationalPlaybook struct {
	ID              uuid.UUID
	TenantID        uuid.UUID
	Name            string
	Description     string
	Status          string
	CurrentVersion  int
	CreatedByUserID uuid.UUID
	UpdatedByUserID uuid.UUID
	CreatedAt       time.Time
	UpdatedAt       time.Time
}

type PlaybookVersion struct {
	ID              uuid.UUID
	TenantID        uuid.UUID
	PlaybookID      uuid.UUID
	Version         int
	Status          string
	CreatedByUserID uuid.UUID
	CreatedAt       time.Time
	Steps           []PlaybookStep
}

type PlaybookStep struct {
	ID                       uuid.UUID
	TenantID                 uuid.UUID
	PlaybookVersionID        uuid.UUID
	Sequence                 int
	Title                    string
	Description              string
	StepType                 string
	Required                 bool
	EstimatedDurationMinutes *int
	ActionCode               string
}

type AutomationRecommendation struct {
	ID                 uuid.UUID
	TenantID           uuid.UUID
	RuleID             uuid.UUID
	RuleVersion        int
	PlaybookID         uuid.UUID
	PlaybookVersion    int
	PlaybookVersionID  uuid.UUID
	TriggerID          string
	TriggerType        string
	CorrelationID      string
	CausationID        string
	ShipmentID         *uuid.UUID
	WorkItemType       string
	WorkItemID         string
	CaseID             *uuid.UUID
	RiskID             string
	ExceptionID        string
	Status             string
	MatchExplanation   []MatchedCondition
	IdempotencyKey     string
	CreatedAt          time.Time
	ExpiresAt          *time.Time
	AcceptedByUserID   *uuid.UUID
	AcceptedAt         *time.Time
	DismissedByUserID  *uuid.UUID
	DismissedAt        *time.Time
	DismissReason      string
	CompletedAt        *time.Time
	PlaybookName       string
	RuleName           string
}

type PlaybookExecution struct {
	ID                uuid.UUID
	TenantID          uuid.UUID
	RecommendationID  *uuid.UUID
	PlaybookID        uuid.UUID
	PlaybookVersion   int
	PlaybookVersionID uuid.UUID
	ShipmentID        *uuid.UUID
	WorkItemType      string
	WorkItemID        string
	CaseID            *uuid.UUID
	OwnerUserID       uuid.UUID
	Status            string
	StartedAt         *time.Time
	CompletedAt       *time.Time
	CreatedByUserID   uuid.UUID
	CreatedAt         time.Time
	UpdatedAt         time.Time
	PlaybookName      string
	Steps             []PlaybookExecutionStep
}

type PlaybookExecutionStep struct {
	ID                uuid.UUID
	TenantID          uuid.UUID
	ExecutionID       uuid.UUID
	PlaybookStepID    uuid.UUID
	Sequence          int
	Title             string
	Description       string
	StepType          string
	Required          bool
	ActionCode        string
	Status            string
	SkipReason        string
	StartedAt         *time.Time
	CompletedAt       *time.Time
	StartedByUserID   *uuid.UUID
	CompletedByUserID *uuid.UUID
}

type AutomationTrigger struct {
	TriggerID             string
	TriggerType           string
	TenantID              uuid.UUID
	OccurredAt            time.Time
	ShipmentID            *uuid.UUID
	WorkItemType          string
	WorkItemID            string
	CaseID                *uuid.UUID
	RiskID                string
	ExceptionID           string
	Attributes            TriggerAttributes
	CorrelationID         string
	CausationID           string
	SourceOrigin          string
	AutomationExecutionID *uuid.UUID
}

type TriggerAttributes struct {
	ItemType               string `json:"itemType,omitempty"`
	WorkflowStatus         string `json:"workflowStatus,omitempty"`
	Priority               string `json:"priority,omitempty"`
	BusinessImpact         string `json:"businessImpact,omitempty"`
	ExceptionCategory      string `json:"exceptionCategory,omitempty"`
	RiskLevel              string `json:"riskLevel,omitempty"`
	RiskStatus             string `json:"riskStatus,omitempty"`
	PredictedExceptionType string `json:"predictedExceptionType,omitempty"`
	SLAStatus              string `json:"slaStatus,omitempty"`
	EscalationLevel        string `json:"escalationLevel,omitempty"`
	TrackingStatus         string `json:"trackingStatus,omitempty"`
	TrackingQuality        string `json:"trackingQuality,omitempty"`
	ETAStatus              string `json:"etaStatus,omitempty"`
	ArrivalProjection      string `json:"arrivalProjection,omitempty"`
	ProjectedDelaySeconds  *int64 `json:"projectedDelaySeconds,omitempty"`
	SlotType               string `json:"slotType,omitempty"`
	SlotArrivalProjection  string `json:"slotArrivalProjection,omitempty"`
	SlotProjectedLateSeconds *int64 `json:"slotProjectedLateSeconds,omitempty"`
	CaseStatus             string `json:"caseStatus,omitempty"`
	CaseSeverity           string `json:"caseSeverity,omitempty"`
	Assigned               *bool  `json:"assigned,omitempty"`
	HasActiveCase          *bool  `json:"hasActiveCase,omitempty"`
	StateVersion           string `json:"stateVersion,omitempty"`
}

type AutomationContext struct {
	Trigger    AutomationTrigger
	Attributes TriggerAttributes
}

type ConditionGroup struct {
	Logic      string            `json:"logic"`
	Conditions []ConditionClause `json:"conditions"`
	Groups     []ConditionGroup  `json:"groups,omitempty"`
}

type ConditionClause struct {
	Field    string          `json:"field"`
	Operator string          `json:"operator"`
	Value    json.RawMessage `json:"value,omitempty"`
}

type MatchedCondition struct {
	Field    string `json:"field"`
	Operator string `json:"operator"`
	Expected any    `json:"expected"`
	Actual   any    `json:"actual"`
	Matched  bool   `json:"matched"`
}

type RuleMatch struct {
	Rule               AutomationRule
	Matched            bool
	MatchedConditions  []MatchedCondition
	SelectedPlaybookID *uuid.UUID
	PlaybookVersion    int
	PlaybookVersionID  uuid.UUID
}

type CreateRuleInput struct {
	Name          string
	Description   string
	TriggerType   string
	Conditions    ConditionGroup
	PlaybookID    *uuid.UUID
	ExecutionMode string
	Priority      int
}

type UpdateRuleInput struct {
	Name          *string
	Description   *string
	TriggerType   *string
	Conditions    *ConditionGroup
	PlaybookID    *uuid.UUID
	ExecutionMode *string
	Priority      *int
}

type CreatePlaybookInput struct {
	Name        string
	Description string
	Steps       []PlaybookStepInput
}

type PlaybookStepInput struct {
	Sequence                 int
	Title                    string
	Description              string
	StepType                 string
	Required                 bool
	EstimatedDurationMinutes *int
	ActionCode               string
}

type AutomationKPI struct {
	PendingRecommendations     int
	ActivePlaybookExecutions   int
	CompletedPlaybooksToday    int
}

type Page[T any] struct {
	Items   []T
	Page    int
	Limit   int
	Total   int
	HasNext bool
}
