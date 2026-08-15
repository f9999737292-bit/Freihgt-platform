package domain

import (
	"strings"
	"time"

	"github.com/google/uuid"

	apperrors "github.com/freight-platform/control-tower-read-model-service/internal/platform/errors"
)

const (
	ActionSafetySafeInternal     = "SAFE_INTERNAL"
	ActionSafetyGuardedExternal  = "GUARDED_EXTERNAL"
	ActionSafetyApprovalRequired = "APPROVAL_REQUIRED"
	ActionSafetyForbidden        = "FORBIDDEN"

	GuardDecisionAllow           = "allow"
	GuardDecisionRequireApproval = "require_approval"
	GuardDecisionDeny            = "deny"
	GuardDecisionSkip            = "skip"

	ApprovalLevelNone       = "none"
	ApprovalLevelOperator   = "operator"
	ApprovalLevelSupervisor = "supervisor"

	ApprovalStatusPending  = "pending"
	ApprovalStatusApproved = "approved"
	ApprovalStatusRejected = "rejected"

	GuardedActionStatusPending          = "pending"
	GuardedActionStatusWaitingApproval  = "waiting_approval"
	GuardedActionStatusRunning          = "running"
	GuardedActionStatusWaitingResponse  = "waiting_response"
	GuardedActionStatusSucceeded        = "succeeded"
	GuardedActionStatusFailed           = "failed"
	GuardedActionStatusDenied           = "denied"
	GuardedActionStatusRejected         = "rejected"
	GuardedActionStatusTimedOut         = "timed_out"
	GuardedActionStatusSkipped          = "skipped"

	ExecutionStepStatusWaitingApproval = "waiting_approval"
	ExecutionStepStatusWaitingResponse = "waiting_response"
	ExecutionStepStatusDenied          = "denied"
	ExecutionStepStatusFailed          = "failed"
	ExecutionStepStatusRejected        = "rejected"
	ExecutionStepStatusTimedOut        = "timed_out"

	ActionRequestDriverDelayReason         = "REQUEST_DRIVER_DELAY_REASON"
	ActionRequestDriverStatusConfirmation  = "REQUEST_DRIVER_STATUS_CONFIRMATION"
	ActionRequestDriverArrivalConfirmation = "REQUEST_DRIVER_ARRIVAL_CONFIRMATION"
	ActionCreateDriverOperationalNotice    = "CREATE_DRIVER_OPERATIONAL_NOTICE"
	ActionEscalateDriverTaskTimeout        = "ESCALATE_DRIVER_TASK_TIMEOUT"

	DriverTaskTypeRequestDelayReason         = "REQUEST_DELAY_REASON"
	DriverTaskTypeRequestStatusConfirmation  = "REQUEST_STATUS_CONFIRMATION"
	DriverTaskTypeRequestArrivalConfirmation = "REQUEST_ARRIVAL_CONFIRMATION"
	DriverTaskTypeGeneralOperationalNotice   = "GENERAL_OPERATIONAL_NOTICE"

	DriverTaskSourceControlTower = "CONTROL_TOWER"

	DefaultDriverTaskExpiryMinutes = 120
	MinDriverTaskExpiryMinutes     = 5
	MaxDriverTaskExpiryMinutes     = 1440
	MaxAutomationCausationDepth    = 5

	ActionMaxAttempts = 3
	ActionBackoffBase = time.Second
)

var forbiddenAutomationActions = map[string]struct{}{
	"CHANGE_ROUTE": {}, "CHANGE_CARRIER": {}, "CHANGE_DRIVER_ASSIGNMENT": {},
	"CANCEL_SHIPMENT": {}, "CANCEL_TRANSPORT_ORDER": {}, "REBOOK_SLOT": {}, "CANCEL_SLOT": {},
	"CHANGE_RATE": {}, "ACCEPT_RATE": {}, "AUTHORIZE_PAYMENT": {}, "RELEASE_PAYMENT": {},
	"MODIFY_CONTRACT": {}, "SIGN_DOCUMENT": {}, "DELETE_DOCUMENT": {},
	"ARBITRARY_DRIVER_COMMAND": {}, "ARBITRARY_PUSH": {}, "ARBITRARY_URL": {},
	"ARBITRARY_DEEP_LINK": {}, "ARBITRARY_WEBHOOK": {}, "ARBITRARY_HTTP_REQUEST": {},
	"ARBITRARY_SCRIPT": {}, "ARBITRARY_SQL": {}, "CREATE_DRIVER_TASK": {},
}

type GuardedActionSpec struct {
	SafetyClass      string
	ApprovalLevel    string
	DriverTaskType   string
	RequiresResponse bool
	RequiresShipment bool
	RequiresDriver   bool
}

var guardedActionRegistry = map[string]GuardedActionSpec{
	ActionRequestDriverDelayReason: {
		SafetyClass: ActionSafetyGuardedExternal, ApprovalLevel: ApprovalLevelNone,
		DriverTaskType: DriverTaskTypeRequestDelayReason, RequiresResponse: true, RequiresShipment: true, RequiresDriver: true,
	},
	ActionRequestDriverStatusConfirmation: {
		SafetyClass: ActionSafetyGuardedExternal, ApprovalLevel: ApprovalLevelNone,
		DriverTaskType: DriverTaskTypeRequestStatusConfirmation, RequiresResponse: true, RequiresShipment: true, RequiresDriver: true,
	},
	ActionRequestDriverArrivalConfirmation: {
		SafetyClass: ActionSafetyGuardedExternal, ApprovalLevel: ApprovalLevelNone,
		DriverTaskType: DriverTaskTypeRequestArrivalConfirmation, RequiresResponse: true, RequiresShipment: true, RequiresDriver: true,
	},
	ActionCreateDriverOperationalNotice: {
		SafetyClass: ActionSafetyGuardedExternal, ApprovalLevel: ApprovalLevelOperator,
		DriverTaskType: DriverTaskTypeGeneralOperationalNotice, RequiresResponse: false, RequiresShipment: false, RequiresDriver: true,
	},
	ActionEscalateDriverTaskTimeout: {
		SafetyClass: ActionSafetySafeInternal, ApprovalLevel: ApprovalLevelNone,
		RequiresResponse: false, RequiresShipment: false, RequiresDriver: false,
	},
}

type GuardedAction struct {
	ID              uuid.UUID
	TenantID        uuid.UUID
	ExecutionID     uuid.UUID
	ExecutionStepID uuid.UUID
	ActionType      string
	SafetyClass     string
	GuardDecision   string
	GuardReason     string
	Status          string
	DriverID        *uuid.UUID
	ShipmentID      *uuid.UUID
	DriverTaskID    *uuid.UUID
	CorrelationID   string
	SourceEventID   string
	IdempotencyKey  string
	ResponsePayload []byte
	ErrorReason     string
	ExpiresAt       *time.Time
	DispatchedAt    *time.Time
	CompletedAt     *time.Time
	CreatedAt       time.Time
	UpdatedAt       time.Time
	Version         int
}

type ActionApproval struct {
	ID              uuid.UUID
	TenantID        uuid.UUID
	GuardedActionID uuid.UUID
	RequiredLevel   string
	Status          string
	RequestedAt     time.Time
	ApprovedAt      *time.Time
	ApprovedBy      *uuid.UUID
	RejectedAt      *time.Time
	RejectedBy      *uuid.UUID
	Reason          string
	Version         int
}

type TenantActionPolicy struct {
	TenantID        uuid.UUID
	ActionType      string
	Enabled         bool
	ApprovalLevel   *string
	PriorityCeiling *string
}

func NormalizeGuardedActionType(code string) string {
	return strings.ToUpper(strings.TrimSpace(code))
}

func LookupGuardedActionSpec(actionType string) (GuardedActionSpec, bool) {
	spec, ok := guardedActionRegistry[NormalizeGuardedActionType(actionType)]
	return spec, ok
}

func IsForbiddenAutomationAction(actionType string) bool {
	_, ok := forbiddenAutomationActions[NormalizeGuardedActionType(actionType)]
	return ok
}

func ValidateGuardedActionCode(code string) error {
	code = NormalizeGuardedActionCode(code)
	if code == "" {
		return apperrors.Validation("action code is required for system_action steps", map[string]any{"field": "actionCode"})
	}
	if IsForbiddenAutomationAction(code) {
		return apperrors.Validation("forbidden automation action", map[string]any{"actionCode": code})
	}
	if _, ok := guardedActionRegistry[code]; !ok {
		return apperrors.Validation("unknown guarded action type", map[string]any{"actionCode": code})
	}
	return nil
}

func NormalizeGuardedActionCode(code string) string {
	switch strings.ToLower(strings.TrimSpace(code)) {
	case "request_driver_delay_reason":
		return ActionRequestDriverDelayReason
	case "request_driver_status_confirmation":
		return ActionRequestDriverStatusConfirmation
	case "request_driver_arrival_confirmation":
		return ActionRequestDriverArrivalConfirmation
	case "create_driver_operational_notice":
		return ActionCreateDriverOperationalNotice
	default:
		return NormalizeGuardedActionType(code)
	}
}

func MapToDriverTaskType(actionType string) (string, error) {
	spec, ok := guardedActionRegistry[NormalizeGuardedActionType(actionType)]
	if !ok || spec.DriverTaskType == "" {
		return "", apperrors.Validation("action cannot map to driver task", map[string]any{"actionType": actionType})
	}
	return spec.DriverTaskType, nil
}

func IsTerminalGuardedActionStatus(status string) bool {
	switch strings.TrimSpace(status) {
	case GuardedActionStatusSucceeded, GuardedActionStatusFailed, GuardedActionStatusDenied,
		GuardedActionStatusRejected, GuardedActionStatusTimedOut, GuardedActionStatusSkipped:
		return true
	default:
		return false
	}
}

func EffectiveApprovalLevel(actionType string, tenantPolicy *TenantActionPolicy) string {
	spec, ok := guardedActionRegistry[NormalizeGuardedActionType(actionType)]
	if !ok {
		return ApprovalLevelSupervisor
	}
	level := spec.ApprovalLevel
	if tenantPolicy != nil && tenantPolicy.ApprovalLevel != nil {
		if moreRestrictive(*tenantPolicy.ApprovalLevel, level) {
			level = *tenantPolicy.ApprovalLevel
		}
	}
	return level
}

func moreRestrictive(candidate, baseline string) bool {
	rank := map[string]int{ApprovalLevelNone: 0, ApprovalLevelOperator: 1, ApprovalLevelSupervisor: 2}
	return rank[candidate] > rank[baseline]
}
