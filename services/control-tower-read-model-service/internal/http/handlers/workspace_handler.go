package handlers

import (
	"context"
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/freight-platform/control-tower-read-model-service/internal/domain"
	apperrors "github.com/freight-platform/control-tower-read-model-service/internal/platform/errors"
	"github.com/freight-platform/control-tower-read-model-service/internal/platform/respond"
	"github.com/freight-platform/control-tower-read-model-service/internal/repository"
)

type WorkspaceHandler struct {
	workItems *repository.WorkItemRepository
	views     *repository.ViewRepository
	handoffs  *repository.HandoffRepository
	cases     *repository.CaseRepository
}

func NewWorkspaceHandler(
	workItems *repository.WorkItemRepository,
	views *repository.ViewRepository,
	handoffs *repository.HandoffRepository,
	cases *repository.CaseRepository,
) *WorkspaceHandler {
	return &WorkspaceHandler{workItems: workItems, views: views, handoffs: handoffs, cases: cases}
}

func (h *WorkspaceHandler) ListWorkItems(w http.ResponseWriter, r *http.Request) {
	tenantID, userID, ok := h.resolveContext(w, r)
	if !ok {
		return
	}
	filter := parseWorkItemFilter(r)
	if filter.MyWorkOnly {
		filter.OwnerUserID = &userID
	}
	page, err := h.workItems.ListWorkItems(r.Context(), tenantID, filter, &userID)
	if err != nil {
		respond.Error(w, err)
		return
	}
	respond.JSON(w, http.StatusOK, toWorkItemPageResponse(r.Context(), tenantID, page, h.cases))
}

func (h *WorkspaceHandler) GetWorkItem(w http.ResponseWriter, r *http.Request) {
	tenantID, _, ok := h.resolveContext(w, r)
	if !ok {
		return
	}
	itemType := strings.TrimSpace(chi.URLParam(r, "itemType"))
	itemID := strings.TrimSpace(chi.URLParam(r, "itemId"))
	item, err := h.workItems.GetWorkItem(r.Context(), tenantID, itemType, itemID)
	if err != nil {
		respond.Error(w, apperrors.NotFound("work item not found"))
		return
	}
	timeline, _ := h.workItems.GetWorkItemTimeline(r.Context(), tenantID, itemType, itemID)
	resp := h.enrichWorkItemResponse(r.Context(), tenantID, toWorkItemResponse(item))
	resp["timeline"] = toTimelineResponse(timeline)
	respond.JSON(w, http.StatusOK, resp)
}

func (h *WorkspaceHandler) ClaimWorkItem(w http.ResponseWriter, r *http.Request) {
	h.mutateWorkItem(w, r, "claim", nil)
}

func (h *WorkspaceHandler) AssignWorkItem(w http.ResponseWriter, r *http.Request) {
	var body struct {
		UserID string `json:"userId"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil || strings.TrimSpace(body.UserID) == "" {
		respond.Error(w, apperrors.Validation("userId is required", map[string]any{"field": "userId"}))
		return
	}
	targetID, err := uuid.Parse(strings.TrimSpace(body.UserID))
	if err != nil {
		respond.Error(w, apperrors.Validation("invalid userId", map[string]any{"field": "userId"}))
		return
	}
	h.mutateWorkItem(w, r, "assign", &targetID)
}

func (h *WorkspaceHandler) UnassignWorkItem(w http.ResponseWriter, r *http.Request) {
	h.mutateWorkItem(w, r, "unassign", nil)
}

func (h *WorkspaceHandler) BulkAction(w http.ResponseWriter, r *http.Request) {
	tenantID, userID, ok := h.resolveContext(w, r)
	if !ok {
		return
	}
	var body struct {
		Action       string                  `json:"action"`
		Items        []domain.BulkActionItem `json:"items"`
		TargetUserID *string                 `json:"targetUserId,omitempty"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		respond.Error(w, apperrors.Validation("invalid request body", map[string]any{"field": "body"}))
		return
	}
	if len(body.Items) == 0 {
		respond.Error(w, apperrors.Validation("items are required", map[string]any{"field": "items"}))
		return
	}
	if len(body.Items) > domain.BulkActionMaxBatch {
		respond.Error(w, apperrors.Validation("batch size exceeds limit", map[string]any{"max": domain.BulkActionMaxBatch}))
		return
	}
	var target *uuid.UUID
	if body.TargetUserID != nil && strings.TrimSpace(*body.TargetUserID) != "" {
		id, err := uuid.Parse(strings.TrimSpace(*body.TargetUserID))
		if err != nil {
			respond.Error(w, apperrors.Validation("invalid targetUserId", map[string]any{"field": "targetUserId"}))
			return
		}
		target = &id
	}
	outcome := h.workItems.ExecuteBulkAction(r.Context(), tenantID, userID, strings.TrimSpace(body.Action), body.Items, target)
	respond.JSON(w, http.StatusOK, outcome)
}

func (h *WorkspaceHandler) GetWorkload(w http.ResponseWriter, r *http.Request) {
	tenantID, userID, ok := h.resolveContext(w, r)
	if !ok {
		return
	}
	summaries, unassigned, err := h.workItems.CountWorkload(r.Context(), tenantID)
	if err != nil {
		respond.Error(w, err)
		return
	}
	kpi, err := h.workItems.CountWorkspaceKPI(r.Context(), tenantID, userID)
	if err != nil {
		respond.Error(w, err)
		return
	}
	items := make([]map[string]any, 0, len(summaries))
	for _, s := range summaries {
		items = append(items, map[string]any{
			"userId": s.UserID.String(), "activeWorkItems": s.ActiveWork,
			"criticalWork": s.CriticalWork,
			"unacknowledged": s.Unacknowledged, "p1": s.P1, "p2": s.P2,
			"slaBreached": s.SLABreached, "slaWarning": s.SLAWarning,
			"criticalRisks": s.CriticalRisks, "highRisks": s.HighRisks,
		})
	}
	respond.JSON(w, http.StatusOK, map[string]any{
		"operators":      items,
		"unassignedPool": unassigned,
		"kpi": map[string]any{
			"myActiveWork": kpi.MyActiveWork, "myCriticalWork": kpi.MyCriticalWork,
			"unassignedWork": kpi.UnassignedWork, "teamActiveWork": kpi.TeamActiveWork,
			"slaBreachedWork": kpi.SLABreachedWork, "slaWarningWork": kpi.SLAWarningWork,
			"criticalRiskWork": kpi.CriticalRiskWork,
		},
	})
}

func (h *WorkspaceHandler) ListViews(w http.ResponseWriter, r *http.Request) {
	tenantID, userID, ok := h.resolveContext(w, r)
	if !ok {
		return
	}
	views, err := h.views.ListViews(r.Context(), tenantID, userID)
	if err != nil {
		respond.Error(w, err)
		return
	}
	respond.JSON(w, http.StatusOK, map[string]any{"items": toViewsResponse(views)})
}

func (h *WorkspaceHandler) CreateView(w http.ResponseWriter, r *http.Request) {
	tenantID, userID, ok := h.resolveContext(w, r)
	if !ok {
		return
	}
	var body struct {
		Name    string         `json:"name"`
		Scope   string         `json:"scope"`
		Filters map[string]any `json:"filters"`
		Sort    map[string]any `json:"sort"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil || strings.TrimSpace(body.Name) == "" {
		respond.Error(w, apperrors.Validation("name is required", map[string]any{"field": "name"}))
		return
	}
	scope := body.Scope
	if scope == "" {
		scope = domain.ViewScopePrivate
	}
	view, err := h.views.CreateViewChecked(r.Context(), domain.SavedView{
		TenantID: tenantID, OwnerUserID: userID, Name: body.Name, Scope: scope,
		FilterSchemaVersion: domain.FilterSchemaVersion, Filters: body.Filters, Sort: body.Sort,
	})
	if err != nil {
		respond.Error(w, err)
		return
	}
	respond.JSON(w, http.StatusCreated, toViewResponse(view))
}

func (h *WorkspaceHandler) UpdateView(w http.ResponseWriter, r *http.Request) {
	tenantID, userID, ok := h.resolveContext(w, r)
	if !ok {
		return
	}
	viewID, err := uuid.Parse(chi.URLParam(r, "viewId"))
	if err != nil {
		respond.Error(w, apperrors.Validation("invalid viewId", map[string]any{"field": "viewId"}))
		return
	}
	var body struct {
		Name      *string        `json:"name"`
		Scope     *string        `json:"scope"`
		Filters   map[string]any `json:"filters"`
		Sort      map[string]any `json:"sort"`
		IsDefault *bool          `json:"isDefault"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		respond.Error(w, apperrors.Validation("invalid request body", map[string]any{"field": "body"}))
		return
	}
	patch := domain.SavedView{}
	if body.Name != nil {
		patch.Name = *body.Name
	}
	if body.Scope != nil {
		patch.Scope = *body.Scope
	}
	if body.Filters != nil {
		patch.Filters = body.Filters
	}
	if body.Sort != nil {
		patch.Sort = body.Sort
	}
	if body.IsDefault != nil && *body.IsDefault {
		patch.IsDefault = true
	}
	view, err := h.views.UpdateView(r.Context(), tenantID, userID, viewID, patch)
	if err != nil {
		respond.Error(w, err)
		return
	}
	respond.JSON(w, http.StatusOK, toViewResponse(view))
}

func (h *WorkspaceHandler) DeleteView(w http.ResponseWriter, r *http.Request) {
	tenantID, userID, ok := h.resolveContext(w, r)
	if !ok {
		return
	}
	viewID, err := uuid.Parse(chi.URLParam(r, "viewId"))
	if err != nil {
		respond.Error(w, apperrors.Validation("invalid viewId", map[string]any{"field": "viewId"}))
		return
	}
	if err := h.views.DeleteView(r.Context(), tenantID, userID, viewID); err != nil {
		respond.Error(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *WorkspaceHandler) SetDefaultView(w http.ResponseWriter, r *http.Request) {
	tenantID, userID, ok := h.resolveContext(w, r)
	if !ok {
		return
	}
	viewID, err := uuid.Parse(chi.URLParam(r, "viewId"))
	if err != nil {
		respond.Error(w, apperrors.Validation("invalid viewId", map[string]any{"field": "viewId"}))
		return
	}
	if err := h.views.SetDefaultView(r.Context(), tenantID, userID, viewID); err != nil {
		respond.Error(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *WorkspaceHandler) CreateHandoff(w http.ResponseWriter, r *http.Request) {
	tenantID, userID, ok := h.resolveContext(w, r)
	if !ok {
		return
	}
	var body struct {
		ToUserID string                  `json:"toUserId"`
		Title    *string                 `json:"title"`
		Note     *string                 `json:"note"`
		Items    []domain.BulkActionItem `json:"items"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		respond.Error(w, apperrors.Validation("invalid request body", map[string]any{"field": "body"}))
		return
	}
	toUserID, err := uuid.Parse(strings.TrimSpace(body.ToUserID))
	if err != nil {
		respond.Error(w, apperrors.Validation("invalid toUserId", map[string]any{"field": "toUserId"}))
		return
	}
	handoff, outcome, err := h.handoffs.CreateHandoff(r.Context(), repository.CreateHandoffInput{
		TenantID: tenantID, FromUserID: userID, ToUserID: toUserID,
		Title: body.Title, Note: body.Note, Items: body.Items,
	})
	if err != nil {
		respond.Error(w, err)
		return
	}
	respond.JSON(w, http.StatusOK, map[string]any{
		"handoff": toHandoffResponse(handoff),
		"outcome": outcome,
	})
}

func (h *WorkspaceHandler) ListHandoffs(w http.ResponseWriter, r *http.Request) {
	tenantID, _, ok := h.resolveContext(w, r)
	if !ok {
		return
	}
	filter := repository.HandoffListFilter{Limit: parseIntDefault(r.URL.Query().Get("limit"), 20)}
	if v := strings.TrimSpace(r.URL.Query().Get("fromUser")); v != "" {
		if id, err := uuid.Parse(v); err == nil {
			filter.FromUserID = &id
		}
	}
	if v := strings.TrimSpace(r.URL.Query().Get("toUser")); v != "" {
		if id, err := uuid.Parse(v); err == nil {
			filter.ToUserID = &id
		}
	}
	items, err := h.handoffs.ListHandoffs(r.Context(), tenantID, filter)
	if err != nil {
		respond.Error(w, err)
		return
	}
	out := make([]map[string]any, 0, len(items))
	for _, item := range items {
		out = append(out, toHandoffResponse(item))
	}
	respond.JSON(w, http.StatusOK, map[string]any{"items": out})
}

func (h *WorkspaceHandler) GetHandoff(w http.ResponseWriter, r *http.Request) {
	tenantID, _, ok := h.resolveContext(w, r)
	if !ok {
		return
	}
	handoffID, err := uuid.Parse(chi.URLParam(r, "handoffId"))
	if err != nil {
		respond.Error(w, apperrors.Validation("invalid handoffId", map[string]any{"field": "handoffId"}))
		return
	}
	item, err := h.handoffs.GetHandoff(r.Context(), tenantID, handoffID)
	if err != nil {
		respond.Error(w, err)
		return
	}
	respond.JSON(w, http.StatusOK, toHandoffResponse(item))
}

func (h *WorkspaceHandler) mutateWorkItem(w http.ResponseWriter, r *http.Request, action string, targetUserID *uuid.UUID) {
	tenantID, userID, ok := h.resolveContext(w, r)
	if !ok {
		return
	}
	itemType := strings.TrimSpace(chi.URLParam(r, "itemType"))
	itemID := strings.TrimSpace(chi.URLParam(r, "itemId"))
	item := domain.BulkActionItem{ItemType: itemType, ItemID: itemID}
	outcome := h.workItems.ExecuteBulkAction(r.Context(), tenantID, userID, action, []domain.BulkActionItem{item}, targetUserID)
	if len(outcome.Results) == 0 {
		respond.Error(w, apperrors.Internal("mutation failed", nil))
		return
	}
	result := outcome.Results[0]
	if !result.Success {
		if result.Error != nil && strings.Contains(*result.Error, "already claimed") {
			respond.Error(w, apperrors.Conflict("work item already claimed", map[string]any{"field": "owner"}))
			return
		}
		respond.Error(w, apperrors.Conflict("work item mutation failed", map[string]any{"error": result.Error}))
		return
	}
	updated, err := h.workItems.GetWorkItem(r.Context(), tenantID, itemType, itemID)
	if err != nil {
		respond.JSON(w, http.StatusOK, map[string]any{"success": true})
		return
	}
	respond.JSON(w, http.StatusOK, toWorkItemResponse(updated))
}

func (h *WorkspaceHandler) resolveContext(w http.ResponseWriter, r *http.Request) (uuid.UUID, uuid.UUID, bool) {
	tenantID, err := resolveVerifiedTenant(r)
	if err != nil {
		respond.Error(w, err)
		return uuid.Nil, uuid.Nil, false
	}
	userID, err := resolveVerifiedUser(r)
	if err != nil || userID == uuid.Nil {
		respond.Error(w, apperrors.Unauthorized("verified user context is required"))
		return uuid.Nil, uuid.Nil, false
	}
	return tenantID, userID, true
}

func parseWorkItemFilter(r *http.Request) domain.WorkItemFilter {
	q := r.URL.Query()
	filter := domain.WorkItemFilter{
		ItemType: q.Get("itemType"), WorkflowStatus: q.Get("workflowStatus"),
		Priority: q.Get("priority"), BusinessImpact: q.Get("businessImpact"),
		SLAStatus: q.Get("slaStatus"), EscalationLevel: q.Get("escalationLevel"),
		RiskLevel: q.Get("riskLevel"), RiskStatus: q.Get("riskStatus"),
		PredictedType: q.Get("predictedExceptionType"), ExceptionCategory: q.Get("exceptionCategory"),
		Search: q.Get("search"), Preset: q.Get("preset"),
		Page: parseIntDefault(q.Get("page"), 1), Limit: parseIntDefault(q.Get("limit"), 50),
	}
	if q.Get("myWork") == "true" {
		filter.MyWorkOnly = true
	}
	if q.Get("unassigned") == "true" {
		filter.UnassignedOnly = true
	}
	if q.Get("includeCompleted") == "true" {
		filter.IncludeCompleted = true
	}
	if v := strings.TrimSpace(q.Get("ownerUserId")); v != "" {
		if id, err := uuid.Parse(v); err == nil {
			filter.OwnerUserID = &id
		}
	}
	return filter
}

func parseIntDefault(raw string, fallback int) int {
	if raw == "" {
		return fallback
	}
	n, err := strconv.Atoi(raw)
	if err != nil || n < 1 {
		return fallback
	}
	return n
}

func toWorkItemPageResponse(ctx context.Context, tenantID uuid.UUID, page domain.WorkItemPage, cases *repository.CaseRepository) map[string]any {
	items := make([]map[string]any, 0, len(page.Items))
	for _, item := range page.Items {
		resp := toWorkItemResponse(item)
		if cases != nil {
			if ref, err := cases.LookupActiveCaseForWorkItem(ctx, tenantID, item.ItemType, item.SourceID); err == nil && ref != nil {
				resp["activeCase"] = map[string]any{
					"caseId": ref.CaseID.String(), "reference": ref.Reference,
					"title": ref.Title, "status": ref.Status,
				}
			}
		}
		items = append(items, resp)
	}
	return map[string]any{
		"items": items, "page": page.Page, "limit": page.Limit,
		"total": page.Total, "hasNext": page.HasNext,
	}
}

func (h *WorkspaceHandler) enrichWorkItemResponse(ctx context.Context, tenantID uuid.UUID, resp map[string]any) map[string]any {
	if h.cases == nil {
		return resp
	}
	itemType, _ := resp["itemType"].(string)
	sourceID, _ := resp["sourceId"].(string)
	if ref, err := h.cases.LookupActiveCaseForWorkItem(ctx, tenantID, itemType, sourceID); err == nil && ref != nil {
		resp["activeCase"] = map[string]any{
			"caseId": ref.CaseID.String(), "reference": ref.Reference,
			"title": ref.Title, "status": ref.Status,
		}
	}
	return resp
}

func toWorkItemResponse(item domain.WorkItem) map[string]any {
	resp := map[string]any{
		"id": item.ID, "itemType": item.ItemType, "sourceId": item.SourceID,
		"shipmentId": item.ShipmentID.String(), "title": item.Title, "summary": item.Summary,
		"workflowStatus": item.WorkflowStatus, "urgency": item.Urgency,
		"createdAt":        item.CreatedAt.UTC().Format(time.RFC3339),
		"updatedAt":        item.UpdatedAt.UTC().Format(time.RFC3339),
		"availableActions": item.AvailableActions,
	}
	if item.Priority != nil {
		resp["priority"] = *item.Priority
	}
	if item.BusinessImpact != nil {
		resp["businessImpact"] = *item.BusinessImpact
	}
	if item.ExceptionCategory != nil {
		resp["exceptionCategory"] = *item.ExceptionCategory
	}
	if item.SLAStatus != nil {
		resp["slaStatus"] = *item.SLAStatus
	}
	if item.SLAPhase != nil {
		resp["slaPhase"] = *item.SLAPhase
	}
	if item.SLADueAt != nil {
		resp["slaDueAt"] = item.SLADueAt.UTC().Format(time.RFC3339)
	}
	if item.RiskLevel != nil {
		resp["riskLevel"] = *item.RiskLevel
	}
	if item.RiskScore != nil {
		resp["riskScore"] = *item.RiskScore
	}
	if item.RiskStatus != nil {
		resp["riskStatus"] = *item.RiskStatus
	}
	if item.PredictedType != nil {
		resp["predictedExceptionType"] = *item.PredictedType
	}
	if item.EscalationLevel != nil {
		resp["escalationLevel"] = *item.EscalationLevel
	}
	if item.OwnerUserID != nil {
		resp["ownerUserId"] = item.OwnerUserID.String()
	}
	if item.ThreatenedDeadline != nil {
		resp["threatenedDeadlineAt"] = item.ThreatenedDeadline.UTC().Format(time.RFC3339)
	}
	if item.LinkedEventID != nil {
		resp["linkedEventId"] = *item.LinkedEventID
	}
	if item.EventType != nil {
		resp["eventType"] = *item.EventType
	}
	return resp
}

func toTimelineResponse(entries []domain.WorkItemTimelineEntry) []map[string]any {
	out := make([]map[string]any, 0, len(entries))
	for _, e := range entries {
		entry := map[string]any{
			"source": e.Source, "actionType": e.ActionType,
			"occurredAt": e.OccurredAt.UTC().Format(time.RFC3339),
		}
		if e.ActorUserID != nil {
			entry["actorUserId"] = e.ActorUserID.String()
		}
		if e.Metadata != nil {
			entry["metadata"] = e.Metadata
		}
		out = append(out, entry)
	}
	return out
}

func toViewsResponse(views []domain.SavedView) []map[string]any {
	out := make([]map[string]any, 0, len(views))
	for _, v := range views {
		out = append(out, toViewResponse(v))
	}
	return out
}

func toViewResponse(v domain.SavedView) map[string]any {
	return map[string]any{
		"id": v.ID.String(), "name": v.Name, "scope": v.Scope,
		"filterSchemaVersion": v.FilterSchemaVersion, "filters": v.Filters, "sort": v.Sort,
		"isDefault": v.IsDefault,
		"createdAt": v.CreatedAt.UTC().Format(time.RFC3339),
		"updatedAt": v.UpdatedAt.UTC().Format(time.RFC3339),
	}
}

func toHandoffResponse(h domain.ShiftHandoff) map[string]any {
	items := make([]map[string]any, 0, len(h.Items))
	for _, item := range h.Items {
		entry := map[string]any{
			"id": item.ID.String(), "itemType": item.ItemType, "sourceId": item.SourceID, "outcome": item.Outcome,
		}
		if item.ShipmentID != nil {
			entry["shipmentId"] = item.ShipmentID.String()
		}
		if item.ErrorCode != nil {
			entry["errorCode"] = *item.ErrorCode
		}
		items = append(items, entry)
	}
	resp := map[string]any{
		"id": h.ID.String(), "fromUserId": h.FromUserID.String(),
		"createdAt": h.CreatedAt.UTC().Format(time.RFC3339), "items": items,
	}
	if h.ToUserID != nil {
		resp["toUserId"] = h.ToUserID.String()
	}
	if h.Title != nil {
		resp["title"] = *h.Title
	}
	if h.Note != nil {
		resp["note"] = *h.Note
	}
	return resp
}
