package controltower

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/freight-platform/api-gateway/internal/controltowerreadmodel"
	apperrors "github.com/freight-platform/api-gateway/internal/platform/errors"
)

type assignRequestPayload struct {
	UserID string `json:"userId"`
}

type resolveRequestPayload struct {
	ResolutionCode string  `json:"resolutionCode"`
	Comment        *string `json:"comment,omitempty"`
}

func (s *Service) AssignCriticalEvent(
	ctx context.Context,
	reqCtx RequestContext,
	eventID string,
	rawBody []byte,
) (ControlTowerEventWorkflow, error) {
	event, err := s.validateCriticalEventMutation(ctx, reqCtx, eventID, rawBody, true)
	if err != nil {
		return ControlTowerEventWorkflow{}, err
	}
	_ = event

	var payload assignRequestPayload
	if err := json.Unmarshal(rawBody, &payload); err != nil {
		return ControlTowerEventWorkflow{}, apperrors.Validation("invalid request body", map[string]any{"field": "body"})
	}
	assignee := strings.TrimSpace(payload.UserID)
	if assignee == "" {
		return ControlTowerEventWorkflow{}, apperrors.Validation("userId is required", map[string]any{"field": "userId"})
	}

	rmCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), s.readModelCfg.Timeout)
	defer cancel()

	remote, depErr := s.readModel.AssignCriticalEvent(rmCtx, controltowerreadmodel.AssignCriticalEventInput{
		TenantID:       reqCtx.TenantID,
		UserID:         reqCtx.UserID,
		RequestID:      reqCtx.RequestID,
		EventID:        eventID,
		AssignedToUser: assignee,
	})
	if depErr != nil {
		return ControlTowerEventWorkflow{}, mapWorkflowDependencyError(depErr)
	}

	return mapRemoteWorkflow(*remote), nil
}

func (s *Service) ResolveCriticalEvent(
	ctx context.Context,
	reqCtx RequestContext,
	eventID string,
	rawBody []byte,
) (ControlTowerEventWorkflow, error) {
	event, err := s.validateCriticalEventMutation(ctx, reqCtx, eventID, rawBody, true)
	if err != nil {
		return ControlTowerEventWorkflow{}, err
	}
	_ = event

	var payload resolveRequestPayload
	if err := json.Unmarshal(rawBody, &payload); err != nil {
		return ControlTowerEventWorkflow{}, apperrors.Validation("invalid request body", map[string]any{"field": "body"})
	}
	code := strings.TrimSpace(payload.ResolutionCode)
	if code == "" {
		return ControlTowerEventWorkflow{}, apperrors.Validation("resolutionCode is required", map[string]any{"field": "resolutionCode"})
	}

	rmCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), s.readModelCfg.Timeout)
	defer cancel()

	remote, depErr := s.readModel.ResolveCriticalEvent(rmCtx, controltowerreadmodel.ResolveCriticalEventInput{
		TenantID:          reqCtx.TenantID,
		UserID:            reqCtx.UserID,
		RequestID:         reqCtx.RequestID,
		EventID:           eventID,
		ResolutionCode:    code,
		ResolutionComment: payload.Comment,
	})
	if depErr != nil {
		return ControlTowerEventWorkflow{}, mapWorkflowDependencyError(depErr)
	}

	return mapRemoteWorkflow(*remote), nil
}

func (s *Service) ReopenCriticalEvent(
	ctx context.Context,
	reqCtx RequestContext,
	eventID string,
	rawBody []byte,
) (ControlTowerEventWorkflow, error) {
	_, err := s.validateCriticalEventMutation(ctx, reqCtx, eventID, rawBody, false)
	if err != nil {
		return ControlTowerEventWorkflow{}, err
	}

	rmCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), s.readModelCfg.Timeout)
	defer cancel()

	remote, depErr := s.readModel.ReopenCriticalEvent(rmCtx, controltowerreadmodel.ReopenCriticalEventInput{
		TenantID:  reqCtx.TenantID,
		UserID:    reqCtx.UserID,
		RequestID: reqCtx.RequestID,
		EventID:   eventID,
	})
	if depErr != nil {
		return ControlTowerEventWorkflow{}, mapWorkflowDependencyError(depErr)
	}

	return mapRemoteWorkflow(*remote), nil
}

func (s *Service) GetCriticalEventActions(
	ctx context.Context,
	reqCtx RequestContext,
	eventID string,
) (ControlTowerEventActionsResponse, error) {
	if !ValidateCriticalEventID(strings.ToLower(strings.TrimSpace(eventID))) {
		return ControlTowerEventActionsResponse{}, apperrors.Validation("invalid eventId", map[string]any{"field": "eventId"})
	}
	eventID = strings.ToLower(strings.TrimSpace(eventID))

	if reqCtx.UserID == "" {
		return ControlTowerEventActionsResponse{}, apperrors.Unauthorized("verified user context is required")
	}
	if !s.readModelCfg.Mode.Enabled() || s.readModel == nil {
		return ControlTowerEventActionsResponse{}, apperrors.ControlTowerReadModelUnavailable("control tower read model is temporarily unavailable")
	}

	criticalEvents, err := s.buildTenantCriticalEvents(ctx, reqCtx)
	if err != nil {
		return ControlTowerEventActionsResponse{}, err
	}
	if _, ok := FindCriticalEventByID(criticalEvents, eventID); !ok {
		return ControlTowerEventActionsResponse{}, apperrors.NotFound("critical event not found")
	}

	rmCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), s.readModelCfg.Timeout)
	defer cancel()

	remote, depErr := s.readModel.ListCriticalEventActions(rmCtx, reqCtx.TenantID, reqCtx.UserID, reqCtx.RequestID, eventID)
	if depErr != nil {
		return ControlTowerEventActionsResponse{}, mapWorkflowDependencyError(depErr)
	}

	items := make([]ControlTowerEventAction, 0, len(remote))
	for _, item := range remote {
		occurredAt, err := time.Parse(time.RFC3339, item.OccurredAt)
		if err != nil {
			continue
		}
		items = append(items, ControlTowerEventAction{
			ActionType:  item.ActionType,
			ActorUserID: item.ActorUserID,
			OccurredAt:  occurredAt.UTC(),
			Metadata:    item.Metadata,
		})
	}
	return ControlTowerEventActionsResponse{Items: items}, nil
}

func (s *Service) enrichCriticalEventWorkflows(
	ctx context.Context,
	reqCtx RequestContext,
	events *[]ControlTowerEvent,
) {
	if events == nil || len(*events) == 0 {
		return
	}

	for i := range *events {
		if (*events)[i].Status == "" {
			(*events)[i].Status = WorkflowStatusOpen
		}
	}

	if !s.readModelCfg.Mode.Enabled() || s.readModel == nil {
		s.enrichCriticalEventAcknowledgements(ctx, reqCtx, events)
		for i := range *events {
			if (*events)[i].Acknowledgement != nil && (*events)[i].Status == WorkflowStatusOpen {
				(*events)[i].Status = WorkflowStatusAcknowledged
			}
		}
		return
	}

	eventIDs := make([]string, 0, len(*events))
	for _, event := range *events {
		eventIDs = append(eventIDs, event.ID)
	}

	rmCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), s.readModelCfg.Timeout)
	defer cancel()

	lookup, depErr := s.readModel.LookupWorkflows(rmCtx, reqCtx.TenantID, reqCtx.RequestID, eventIDs)
	if depErr != nil {
		s.enrichCriticalEventAcknowledgements(ctx, reqCtx, events)
		return
	}

	for i := range *events {
		item, ok := lookup[(*events)[i].ID]
		if !ok {
			continue
		}
		applyWorkflowLookup(&(*events)[i], item)
	}
}

func applyWorkflowLookup(event *ControlTowerEvent, item controltowerreadmodel.RemoteWorkflowLookupItem) {
	event.Status = item.Status
	if item.Acknowledgement != nil {
		if ackAt, err := time.Parse(time.RFC3339, item.Acknowledgement.AcknowledgedAt); err == nil {
			event.Acknowledgement = &ControlTowerEventAckSummary{
				AcknowledgedAt: ackAt.UTC(),
				AcknowledgedBy: ControlTowerEventAcknowledgedBy{UserID: item.Acknowledgement.UserID},
			}
		}
	} else if !item.AcknowledgedAt.IsZero() {
		event.Acknowledgement = &ControlTowerEventAckSummary{
			AcknowledgedAt: item.AcknowledgedAt,
			AcknowledgedBy: ControlTowerEventAcknowledgedBy{UserID: item.AcknowledgedByUserID},
		}
	}
	if item.Assignment != nil {
		if assignedAt, err := time.Parse(time.RFC3339, item.Assignment.AssignedAt); err == nil {
			event.Assignment = &ControlTowerEventAssignment{
				AssignedToUserID: item.Assignment.AssignedToUserID,
				AssignedByUserID: item.Assignment.AssignedByUserID,
				AssignedAt:       assignedAt.UTC(),
			}
		}
	}
	if item.Resolution != nil {
		if resolvedAt, err := time.Parse(time.RFC3339, item.Resolution.ResolvedAt); err == nil {
			event.Resolution = &ControlTowerEventResolution{
				ResolvedByUserID: item.Resolution.ResolvedByUserID,
				ResolvedAt:       resolvedAt.UTC(),
				ResolutionCode:   item.Resolution.ResolutionCode,
				Comment:          item.Resolution.Comment,
			}
		}
	}
}

func mapRemoteWorkflow(remote controltowerreadmodel.RemoteWorkflow) ControlTowerEventWorkflow {
	result := ControlTowerEventWorkflow{
		EventID: remote.EventID,
		Status:  remote.Status,
	}
	if remote.Acknowledgement != nil {
		if ackAt, err := time.Parse(time.RFC3339, remote.Acknowledgement.AcknowledgedAt); err == nil {
			result.Acknowledgement = &ControlTowerEventAckSummary{
				AcknowledgedAt: ackAt.UTC(),
				AcknowledgedBy: ControlTowerEventAcknowledgedBy{UserID: remote.Acknowledgement.UserID},
			}
		}
	}
	if remote.Assignment != nil {
		if assignedAt, err := time.Parse(time.RFC3339, remote.Assignment.AssignedAt); err == nil {
			result.Assignment = &ControlTowerEventAssignment{
				AssignedToUserID: remote.Assignment.AssignedToUserID,
				AssignedByUserID: remote.Assignment.AssignedByUserID,
				AssignedAt:       assignedAt.UTC(),
			}
		}
	}
	if remote.Resolution != nil {
		if resolvedAt, err := time.Parse(time.RFC3339, remote.Resolution.ResolvedAt); err == nil {
			result.Resolution = &ControlTowerEventResolution{
				ResolvedByUserID: remote.Resolution.ResolvedByUserID,
				ResolvedAt:       resolvedAt.UTC(),
				ResolutionCode:   remote.Resolution.ResolutionCode,
				Comment:          remote.Resolution.Comment,
			}
		}
	}
	return result
}

func mapWorkflowDependencyError(depErr *controltowerreadmodel.DependencyError) error {
	if depErr == nil {
		return apperrors.ControlTowerReadModelUnavailable("control tower read model is temporarily unavailable")
	}
	switch depErr.Status {
	case http.StatusNotFound:
		return apperrors.NotFound("critical event workflow not found")
	case http.StatusConflict:
		return apperrors.Conflict("invalid critical event workflow transition", map[string]any{})
	case http.StatusBadRequest:
		return apperrors.Validation("invalid workflow request", map[string]any{})
	case http.StatusUnauthorized:
		return apperrors.Unauthorized("verified user context is required")
	}
	if depErr.Status >= 500 || depErr.Reason == controltowerreadmodel.ReasonTimeout || depErr.Reason == controltowerreadmodel.ReasonNetworkError {
		return apperrors.ControlTowerReadModelUnavailable("control tower read model is temporarily unavailable")
	}
	return apperrors.ControlTowerReadModelUnavailable("control tower read model is temporarily unavailable")
}

func (s *Service) validateCriticalEventMutation(
	ctx context.Context,
	reqCtx RequestContext,
	eventID string,
	rawBody []byte,
	requireBody bool,
) (ControlTowerEvent, error) {
	if !ValidateCriticalEventID(strings.ToLower(strings.TrimSpace(eventID))) {
		return ControlTowerEvent{}, apperrors.Validation("invalid eventId", map[string]any{"field": "eventId"})
	}
	eventID = strings.ToLower(strings.TrimSpace(eventID))

	if requireBody {
		if len(rawBody) == 0 {
			return ControlTowerEvent{}, apperrors.Validation("request body is required", map[string]any{"field": "body"})
		}
	} else if err := validateEmptyBody(rawBody); err != nil {
		return ControlTowerEvent{}, err
	}

	if reqCtx.UserID == "" {
		return ControlTowerEvent{}, apperrors.Unauthorized("verified user context is required")
	}
	if !s.readModelCfg.Mode.Enabled() || s.readModel == nil {
		return ControlTowerEvent{}, apperrors.ControlTowerReadModelUnavailable("control tower read model is temporarily unavailable")
	}

	criticalEvents, err := s.buildTenantCriticalEvents(ctx, reqCtx)
	if err != nil {
		return ControlTowerEvent{}, err
	}
	event, ok := FindCriticalEventByID(criticalEvents, eventID)
	if !ok {
		return ControlTowerEvent{}, apperrors.NotFound("critical event not found")
	}
	return event, nil
}

func validateEmptyBody(rawBody []byte) error {
	if len(rawBody) == 0 {
		return nil
	}
	var payload map[string]json.RawMessage
	if err := json.Unmarshal(rawBody, &payload); err != nil {
		return apperrors.Validation("request body must be empty", map[string]any{"field": "body"})
	}
	if len(payload) > 0 {
		return apperrors.Validation("request body must be empty", map[string]any{"field": "body"})
	}
	return nil
}

func readWorkflowRequestBody(r *http.Request) ([]byte, error) {
	if r.Body == nil {
		return nil, nil
	}
	defer r.Body.Close()
	return io.ReadAll(io.LimitReader(r.Body, 4096))
}
