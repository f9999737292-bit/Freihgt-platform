package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/freight-platform/control-tower-read-model-service/internal/client"
	"github.com/freight-platform/control-tower-read-model-service/internal/domain"
	apperrors "github.com/freight-platform/control-tower-read-model-service/internal/platform/errors"
	"github.com/freight-platform/control-tower-read-model-service/internal/repository"
)

type GuardedActionService struct {
	actions    *repository.GuardedActionRepository
	shipments  *repository.ShipmentLookupRepository
	automation *repository.AutomationRepository
	guard      *GuardEvaluator
	driverTask client.DriverTaskClient
}

func NewGuardedActionService(
	actions *repository.GuardedActionRepository,
	shipments *repository.ShipmentLookupRepository,
	automation *repository.AutomationRepository,
	guard *GuardEvaluator,
	driverTask client.DriverTaskClient,
) *GuardedActionService {
	return &GuardedActionService{
		actions: actions, shipments: shipments, automation: automation, guard: guard, driverTask: driverTask,
	}
}

type DispatchGuardedStepInput struct {
	TenantID    uuid.UUID
	Execution   domain.PlaybookExecution
	Step        domain.PlaybookExecutionStep
	Trigger     domain.AutomationTrigger
	ActorUserID *uuid.UUID
}

func (s *GuardedActionService) DispatchSystemStep(ctx context.Context, in DispatchGuardedStepInput) (domain.GuardedAction, error) {
	actionType := domain.NormalizeGuardedActionCode(in.Step.ActionCode)
	spec, hasSpec := domain.LookupGuardedActionSpec(actionType)
	safetyClass := domain.ActionSafetyForbidden
	if hasSpec {
		safetyClass = spec.SafetyClass
	}
	var driverID *uuid.UUID
	shipmentID := in.Execution.ShipmentID
	if shipmentID == nil {
		shipmentID = in.Trigger.ShipmentID
	}
	if shipmentID != nil {
		assignment, err := s.shipments.GetAssignedDriver(ctx, in.TenantID, *shipmentID)
		if err != nil {
			if hasSpec && (spec.RequiresShipment || spec.RequiresDriver) {
				return s.persistDenied(ctx, in, actionType, safetyClass, "SHIPMENT_LOOKUP_FAILED")
			}
		} else {
			driverID = assignment.DriverID
		}
	}
	guardResult, err := s.guard.EvaluateAction(ctx, GuardEvaluationInput{
		TenantID: in.TenantID, Trigger: in.Trigger, ActionType: actionType,
		ShipmentID: shipmentID, DriverID: driverID,
	})
	if err != nil {
		return domain.GuardedAction{}, err
	}
	idempotencyKey := buildGuardedActionIdempotencyKey(in.TenantID, in.Execution.ID, in.Step.ID, in.Trigger.TriggerID)
	expiresAt := time.Now().UTC().Add(time.Duration(domain.DefaultDriverTaskExpiryMinutes) * time.Minute)
	action, inserted, err := s.actions.CreateAction(ctx, repository.CreateGuardedActionParams{
		TenantID: in.TenantID, ExecutionID: in.Execution.ID, ExecutionStepID: in.Step.ID,
		ActionType: actionType, SafetyClass: safetyClass, GuardDecision: guardResult.Decision,
		GuardReason: guardResult.Reason, Status: initialGuardedActionStatus(guardResult.Decision),
		DriverID: driverID, ShipmentID: shipmentID, CorrelationID: in.Trigger.CorrelationID,
		SourceEventID: in.Trigger.TriggerID, IdempotencyKey: idempotencyKey, ExpiresAt: &expiresAt,
	})
	if err != nil {
		return domain.GuardedAction{}, err
	}
	if !inserted {
		if action.DriverTaskID != nil || domain.IsTerminalGuardedActionStatus(action.Status) {
			return action, nil
		}
	}
	stepSpec := spec
	if !hasSpec {
		stepSpec = domain.GuardedActionSpec{}
	}
	_ = s.actions.UpdateExecutionStepStatus(ctx, in.TenantID, in.Step.ID, stepStatusForGuard(guardResult.Decision, stepSpec))
	switch guardResult.Decision {
	case domain.GuardDecisionDeny, domain.GuardDecisionSkip:
		return action, nil
	case domain.GuardDecisionRequireApproval:
		if !hasSpec {
			return action, nil
		}
		_, _, _ = s.actions.CreateApproval(ctx, in.TenantID, action.ID, domain.EffectiveApprovalLevel(actionType, nil))
		return action, nil
	case domain.GuardDecisionAllow:
		if !hasSpec {
			return s.persistDenied(ctx, in, actionType, safetyClass, "UNKNOWN_ACTION")
		}
		return s.executeDriverTaskAction(ctx, in, action, spec, driverID, shipmentID)
	default:
		return action, nil
	}
}

func (s *GuardedActionService) ApproveAction(ctx context.Context, tenantID, userID, actionID uuid.UUID) (domain.GuardedAction, error) {
	if !hasAutomationPermission(ctx, "automation.approve") {
		return domain.GuardedAction{}, apperrors.Forbidden("automation approval permission required")
	}
	action, err := s.actions.GetAction(ctx, tenantID, actionID)
	if err != nil {
		return domain.GuardedAction{}, err
	}
	if action.Status != domain.GuardedActionStatusWaitingApproval {
		if action.DriverTaskID != nil || domain.IsTerminalGuardedActionStatus(action.Status) {
			return action, nil
		}
		return domain.GuardedAction{}, apperrors.Conflict("action is not waiting for approval", nil)
	}
	if _, err := s.actions.Approve(ctx, tenantID, actionID, userID); err != nil {
		var appErr *apperrors.AppError
		if errors.As(err, &appErr) && appErr.Code == apperrors.CodeConflict {
			refreshed, getErr := s.actions.GetAction(ctx, tenantID, actionID)
			if getErr == nil && (refreshed.DriverTaskID != nil || refreshed.Status != domain.GuardedActionStatusWaitingApproval) {
				return refreshed, nil
			}
		}
		return domain.GuardedAction{}, err
	}
	spec, _ := domain.LookupGuardedActionSpec(action.ActionType)
	exec, err := s.automation.GetExecution(ctx, tenantID, action.ExecutionID)
	if err != nil {
		return domain.GuardedAction{}, err
	}
	var step domain.PlaybookExecutionStep
	for _, st := range exec.Steps {
		if st.ID == action.ExecutionStepID {
			step = st
			break
		}
	}
	trigger := domain.AutomationTrigger{
		TriggerID: action.SourceEventID, CorrelationID: action.CorrelationID, ShipmentID: action.ShipmentID,
	}
	return s.executeDriverTaskAction(ctx, DispatchGuardedStepInput{
		TenantID: tenantID, Execution: exec, Step: step, Trigger: trigger, ActorUserID: &userID,
	}, action, spec, action.DriverID, action.ShipmentID)
}

func (s *GuardedActionService) RejectAction(ctx context.Context, tenantID, userID, actionID uuid.UUID, reason string) (domain.GuardedAction, error) {
	if !hasAutomationPermission(ctx, "automation.approve") {
		return domain.GuardedAction{}, apperrors.Forbidden("automation approval permission required")
	}
	if _, err := s.actions.RejectApproval(ctx, tenantID, actionID, userID, reason); err != nil {
		return domain.GuardedAction{}, err
	}
	action, err := s.actions.TransitionAction(ctx, tenantID, actionID,
		[]string{domain.GuardedActionStatusWaitingApproval}, domain.GuardedActionStatusRejected, reason, nil)
	if err != nil {
		return domain.GuardedAction{}, err
	}
	_ = s.actions.UpdateExecutionStepStatus(ctx, tenantID, action.ExecutionStepID, domain.ExecutionStepStatusRejected)
	return action, nil
}

type DriverTaskEventInput struct {
	TenantID      uuid.UUID
	EventType     string
	TaskID        uuid.UUID
	SourceEventID string
	CorrelationID string
	Payload       json.RawMessage
}

func (s *GuardedActionService) HandleDriverTaskEvent(ctx context.Context, in DriverTaskEventInput) error {
	action, err := s.actions.GetActionByDriverTask(ctx, in.TenantID, in.TaskID)
	if err != nil {
		return nil
	}
	switch strings.TrimSpace(in.EventType) {
	case "driver.task_completed":
		return s.handleTaskCompleted(ctx, action, in.Payload)
	case "driver.task_expired":
		return s.handleTaskExpired(ctx, action)
	case "driver.task_cancelled":
		_, err := s.actions.TransitionAction(ctx, in.TenantID, action.ID,
			[]string{domain.GuardedActionStatusWaitingResponse}, domain.GuardedActionStatusFailed, "TASK_CANCELLED", nil)
		if err == nil {
			_ = s.actions.UpdateExecutionStepStatus(ctx, in.TenantID, action.ExecutionStepID, domain.ExecutionStepStatusFailed)
		}
		return err
	default:
		return nil
	}
}

func (s *GuardedActionService) executeDriverTaskAction(ctx context.Context, in DispatchGuardedStepInput, action domain.GuardedAction, spec domain.GuardedActionSpec, driverID *uuid.UUID, shipmentID *uuid.UUID) (domain.GuardedAction, error) {
	if s.driverTask == nil {
		return s.failAction(ctx, in.TenantID, action, "DRIVER_TASK_CLIENT_UNAVAILABLE")
	}
	if shipmentID != nil {
		assignment, err := s.shipments.GetAssignedDriver(ctx, in.TenantID, *shipmentID)
		if err != nil || assignment.DriverID == nil {
			if spec.RequiresShipment || spec.RequiresDriver {
				return s.failAction(ctx, in.TenantID, action, "NO_ASSIGNED_DRIVER")
			}
		} else {
			driverID = assignment.DriverID
		}
	}
	if driverID == nil {
		return s.failAction(ctx, in.TenantID, action, "MISSING_DRIVER")
	}
	taskType, err := domain.MapToDriverTaskType(action.ActionType)
	if err != nil {
		return s.failAction(ctx, in.TenantID, action, "INVALID_TASK_MAPPING")
	}
	expiresAt := action.ExpiresAt
	if expiresAt == nil {
		t := time.Now().UTC().Add(time.Duration(domain.DefaultDriverTaskExpiryMinutes) * time.Minute)
		expiresAt = &t
	}
	var lastErr error
	for attempt := 0; attempt < domain.ActionMaxAttempts; attempt++ {
		resp, err := s.driverTask.CreateTask(ctx, client.CreateDriverTaskRequest{
			TenantID: in.TenantID, DriverID: *driverID, ShipmentID: shipmentID, TaskType: taskType,
			Priority: "NORMAL", ExpiresAt: expiresAt, Source: domain.DriverTaskSourceControlTower,
			SourceEventID: action.SourceEventID, CorrelationID: action.CorrelationID,
			IdempotencyKey: action.IdempotencyKey, CreatedByType: "CONTROL_TOWER", CreatedByID: in.ActorUserID,
		})
		if err != nil {
			lastErr = err
			time.Sleep(domain.ActionBackoffBase * time.Duration(attempt+1))
			continue
		}
		status := domain.GuardedActionStatusWaitingResponse
		if !spec.RequiresResponse {
			status = domain.GuardedActionStatusSucceeded
		}
		updated, err := s.actions.MarkDriverTaskDispatched(ctx, in.TenantID, action.ID, resp.TaskID, status)
		if err != nil {
			return domain.GuardedAction{}, err
		}
		stepStatus := domain.ExecutionStepStatusWaitingResponse
		if !spec.RequiresResponse {
			stepStatus = domain.ExecutionStepStatusDone
		}
		_ = s.actions.UpdateExecutionStepStatus(ctx, in.TenantID, in.Step.ID, stepStatus)
		if !spec.RequiresResponse {
			_ = s.automation.CompleteExecutionIfReady(ctx, in.TenantID, in.Execution.ID)
		}
		return updated, nil
	}
	return s.failAction(ctx, in.TenantID, action, fmt.Sprintf("DRIVER_TASK_DISPATCH_FAILED: %v", lastErr))
}

func (s *GuardedActionService) handleTaskCompleted(ctx context.Context, action domain.GuardedAction, payload json.RawMessage) error {
	if domain.IsTerminalGuardedActionStatus(action.Status) {
		return nil
	}
	updated, err := s.actions.TransitionAction(ctx, action.TenantID, action.ID,
		[]string{domain.GuardedActionStatusWaitingResponse}, domain.GuardedActionStatusSucceeded, "", payload)
	if err != nil {
		return nil
	}
	_ = s.actions.UpdateExecutionStepStatus(ctx, action.TenantID, action.ExecutionStepID, domain.ExecutionStepStatusDone)
	exec, _ := s.automation.GetExecution(ctx, action.TenantID, action.ExecutionID)
	if exec.CaseID != nil {
		meta := map[string]any{"actionType": action.ActionType, "driverTaskId": updated.DriverTaskID, "response": json.RawMessage(payload)}
		_ = s.actions.InsertCaseTimelineEvent(ctx, action.TenantID, *exec.CaseID, "driver_task_response", meta)
	}
	return s.automation.CompleteExecutionIfReady(ctx, action.TenantID, action.ExecutionID)
}

func (s *GuardedActionService) handleTaskExpired(ctx context.Context, action domain.GuardedAction) error {
	if domain.IsTerminalGuardedActionStatus(action.Status) {
		return s.ensureTimeoutEscalation(ctx, action)
	}
	_, err := s.actions.TransitionAction(ctx, action.TenantID, action.ID,
		[]string{domain.GuardedActionStatusWaitingResponse}, domain.GuardedActionStatusTimedOut, "TASK_EXPIRED", nil)
	if err != nil {
		return s.ensureTimeoutEscalation(ctx, action)
	}
	_ = s.actions.UpdateExecutionStepStatus(ctx, action.TenantID, action.ExecutionStepID, domain.ExecutionStepStatusTimedOut)
	return s.ensureTimeoutEscalation(ctx, action)
}

func (s *GuardedActionService) ensureTimeoutEscalation(ctx context.Context, action domain.GuardedAction) error {
	exec, err := s.automation.GetExecution(ctx, action.TenantID, action.ExecutionID)
	if err != nil {
		return err
	}
	key := fmt.Sprintf("timeout-escalation:%s", action.ID.String())
	_, inserted, err := s.actions.CreateTimeoutEscalation(ctx, action.TenantID, action.ID, exec.CaseID, key)
	if err != nil || !inserted {
		return err
	}
	if exec.CaseID != nil {
		meta := map[string]any{
			"actionType": action.ActionType, "driverTaskId": action.DriverTaskID, "reason": "DRIVER_TASK_TIMEOUT",
		}
		return s.actions.InsertCaseTimelineEvent(ctx, action.TenantID, *exec.CaseID, "driver_task_timeout_escalation", meta)
	}
	return nil
}

func (s *GuardedActionService) persistDenied(ctx context.Context, in DispatchGuardedStepInput, actionType, safetyClass, reason string) (domain.GuardedAction, error) {
	idempotencyKey := buildGuardedActionIdempotencyKey(in.TenantID, in.Execution.ID, in.Step.ID, in.Trigger.TriggerID)
	action, _, err := s.actions.CreateAction(ctx, repository.CreateGuardedActionParams{
		TenantID: in.TenantID, ExecutionID: in.Execution.ID, ExecutionStepID: in.Step.ID,
		ActionType: actionType, SafetyClass: safetyClass, GuardDecision: domain.GuardDecisionDeny,
		GuardReason: reason, Status: domain.GuardedActionStatusDenied,
		ShipmentID: in.Execution.ShipmentID, CorrelationID: in.Trigger.CorrelationID,
		SourceEventID: in.Trigger.TriggerID, IdempotencyKey: idempotencyKey,
	})
	if err != nil {
		return domain.GuardedAction{}, err
	}
	_ = s.actions.UpdateExecutionStepStatus(ctx, in.TenantID, in.Step.ID, domain.ExecutionStepStatusDenied)
	return action, nil
}

func (s *GuardedActionService) failAction(ctx context.Context, tenantID uuid.UUID, action domain.GuardedAction, reason string) (domain.GuardedAction, error) {
	updated, err := s.actions.TransitionAction(ctx, tenantID, action.ID,
		[]string{domain.GuardedActionStatusPending, domain.GuardedActionStatusRunning, domain.GuardedActionStatusWaitingApproval},
		domain.GuardedActionStatusFailed, reason, nil)
	if err != nil {
		return domain.GuardedAction{}, err
	}
	_ = s.actions.UpdateExecutionStepStatus(ctx, tenantID, action.ExecutionStepID, domain.ExecutionStepStatusFailed)
	return updated, nil
}

func buildGuardedActionIdempotencyKey(tenantID, executionID, stepID uuid.UUID, triggerID string) string {
	return fmt.Sprintf("%s|%s|%s|%s", tenantID, executionID, stepID, strings.TrimSpace(triggerID))
}

func initialGuardedActionStatus(decision string) string {
	switch decision {
	case domain.GuardDecisionRequireApproval:
		return domain.GuardedActionStatusWaitingApproval
	case domain.GuardDecisionDeny, domain.GuardDecisionSkip:
		return domain.GuardedActionStatusDenied
	default:
		return domain.GuardedActionStatusPending
	}
}

func stepStatusForGuard(decision string, spec domain.GuardedActionSpec) string {
	switch decision {
	case domain.GuardDecisionRequireApproval:
		return domain.ExecutionStepStatusWaitingApproval
	case domain.GuardDecisionDeny, domain.GuardDecisionSkip:
		return domain.ExecutionStepStatusDenied
	default:
		return domain.ExecutionStepStatusInProgress
	}
}

type permissionContextKey struct{}

func WithAutomationPermissions(ctx context.Context, perms ...string) context.Context {
	return context.WithValue(ctx, permissionContextKey{}, perms)
}

func hasAutomationPermission(ctx context.Context, perm string) bool {
	raw, ok := ctx.Value(permissionContextKey{}).([]string)
	if !ok || len(raw) == 0 {
		return false
	}
	for _, p := range raw {
		if strings.TrimSpace(p) == perm || strings.TrimSpace(p) == "automation.manage" {
			return true
		}
	}
	return false
}
