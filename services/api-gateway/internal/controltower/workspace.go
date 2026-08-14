package controltower

import (
	"context"
	"encoding/json"
	"net/http"
	"net/url"
	"strings"

	"github.com/freight-platform/api-gateway/internal/controltowerreadmodel"
	apperrors "github.com/freight-platform/api-gateway/internal/platform/errors"
)

type ControlTowerWorkItem struct {
	ID                 string                              `json:"id"`
	ItemType           string                              `json:"itemType"`
	SourceID           string                              `json:"sourceId"`
	ShipmentID         string                              `json:"shipmentId"`
	ShipmentNumber     string                              `json:"shipmentNumber,omitempty"`
	Title              string                              `json:"title"`
	Summary            string                              `json:"summary"`
	WorkflowStatus     string                              `json:"workflowStatus"`
	Priority           *string                             `json:"priority,omitempty"`
	BusinessImpact     *string                             `json:"businessImpact,omitempty"`
	ExceptionCategory  *string                             `json:"exceptionCategory,omitempty"`
	SLAStatus          *string                             `json:"slaStatus,omitempty"`
	SLAPhase           *string                             `json:"slaPhase,omitempty"`
	SLADueAt           *string                             `json:"slaDueAt,omitempty"`
	RiskLevel          *string                             `json:"riskLevel,omitempty"`
	RiskScore          *int                                `json:"riskScore,omitempty"`
	RiskStatus         *string                             `json:"riskStatus,omitempty"`
	PredictedType      *string                             `json:"predictedExceptionType,omitempty"`
	EscalationLevel    *string                             `json:"escalationLevel,omitempty"`
	Urgency            string                              `json:"urgency"`
	OwnerUserID        *string                             `json:"ownerUserId,omitempty"`
	OwnerDisplayName   *string                             `json:"ownerDisplayName,omitempty"`
	CreatedAt          string                              `json:"createdAt"`
	UpdatedAt          string                              `json:"updatedAt"`
	ThreatenedDeadline *string                             `json:"threatenedDeadlineAt,omitempty"`
	AvailableActions   []string                            `json:"availableActions"`
	LinkedEventID      *string                             `json:"linkedEventId,omitempty"`
	EventType          *string                             `json:"eventType,omitempty"`
	Timeline           []ControlTowerWorkItemTimelineEntry `json:"timeline,omitempty"`
}

type ControlTowerWorkItemTimelineEntry struct {
	Source           string         `json:"source"`
	ActionType       string         `json:"actionType"`
	ActorUserID      *string        `json:"actorUserId,omitempty"`
	ActorDisplayName *string        `json:"actorDisplayName,omitempty"`
	OccurredAt       string         `json:"occurredAt"`
	Metadata         map[string]any `json:"metadata,omitempty"`
}

type WorkItemsListResponse struct {
	Items   []ControlTowerWorkItem `json:"items"`
	Page    int                    `json:"page"`
	Limit   int                    `json:"limit"`
	Total   int                    `json:"total"`
	HasNext bool                   `json:"hasNext"`
	KPI     *WorkspaceKPIResponse  `json:"kpi,omitempty"`
}

type WorkspaceKPIResponse struct {
	MyActiveWork     int `json:"myActiveWork"`
	MyCriticalWork   int `json:"myCriticalWork"`
	UnassignedWork   int `json:"unassignedWork"`
	TeamActiveWork   int `json:"teamActiveWork"`
	SLABreachedWork  int `json:"slaBreachedWork"`
	SLAWarningWork   int `json:"slaWarningWork"`
	CriticalRiskWork int `json:"criticalRiskWork"`
}

type OperatorWorkload struct {
	UserID          string  `json:"userId"`
	DisplayName     *string `json:"displayName,omitempty"`
	ActiveWorkItems int     `json:"activeWorkItems"`
	CriticalWork    int     `json:"criticalWork"`
	Unacknowledged  int     `json:"unacknowledged"`
	P1              int     `json:"p1"`
	P2              int     `json:"p2"`
	SLABreached     int     `json:"slaBreached"`
	SLAWarning      int     `json:"slaWarning"`
	CriticalRisks   int     `json:"criticalRisks"`
	HighRisks       int     `json:"highRisks"`
}

type WorkloadResponse struct {
	Operators      []OperatorWorkload   `json:"operators"`
	UnassignedPool int                  `json:"unassignedPool"`
	KPI            WorkspaceKPIResponse `json:"kpi"`
}

type SavedViewResponse struct {
	ID                  string         `json:"id"`
	Name                string         `json:"name"`
	Scope               string         `json:"scope"`
	FilterSchemaVersion int            `json:"filterSchemaVersion"`
	Filters             map[string]any `json:"filters"`
	Sort                map[string]any `json:"sort"`
	IsDefault           bool           `json:"isDefault"`
	WorkspaceScope      string         `json:"workspaceScope"`
	CreatedAt           string         `json:"createdAt"`
	UpdatedAt           string         `json:"updatedAt"`
}

type SavedViewsListResponse struct {
	Items []SavedViewResponse `json:"items"`
}

type HandoffResponse struct {
	ID         string                `json:"id"`
	FromUserID string                `json:"fromUserId"`
	ToUserID   *string               `json:"toUserId,omitempty"`
	Title      *string               `json:"title,omitempty"`
	Note       *string               `json:"note,omitempty"`
	CreatedAt  string                `json:"createdAt"`
	Items      []HandoffItemResponse `json:"items"`
}

type HandoffItemResponse struct {
	ID         string  `json:"id"`
	ItemType   string  `json:"itemType"`
	SourceID   string  `json:"sourceId"`
	ShipmentID *string `json:"shipmentId,omitempty"`
	Outcome    string  `json:"outcome"`
	ErrorCode  *string `json:"errorCode,omitempty"`
}

func (s *Service) requireReadModel() error {
	if !s.readModelCfg.Mode.Enabled() || s.readModel == nil {
		return apperrors.ControlTowerReadModelUnavailable("operator workspace is temporarily unavailable")
	}
	return nil
}

func (s *Service) ListWorkItems(ctx context.Context, reqCtx RequestContext, query ListQuery) (WorkItemsListResponse, error) {
	if err := s.requireReadModel(); err != nil {
		return WorkItemsListResponse{}, err
	}
	shipmentNumbers, err := s.loadShipmentNumberMap(ctx, reqCtx)
	if err != nil {
		return WorkItemsListResponse{}, err
	}
	userNames, _ := s.loadTenantUserNames(ctx, reqCtx)

	rmCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), s.readModelCfg.Timeout)
	defer cancel()

	filter := controltowerreadmodel.WorkItemsFilter{
		ItemType: query.WorkItemType, Priority: query.Priority, RiskLevel: query.RiskLevel,
		SLAStatus: query.EventSLAStatus, Search: query.Search, Preset: query.Preset,
		Page: query.Page, Limit: query.Limit,
	}
	if query.MyWorkOnly {
		filter.MyWork = true
	}
	if query.UnassignedOnly {
		filter.Unassigned = true
	}
	if query.IncludeCompleted {
		filter.IncludeCompleted = true
	}

	page, depErr := s.readModel.ListWorkItems(rmCtx, reqCtx.TenantID, reqCtx.UserID, reqCtx.RequestID, filter)
	if depErr != nil {
		return WorkItemsListResponse{}, mapWorkspaceDependencyError(depErr)
	}

	items := make([]ControlTowerWorkItem, 0, len(page.Items))
	for _, item := range page.Items {
		items = append(items, mapRemoteWorkItem(item, shipmentNumbers, userNames))
	}

	resp := WorkItemsListResponse{
		Items: items, Page: page.Page, Limit: page.Limit, Total: page.Total, HasNext: page.HasNext,
	}
	if wl, depErr := s.readModel.GetWorkload(rmCtx, reqCtx.TenantID, reqCtx.UserID, reqCtx.RequestID); depErr == nil && wl != nil {
		resp.KPI = &WorkspaceKPIResponse{
			MyActiveWork: wl.KPI.MyActiveWork, MyCriticalWork: wl.KPI.MyCriticalWork,
			UnassignedWork: wl.KPI.UnassignedWork, TeamActiveWork: wl.KPI.TeamActiveWork,
			SLABreachedWork: wl.KPI.SLABreachedWork, SLAWarningWork: wl.KPI.SLAWarningWork,
			CriticalRiskWork: wl.KPI.CriticalRiskWork,
		}
	}
	return resp, nil
}

func (s *Service) GetWorkItem(ctx context.Context, reqCtx RequestContext, itemType, itemID string) (ControlTowerWorkItem, error) {
	if err := s.requireReadModel(); err != nil {
		return ControlTowerWorkItem{}, err
	}
	shipmentNumbers, _ := s.loadShipmentNumberMap(ctx, reqCtx)
	userNames, _ := s.loadTenantUserNames(ctx, reqCtx)

	rmCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), s.readModelCfg.Timeout)
	defer cancel()

	remote, depErr := s.readModel.GetWorkItem(rmCtx, reqCtx.TenantID, reqCtx.UserID, reqCtx.RequestID, itemType, itemID)
	if depErr != nil {
		if depErr.Status == http.StatusNotFound {
			return ControlTowerWorkItem{}, apperrors.NotFound("work item not found")
		}
		return ControlTowerWorkItem{}, mapWorkspaceDependencyError(depErr)
	}
	return mapRemoteWorkItem(*remote, shipmentNumbers, userNames), nil
}

func (s *Service) ClaimWorkItem(ctx context.Context, reqCtx RequestContext, itemType, itemID string) (ControlTowerWorkItem, error) {
	if err := s.requireReadModel(); err != nil {
		return ControlTowerWorkItem{}, err
	}
	rmCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), s.readModelCfg.Timeout)
	defer cancel()
	remote, depErr := s.readModel.ClaimWorkItem(rmCtx, controltowerreadmodel.WorkItemMutationInput{
		TenantID: reqCtx.TenantID, UserID: reqCtx.UserID, RequestID: reqCtx.RequestID,
		ItemType: itemType, ItemID: itemID,
	})
	if depErr != nil {
		if depErr.Status == http.StatusConflict {
			return ControlTowerWorkItem{}, apperrors.Conflict("work item already claimed", map[string]any{"field": "owner"})
		}
		return ControlTowerWorkItem{}, mapWorkspaceDependencyError(depErr)
	}
	shipmentNumbers, _ := s.loadShipmentNumberMap(ctx, reqCtx)
	userNames, _ := s.loadTenantUserNames(ctx, reqCtx)
	return mapRemoteWorkItem(*remote, shipmentNumbers, userNames), nil
}

func (s *Service) AssignWorkItem(ctx context.Context, reqCtx RequestContext, itemType, itemID string, rawBody []byte) (ControlTowerWorkItem, error) {
	if err := s.requireReadModel(); err != nil {
		return ControlTowerWorkItem{}, err
	}
	var payload struct {
		UserID string `json:"userId"`
	}
	if err := json.Unmarshal(rawBody, &payload); err != nil || strings.TrimSpace(payload.UserID) == "" {
		return ControlTowerWorkItem{}, apperrors.Validation("userId is required", map[string]any{"field": "userId"})
	}
	if err := s.validateTargetUser(ctx, reqCtx, payload.UserID); err != nil {
		return ControlTowerWorkItem{}, err
	}
	rmCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), s.readModelCfg.Timeout)
	defer cancel()
	remote, depErr := s.readModel.AssignWorkItem(rmCtx, controltowerreadmodel.AssignWorkItemInput{
		TenantID: reqCtx.TenantID, UserID: reqCtx.UserID, RequestID: reqCtx.RequestID,
		ItemType: itemType, ItemID: itemID, TargetUserID: strings.TrimSpace(payload.UserID),
	})
	if depErr != nil {
		return ControlTowerWorkItem{}, mapWorkspaceDependencyError(depErr)
	}
	shipmentNumbers, _ := s.loadShipmentNumberMap(ctx, reqCtx)
	userNames, _ := s.loadTenantUserNames(ctx, reqCtx)
	return mapRemoteWorkItem(*remote, shipmentNumbers, userNames), nil
}

func (s *Service) UnassignWorkItem(ctx context.Context, reqCtx RequestContext, itemType, itemID string) (ControlTowerWorkItem, error) {
	if err := s.requireReadModel(); err != nil {
		return ControlTowerWorkItem{}, err
	}
	rmCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), s.readModelCfg.Timeout)
	defer cancel()
	remote, depErr := s.readModel.UnassignWorkItem(rmCtx, controltowerreadmodel.WorkItemMutationInput{
		TenantID: reqCtx.TenantID, UserID: reqCtx.UserID, RequestID: reqCtx.RequestID,
		ItemType: itemType, ItemID: itemID,
	})
	if depErr != nil {
		return ControlTowerWorkItem{}, mapWorkspaceDependencyError(depErr)
	}
	shipmentNumbers, _ := s.loadShipmentNumberMap(ctx, reqCtx)
	userNames, _ := s.loadTenantUserNames(ctx, reqCtx)
	return mapRemoteWorkItem(*remote, shipmentNumbers, userNames), nil
}

func (s *Service) BulkWorkItemsAction(ctx context.Context, reqCtx RequestContext, rawBody []byte) (controltowerreadmodel.BulkActionOutcome, error) {
	if err := s.requireReadModel(); err != nil {
		return controltowerreadmodel.BulkActionOutcome{}, err
	}
	var payload struct {
		Action       string                                 `json:"action"`
		Items        []controltowerreadmodel.BulkActionItem `json:"items"`
		TargetUserID *string                                `json:"targetUserId,omitempty"`
	}
	if err := json.Unmarshal(rawBody, &payload); err != nil {
		return controltowerreadmodel.BulkActionOutcome{}, apperrors.Validation("invalid request body", map[string]any{"field": "body"})
	}
	if len(payload.Items) == 0 {
		return controltowerreadmodel.BulkActionOutcome{}, apperrors.Validation("items are required", map[string]any{"field": "items"})
	}
	if len(payload.Items) > 100 {
		return controltowerreadmodel.BulkActionOutcome{}, apperrors.Validation("batch size exceeds limit", map[string]any{"max": 100})
	}
	if payload.Action == "assign" {
		if payload.TargetUserID == nil || strings.TrimSpace(*payload.TargetUserID) == "" {
			return controltowerreadmodel.BulkActionOutcome{}, apperrors.Validation("targetUserId is required", map[string]any{"field": "targetUserId"})
		}
		if err := s.validateTargetUser(ctx, reqCtx, *payload.TargetUserID); err != nil {
			return controltowerreadmodel.BulkActionOutcome{}, err
		}
	}
	rmCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), s.readModelCfg.Timeout)
	defer cancel()
	outcome, depErr := s.readModel.BulkWorkItemsAction(rmCtx, controltowerreadmodel.BulkActionInput{
		TenantID: reqCtx.TenantID, UserID: reqCtx.UserID, RequestID: reqCtx.RequestID,
		Action: payload.Action, Items: payload.Items, TargetUserID: payload.TargetUserID,
	})
	if depErr != nil {
		return controltowerreadmodel.BulkActionOutcome{}, mapWorkspaceDependencyError(depErr)
	}
	return *outcome, nil
}

func (s *Service) GetWorkload(ctx context.Context, reqCtx RequestContext) (WorkloadResponse, error) {
	if err := s.requireReadModel(); err != nil {
		return WorkloadResponse{}, err
	}
	userNames, _ := s.loadTenantUserNames(ctx, reqCtx)
	rmCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), s.readModelCfg.Timeout)
	defer cancel()
	remote, depErr := s.readModel.GetWorkload(rmCtx, reqCtx.TenantID, reqCtx.UserID, reqCtx.RequestID)
	if depErr != nil {
		return WorkloadResponse{}, mapWorkspaceDependencyError(depErr)
	}
	operators := make([]OperatorWorkload, 0, len(remote.Operators))
	for _, op := range remote.Operators {
		entry := OperatorWorkload{
			UserID: op.UserID, ActiveWorkItems: op.ActiveWorkItems, CriticalWork: op.CriticalWork,
			Unacknowledged: op.Unacknowledged,
			P1: op.P1, P2: op.P2, SLABreached: op.SLABreached, SLAWarning: op.SLAWarning,
			CriticalRisks: op.CriticalRisks, HighRisks: op.HighRisks,
		}
		if name, ok := userNames[op.UserID]; ok {
			entry.DisplayName = &name
		}
		operators = append(operators, entry)
	}
	return WorkloadResponse{
		Operators: operators, UnassignedPool: remote.UnassignedPool,
		KPI: WorkspaceKPIResponse{
			MyActiveWork: remote.KPI.MyActiveWork, MyCriticalWork: remote.KPI.MyCriticalWork,
			UnassignedWork: remote.KPI.UnassignedWork, TeamActiveWork: remote.KPI.TeamActiveWork,
			SLABreachedWork: remote.KPI.SLABreachedWork, SLAWarningWork: remote.KPI.SLAWarningWork,
			CriticalRiskWork: remote.KPI.CriticalRiskWork,
		},
	}, nil
}

func (s *Service) ListSavedViews(ctx context.Context, reqCtx RequestContext, workspaceScope string) (SavedViewsListResponse, error) {
	if err := s.requireReadModel(); err != nil {
		return SavedViewsListResponse{}, err
	}
	rmCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), s.readModelCfg.Timeout)
	defer cancel()
	items, depErr := s.readModel.ListViews(rmCtx, reqCtx.TenantID, reqCtx.UserID, reqCtx.RequestID, workspaceScope)
	if depErr != nil {
		return SavedViewsListResponse{}, mapWorkspaceDependencyError(depErr)
	}
	out := make([]SavedViewResponse, 0, len(items))
	for _, item := range items {
		out = append(out, mapSavedView(item))
	}
	return SavedViewsListResponse{Items: out}, nil
}

func (s *Service) CreateSavedView(ctx context.Context, reqCtx RequestContext, rawBody []byte) (SavedViewResponse, error) {
	if err := s.requireReadModel(); err != nil {
		return SavedViewResponse{}, err
	}
	var payload struct {
		Name           string         `json:"name"`
		Scope          string         `json:"scope"`
		WorkspaceScope string         `json:"workspaceScope"`
		Filters        map[string]any `json:"filters"`
		Sort           map[string]any `json:"sort"`
	}
	if err := json.Unmarshal(rawBody, &payload); err != nil || strings.TrimSpace(payload.Name) == "" {
		return SavedViewResponse{}, apperrors.Validation("name is required", map[string]any{"field": "name"})
	}
	rmCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), s.readModelCfg.Timeout)
	defer cancel()
	item, depErr := s.readModel.CreateView(rmCtx, controltowerreadmodel.CreateViewInput{
		TenantID: reqCtx.TenantID, UserID: reqCtx.UserID, RequestID: reqCtx.RequestID,
		Name: payload.Name, Scope: payload.Scope, WorkspaceScope: payload.WorkspaceScope,
		Filters: payload.Filters, Sort: payload.Sort,
	})
	if depErr != nil {
		return SavedViewResponse{}, mapWorkspaceDependencyError(depErr)
	}
	return mapSavedView(*item), nil
}

func (s *Service) UpdateSavedView(ctx context.Context, reqCtx RequestContext, viewID string, rawBody []byte) (SavedViewResponse, error) {
	if err := s.requireReadModel(); err != nil {
		return SavedViewResponse{}, err
	}
	var patch map[string]any
	if err := json.Unmarshal(rawBody, &patch); err != nil {
		return SavedViewResponse{}, apperrors.Validation("invalid request body", map[string]any{"field": "body"})
	}
	rmCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), s.readModelCfg.Timeout)
	defer cancel()
	item, depErr := s.readModel.UpdateView(rmCtx, reqCtx.TenantID, reqCtx.UserID, reqCtx.RequestID, viewID, patch)
	if depErr != nil {
		return SavedViewResponse{}, mapWorkspaceDependencyError(depErr)
	}
	return mapSavedView(*item), nil
}

func (s *Service) DeleteSavedView(ctx context.Context, reqCtx RequestContext, viewID string) error {
	if err := s.requireReadModel(); err != nil {
		return err
	}
	rmCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), s.readModelCfg.Timeout)
	defer cancel()
	if depErr := s.readModel.DeleteView(rmCtx, reqCtx.TenantID, reqCtx.UserID, reqCtx.RequestID, viewID); depErr != nil {
		return mapWorkspaceDependencyError(depErr)
	}
	return nil
}

func (s *Service) SetDefaultSavedView(ctx context.Context, reqCtx RequestContext, viewID string) error {
	if err := s.requireReadModel(); err != nil {
		return err
	}
	rmCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), s.readModelCfg.Timeout)
	defer cancel()
	if depErr := s.readModel.SetDefaultView(rmCtx, reqCtx.TenantID, reqCtx.UserID, reqCtx.RequestID, viewID); depErr != nil {
		return mapWorkspaceDependencyError(depErr)
	}
	return nil
}

func (s *Service) CreateHandoff(ctx context.Context, reqCtx RequestContext, rawBody []byte) (map[string]any, error) {
	if err := s.requireReadModel(); err != nil {
		return nil, err
	}
	var payload struct {
		ToUserID string                                 `json:"toUserId"`
		Title    *string                                `json:"title"`
		Note     *string                                `json:"note"`
		Items    []controltowerreadmodel.BulkActionItem `json:"items"`
	}
	if err := json.Unmarshal(rawBody, &payload); err != nil {
		return nil, apperrors.Validation("invalid request body", map[string]any{"field": "body"})
	}
	if err := s.validateTargetUser(ctx, reqCtx, payload.ToUserID); err != nil {
		return nil, err
	}
	rmCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), s.readModelCfg.Timeout)
	defer cancel()
	result, depErr := s.readModel.CreateHandoff(rmCtx, controltowerreadmodel.CreateHandoffInput{
		TenantID: reqCtx.TenantID, UserID: reqCtx.UserID, RequestID: reqCtx.RequestID,
		ToUserID: payload.ToUserID, Title: payload.Title, Note: payload.Note, Items: payload.Items,
	})
	if depErr != nil {
		return nil, mapWorkspaceDependencyError(depErr)
	}
	return result, nil
}

func (s *Service) ListHandoffs(ctx context.Context, reqCtx RequestContext, query url.Values) ([]HandoffResponse, error) {
	if err := s.requireReadModel(); err != nil {
		return nil, err
	}
	rmCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), s.readModelCfg.Timeout)
	defer cancel()
	items, depErr := s.readModel.ListHandoffs(rmCtx, reqCtx.TenantID, reqCtx.UserID, reqCtx.RequestID, query)
	if depErr != nil {
		return nil, mapWorkspaceDependencyError(depErr)
	}
	out := make([]HandoffResponse, 0, len(items))
	for _, item := range items {
		out = append(out, mapHandoff(item))
	}
	return out, nil
}

func (s *Service) GetHandoff(ctx context.Context, reqCtx RequestContext, handoffID string) (HandoffResponse, error) {
	if err := s.requireReadModel(); err != nil {
		return HandoffResponse{}, err
	}
	rmCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), s.readModelCfg.Timeout)
	defer cancel()
	item, depErr := s.readModel.GetHandoff(rmCtx, reqCtx.TenantID, reqCtx.UserID, reqCtx.RequestID, handoffID)
	if depErr != nil {
		if depErr.Status == http.StatusNotFound {
			return HandoffResponse{}, apperrors.NotFound("handoff not found")
		}
		return HandoffResponse{}, mapWorkspaceDependencyError(depErr)
	}
	return mapHandoff(*item), nil
}

func (s *Service) loadShipmentNumberMap(ctx context.Context, reqCtx RequestContext) (map[string]string, error) {
	shipmentsRaw, _, err := s.client.FetchShipments(ctx, reqCtx)
	if err != nil {
		return nil, apperrors.ControlTowerShipmentsUnavailable("required shipment data is temporarily unavailable")
	}
	out := map[string]string{}
	for _, raw := range shipmentsRaw {
		out[raw.ID] = raw.ShipmentNumber
	}
	return out, nil
}

func (s *Service) loadTenantUserNames(ctx context.Context, reqCtx RequestContext) (map[string]string, error) {
	users, err := s.client.FetchTenantUsers(ctx, reqCtx)
	if err != nil {
		return map[string]string{}, err
	}
	out := map[string]string{}
	for _, user := range users {
		out[user.ID] = user.FullName
	}
	return out, nil
}

func (s *Service) validateTargetUser(ctx context.Context, reqCtx RequestContext, userID string) error {
	userID = strings.TrimSpace(userID)
	if userID == "" {
		return apperrors.Validation("userId is required", map[string]any{"field": "userId"})
	}
	users, err := s.client.FetchTenantUsers(ctx, reqCtx)
	if err != nil {
		return apperrors.AuthDependencyUnavailable("user directory is temporarily unavailable")
	}
	for _, user := range users {
		if user.ID == userID && user.TenantID == reqCtx.TenantID {
			return nil
		}
	}
	return apperrors.Validation("target user is not eligible for assignment", map[string]any{"field": "userId"})
}

func mapWorkspaceDependencyError(depErr *controltowerreadmodel.DependencyError) error {
	if depErr == nil {
		return apperrors.ControlTowerReadModelUnavailable("operator workspace is temporarily unavailable")
	}
	if depErr.Status == http.StatusConflict {
		return apperrors.Conflict("work item state conflict", map[string]any{"reason": depErr.Reason})
	}
	if depErr.Status == http.StatusNotFound {
		return apperrors.NotFound("resource not found")
	}
	return apperrors.ControlTowerReadModelUnavailable("operator workspace is temporarily unavailable")
}

func mapRemoteWorkItem(item controltowerreadmodel.RemoteWorkItem, shipmentNumbers, userNames map[string]string) ControlTowerWorkItem {
	out := ControlTowerWorkItem{
		ID: item.ID, ItemType: item.ItemType, SourceID: item.SourceID, ShipmentID: item.ShipmentID,
		Title: item.Title, Summary: item.Summary, WorkflowStatus: item.WorkflowStatus,
		Priority: item.Priority, BusinessImpact: item.BusinessImpact, ExceptionCategory: item.ExceptionCategory,
		SLAStatus: item.SLAStatus, SLAPhase: item.SLAPhase, SLADueAt: item.SLADueAt,
		RiskLevel: item.RiskLevel, RiskScore: item.RiskScore, RiskStatus: item.RiskStatus,
		PredictedType: item.PredictedType, EscalationLevel: item.EscalationLevel, Urgency: item.Urgency,
		OwnerUserID: item.OwnerUserID, CreatedAt: item.CreatedAt, UpdatedAt: item.UpdatedAt,
		ThreatenedDeadline: item.ThreatenedDeadline, AvailableActions: item.AvailableActions,
		LinkedEventID: item.LinkedEventID, EventType: item.EventType,
	}
	if num, ok := shipmentNumbers[item.ShipmentID]; ok {
		out.ShipmentNumber = num
	}
	if item.OwnerUserID != nil {
		if name, ok := userNames[*item.OwnerUserID]; ok {
			out.OwnerDisplayName = &name
		}
	}
	if len(item.Timeline) > 0 {
		out.Timeline = make([]ControlTowerWorkItemTimelineEntry, 0, len(item.Timeline))
		for _, entry := range item.Timeline {
			mapped := ControlTowerWorkItemTimelineEntry{
				Source: entry.Source, ActionType: entry.ActionType,
				ActorUserID: entry.ActorUserID, OccurredAt: entry.OccurredAt, Metadata: entry.Metadata,
			}
			if entry.ActorUserID != nil {
				if name, ok := userNames[*entry.ActorUserID]; ok {
					mapped.ActorDisplayName = &name
				}
			}
			out.Timeline = append(out.Timeline, mapped)
		}
	}
	return out
}

func mapSavedView(item controltowerreadmodel.RemoteSavedView) SavedViewResponse {
	return SavedViewResponse{
		ID: item.ID, Name: item.Name, Scope: item.Scope,
		FilterSchemaVersion: item.FilterSchemaVersion, Filters: item.Filters, Sort: item.Sort,
		IsDefault: item.IsDefault, WorkspaceScope: item.WorkspaceScope,
		CreatedAt: item.CreatedAt, UpdatedAt: item.UpdatedAt,
	}
}

func mapHandoff(item controltowerreadmodel.RemoteHandoff) HandoffResponse {
	items := make([]HandoffItemResponse, 0, len(item.Items))
	for _, hi := range item.Items {
		items = append(items, HandoffItemResponse{
			ID: hi.ID, ItemType: hi.ItemType, SourceID: hi.SourceID,
			ShipmentID: hi.ShipmentID, Outcome: hi.Outcome, ErrorCode: hi.ErrorCode,
		})
	}
	return HandoffResponse{
		ID: item.ID, FromUserID: item.FromUserID, ToUserID: item.ToUserID,
		Title: item.Title, Note: item.Note, CreatedAt: item.CreatedAt, Items: items,
	}
}
