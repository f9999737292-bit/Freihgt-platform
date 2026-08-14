package handlers

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/freight-platform/control-tower-read-model-service/internal/domain"
	apperrors "github.com/freight-platform/control-tower-read-model-service/internal/platform/errors"
	"github.com/freight-platform/control-tower-read-model-service/internal/platform/respond"
	"github.com/freight-platform/control-tower-read-model-service/internal/repository"
)

type CaseHandler struct {
	cases *repository.CaseRepository
}

func NewCaseHandler(cases *repository.CaseRepository) *CaseHandler {
	return &CaseHandler{cases: cases}
}

func (h *CaseHandler) ListCases(w http.ResponseWriter, r *http.Request) {
	tenantID, userID, ok := resolveWorkspaceContext(w, r)
	if !ok {
		return
	}
	filter := parseCaseFilter(r, userID)
	page, err := h.cases.ListCases(r.Context(), tenantID, userID, filter)
	if err != nil {
		respond.Error(w, err)
		return
	}
	caseIDs := make([]uuid.UUID, 0, len(page.Items))
	for _, c := range page.Items {
		caseIDs = append(caseIDs, c.ID)
	}
	healthMap, _ := h.cases.BatchCaseHealth(r.Context(), tenantID, caseIDs)
	respond.JSON(w, http.StatusOK, toCasePageResponse(page, healthMap))
}

func (h *CaseHandler) GetCase(w http.ResponseWriter, r *http.Request) {
	tenantID, _, ok := resolveWorkspaceContext(w, r)
	if !ok {
		return
	}
	caseID, err := parseCaseID(r)
	if err != nil {
		respond.Error(w, err)
		return
	}
	c, err := h.cases.GetCase(r.Context(), tenantID, caseID)
	if err != nil {
		respond.Error(w, err)
		return
	}
	links, _ := h.cases.ListLinks(r.Context(), tenantID, caseID)
	participants, _ := h.cases.ListParticipants(r.Context(), tenantID, caseID)
	notes, _ := h.cases.ListNotes(r.Context(), tenantID, caseID, 20)
	actions, _ := h.cases.ListActionItems(r.Context(), tenantID, caseID)
	decisions, _ := h.cases.ListDecisions(r.Context(), tenantID, caseID)
	health, _ := h.cases.GetCaseHealth(r.Context(), tenantID, caseID)
	resp := toCaseResponse(c)
	resp["links"] = toLinksResponse(links)
	resp["participants"] = toParticipantsResponse(participants)
	resp["notes"] = toNotesResponse(notes)
	resp["actionItems"] = toActionItemsResponse(actions)
	resp["decisions"] = toDecisionsResponse(decisions)
	resp["health"] = toHealthResponse(health)
	respond.JSON(w, http.StatusOK, resp)
}

func (h *CaseHandler) CreateCase(w http.ResponseWriter, r *http.Request) {
	tenantID, userID, ok := resolveWorkspaceContext(w, r)
	if !ok {
		return
	}
	var body struct {
		Title              string                  `json:"title"`
		Summary            string                  `json:"summary"`
		Severity           string                  `json:"severity"`
		OwnerUserID        *string                 `json:"ownerUserId"`
		ShipmentIDs        []string                `json:"shipmentIds"`
		WorkItems          []domain.BulkActionItem `json:"workItems"`
		ParticipantUserIDs []string                `json:"participantUserIds"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		respond.Error(w, apperrors.Validation("invalid request body", nil))
		return
	}
	input := domain.CreateCaseInput{
		Title: body.Title, Summary: body.Summary, Severity: body.Severity,
		WorkItems: body.WorkItems,
	}
	if body.OwnerUserID != nil {
		if id, err := uuid.Parse(strings.TrimSpace(*body.OwnerUserID)); err == nil {
			input.OwnerUserID = &id
		}
	}
	for _, sid := range body.ShipmentIDs {
		if id, err := uuid.Parse(strings.TrimSpace(sid)); err == nil {
			input.ShipmentIDs = append(input.ShipmentIDs, id)
		}
	}
	for _, pid := range body.ParticipantUserIDs {
		if id, err := uuid.Parse(strings.TrimSpace(pid)); err == nil {
			input.ParticipantUserIDs = append(input.ParticipantUserIDs, id)
		}
	}
	c, err := h.cases.CreateCase(r.Context(), tenantID, userID, input)
	if err != nil {
		respond.Error(w, err)
		return
	}
	respond.JSON(w, http.StatusCreated, toCaseResponse(c))
}

func (h *CaseHandler) UpdateCase(w http.ResponseWriter, r *http.Request) {
	tenantID, userID, ok := resolveWorkspaceContext(w, r)
	if !ok {
		return
	}
	caseID, err := parseCaseID(r)
	if err != nil {
		respond.Error(w, err)
		return
	}
	var body map[string]any
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		respond.Error(w, apperrors.Validation("invalid request body", nil))
		return
	}
	version := int64(0)
	if v, ok := body["version"].(float64); ok {
		version = int64(v)
	}
	c, err := h.cases.UpdateCase(r.Context(), tenantID, userID, caseID, version, body)
	if err != nil {
		respond.Error(w, err)
		return
	}
	respond.JSON(w, http.StatusOK, toCaseResponse(c))
}

func (h *CaseHandler) ClaimCase(w http.ResponseWriter, r *http.Request) {
	h.mutateCase(w, r, func(ctx context.Context, tenantID, userID, caseID uuid.UUID) (domain.OperationalCase, error) {
		return h.cases.ClaimCase(ctx, tenantID, userID, caseID)
	})
}

func (h *CaseHandler) AssignCase(w http.ResponseWriter, r *http.Request) {
	tenantID, actorID, ok := resolveWorkspaceContext(w, r)
	if !ok {
		return
	}
	caseID, err := parseCaseID(r)
	if err != nil {
		respond.Error(w, err)
		return
	}
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
	c, err := h.cases.AssignCase(r.Context(), tenantID, actorID, caseID, targetID)
	if err != nil {
		respond.Error(w, err)
		return
	}
	respond.JSON(w, http.StatusOK, toCaseResponse(c))
}

func (h *CaseHandler) UnassignCase(w http.ResponseWriter, r *http.Request) {
	h.mutateCase(w, r, func(ctx context.Context, tenantID, userID, caseID uuid.UUID) (domain.OperationalCase, error) {
		return h.cases.UnassignCase(ctx, tenantID, userID, caseID)
	})
}

func (h *CaseHandler) AddLink(w http.ResponseWriter, r *http.Request) {
	tenantID, userID, ok := resolveWorkspaceContext(w, r)
	if !ok {
		return
	}
	caseID, err := parseCaseID(r)
	if err != nil {
		respond.Error(w, err)
		return
	}
	var body struct {
		EntityType string `json:"entityType"`
		EntityID   string `json:"entityId"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		respond.Error(w, apperrors.Validation("invalid request body", nil))
		return
	}
	link, err := h.cases.AddLink(r.Context(), tenantID, userID, caseID, strings.TrimSpace(body.EntityType), strings.TrimSpace(body.EntityID))
	if err != nil {
		respond.Error(w, err)
		return
	}
	respond.JSON(w, http.StatusCreated, toLinkResponse(link))
}

func (h *CaseHandler) RemoveLink(w http.ResponseWriter, r *http.Request) {
	tenantID, userID, ok := resolveWorkspaceContext(w, r)
	if !ok {
		return
	}
	caseID, err := parseCaseID(r)
	if err != nil {
		respond.Error(w, err)
		return
	}
	linkID, err := uuid.Parse(chi.URLParam(r, "linkId"))
	if err != nil {
		respond.Error(w, apperrors.Validation("invalid linkId", nil))
		return
	}
	if err := h.cases.RemoveLink(r.Context(), tenantID, userID, caseID, linkID); err != nil {
		respond.Error(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *CaseHandler) CreateNote(w http.ResponseWriter, r *http.Request) {
	tenantID, userID, ok := resolveWorkspaceContext(w, r)
	if !ok {
		return
	}
	caseID, err := parseCaseID(r)
	if err != nil {
		respond.Error(w, err)
		return
	}
	var body struct {
		Body             string   `json:"body"`
		MentionedUserIDs []string `json:"mentionedUserIds"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		respond.Error(w, apperrors.Validation("invalid request body", nil))
		return
	}
	mentions := make([]uuid.UUID, 0)
	for _, mid := range body.MentionedUserIDs {
		if id, err := uuid.Parse(strings.TrimSpace(mid)); err == nil {
			mentions = append(mentions, id)
		}
	}
	note, err := h.cases.CreateNote(r.Context(), tenantID, userID, caseID, body.Body, mentions)
	if err != nil {
		respond.Error(w, err)
		return
	}
	respond.JSON(w, http.StatusCreated, toNoteResponse(note))
}

func (h *CaseHandler) UpdateNote(w http.ResponseWriter, r *http.Request) {
	tenantID, userID, ok := resolveWorkspaceContext(w, r)
	if !ok {
		return
	}
	caseID, err := parseCaseID(r)
	if err != nil {
		respond.Error(w, err)
		return
	}
	noteID, err := uuid.Parse(chi.URLParam(r, "noteId"))
	if err != nil {
		respond.Error(w, apperrors.Validation("invalid noteId", nil))
		return
	}
	var body struct {
		Body string `json:"body"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		respond.Error(w, apperrors.Validation("invalid request body", nil))
		return
	}
	note, err := h.cases.UpdateNote(r.Context(), tenantID, userID, caseID, noteID, body.Body)
	if err != nil {
		respond.Error(w, err)
		return
	}
	respond.JSON(w, http.StatusOK, toNoteResponse(note))
}

func (h *CaseHandler) CreateActionItem(w http.ResponseWriter, r *http.Request) {
	tenantID, userID, ok := resolveWorkspaceContext(w, r)
	if !ok {
		return
	}
	caseID, err := parseCaseID(r)
	if err != nil {
		respond.Error(w, err)
		return
	}
	var body struct {
		Title          string  `json:"title"`
		Description    string  `json:"description"`
		AssigneeUserID *string `json:"assigneeUserId"`
		DueAt          *string `json:"dueAt"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		respond.Error(w, apperrors.Validation("invalid request body", nil))
		return
	}
	var assigneeID *uuid.UUID
	if body.AssigneeUserID != nil {
		if id, err := uuid.Parse(strings.TrimSpace(*body.AssigneeUserID)); err == nil {
			assigneeID = &id
		}
	}
	var dueAt *time.Time
	if body.DueAt != nil && strings.TrimSpace(*body.DueAt) != "" {
		if t, err := time.Parse(time.RFC3339, strings.TrimSpace(*body.DueAt)); err == nil {
			dueAt = &t
		}
	}
	item, err := h.cases.CreateActionItem(r.Context(), tenantID, userID, caseID, body.Title, body.Description, assigneeID, dueAt)
	if err != nil {
		respond.Error(w, err)
		return
	}
	respond.JSON(w, http.StatusCreated, toActionItemResponse(item))
}

func (h *CaseHandler) UpdateActionItem(w http.ResponseWriter, r *http.Request) {
	tenantID, userID, ok := resolveWorkspaceContext(w, r)
	if !ok {
		return
	}
	caseID, err := parseCaseID(r)
	if err != nil {
		respond.Error(w, err)
		return
	}
	actionID, err := uuid.Parse(chi.URLParam(r, "actionId"))
	if err != nil {
		respond.Error(w, apperrors.Validation("invalid actionId", nil))
		return
	}
	var body map[string]any
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		respond.Error(w, apperrors.Validation("invalid request body", nil))
		return
	}
	item, err := h.cases.UpdateActionItem(r.Context(), tenantID, userID, caseID, actionID, body)
	if err != nil {
		respond.Error(w, err)
		return
	}
	respond.JSON(w, http.StatusOK, toActionItemResponse(item))
}

func (h *CaseHandler) CompleteActionItem(w http.ResponseWriter, r *http.Request) {
	tenantID, userID, ok := resolveWorkspaceContext(w, r)
	if !ok {
		return
	}
	caseID, err := parseCaseID(r)
	if err != nil {
		respond.Error(w, err)
		return
	}
	actionID, err := uuid.Parse(chi.URLParam(r, "actionId"))
	if err != nil {
		respond.Error(w, apperrors.Validation("invalid actionId", nil))
		return
	}
	item, err := h.cases.CompleteActionItem(r.Context(), tenantID, userID, caseID, actionID)
	if err != nil {
		respond.Error(w, err)
		return
	}
	respond.JSON(w, http.StatusOK, toActionItemResponse(item))
}

func (h *CaseHandler) CreateDecision(w http.ResponseWriter, r *http.Request) {
	tenantID, userID, ok := resolveWorkspaceContext(w, r)
	if !ok {
		return
	}
	caseID, err := parseCaseID(r)
	if err != nil {
		respond.Error(w, err)
		return
	}
	var body struct {
		Decision  string `json:"decision"`
		Rationale string `json:"rationale"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		respond.Error(w, apperrors.Validation("invalid request body", nil))
		return
	}
	d, err := h.cases.CreateDecision(r.Context(), tenantID, userID, caseID, body.Decision, body.Rationale)
	if err != nil {
		respond.Error(w, err)
		return
	}
	respond.JSON(w, http.StatusCreated, toDecisionResponse(d))
}

func (h *CaseHandler) ResolveCase(w http.ResponseWriter, r *http.Request) {
	tenantID, userID, ok := resolveWorkspaceContext(w, r)
	if !ok {
		return
	}
	caseID, err := parseCaseID(r)
	if err != nil {
		respond.Error(w, err)
		return
	}
	var body struct {
		ResolutionCode    string `json:"resolutionCode"`
		ResolutionSummary string `json:"resolutionSummary"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		respond.Error(w, apperrors.Validation("invalid request body", nil))
		return
	}
	c, err := h.cases.ResolveCase(r.Context(), tenantID, userID, caseID, body.ResolutionCode, body.ResolutionSummary)
	if err != nil {
		respond.Error(w, err)
		return
	}
	respond.JSON(w, http.StatusOK, toCaseResponse(c))
}

func (h *CaseHandler) CloseCase(w http.ResponseWriter, r *http.Request) {
	h.mutateCase(w, r, func(ctx context.Context, tenantID, userID, caseID uuid.UUID) (domain.OperationalCase, error) {
		return h.cases.CloseCase(ctx, tenantID, userID, caseID)
	})
}

func (h *CaseHandler) ReopenCase(w http.ResponseWriter, r *http.Request) {
	h.mutateCase(w, r, func(ctx context.Context, tenantID, userID, caseID uuid.UUID) (domain.OperationalCase, error) {
		return h.cases.ReopenCase(ctx, tenantID, userID, caseID)
	})
}

func (h *CaseHandler) GetTimeline(w http.ResponseWriter, r *http.Request) {
	tenantID, _, ok := resolveWorkspaceContext(w, r)
	if !ok {
		return
	}
	caseID, err := parseCaseID(r)
	if err != nil {
		respond.Error(w, err)
		return
	}
	page := parseIntDefault(r.URL.Query().Get("page"), 1)
	limit := parseIntDefault(r.URL.Query().Get("limit"), 50)
	events, total, err := h.cases.ListTimeline(r.Context(), tenantID, caseID, page, limit)
	if err != nil {
		respond.Error(w, err)
		return
	}
	respond.JSON(w, http.StatusOK, map[string]any{
		"items": toCaseTimelineResponse(events), "page": page, "limit": limit, "total": total,
		"hasNext": page*limit < total,
	})
}

func (h *CaseHandler) GetKPI(w http.ResponseWriter, r *http.Request) {
	tenantID, userID, ok := resolveWorkspaceContext(w, r)
	if !ok {
		return
	}
	kpi, err := h.cases.GetKPI(r.Context(), tenantID, userID)
	if err != nil {
		respond.Error(w, err)
		return
	}
	respond.JSON(w, http.StatusOK, map[string]any{
		"openCases": kpi.OpenCases, "myOpenCases": kpi.MyOpenCases,
		"criticalCases": kpi.CriticalCases, "unassignedCases": kpi.UnassignedCases,
		"casesWithSlaBreach": kpi.CasesWithSLABreach, "casesWithSlaWarning": kpi.CasesWithSLAWarning,
		"slaAtRiskCases": kpi.SlaAtRiskCases, "casesWithOverdueActions": kpi.CasesWithOverdueActions,
		"resolvedCases": kpi.ResolvedCases,
	})
}

func (h *CaseHandler) AddParticipant(w http.ResponseWriter, r *http.Request) {
	tenantID, actorID, ok := resolveWorkspaceContext(w, r)
	if !ok {
		return
	}
	caseID, err := parseCaseID(r)
	if err != nil {
		respond.Error(w, err)
		return
	}
	var body struct {
		UserID string `json:"userId"`
		Role   string `json:"role"`
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
	role := strings.TrimSpace(body.Role)
	if role == "" {
		role = domain.ParticipantRoleCollaborator
	}
	if _, err := h.cases.GetCase(r.Context(), tenantID, caseID); err != nil {
		respond.Error(w, err)
		return
	}
	if err := h.cases.AddParticipant(r.Context(), tenantID, actorID, caseID, targetID, role); err != nil {
		respond.Error(w, err)
		return
	}
	w.WriteHeader(http.StatusCreated)
}

func (h *CaseHandler) UpdateParticipant(w http.ResponseWriter, r *http.Request) {
	tenantID, actorID, ok := resolveWorkspaceContext(w, r)
	if !ok {
		return
	}
	caseID, err := parseCaseID(r)
	if err != nil {
		respond.Error(w, err)
		return
	}
	targetID, err := uuid.Parse(strings.TrimSpace(chi.URLParam(r, "userId")))
	if err != nil {
		respond.Error(w, apperrors.Validation("invalid userId", map[string]any{"field": "userId"}))
		return
	}
	var body struct {
		Role string `json:"role"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil || strings.TrimSpace(body.Role) == "" {
		respond.Error(w, apperrors.Validation("role is required", map[string]any{"field": "role"}))
		return
	}
	if err := h.cases.UpdateParticipantRole(r.Context(), tenantID, actorID, caseID, targetID, strings.TrimSpace(body.Role)); err != nil {
		respond.Error(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *CaseHandler) RemoveParticipant(w http.ResponseWriter, r *http.Request) {
	tenantID, actorID, ok := resolveWorkspaceContext(w, r)
	if !ok {
		return
	}
	caseID, err := parseCaseID(r)
	if err != nil {
		respond.Error(w, err)
		return
	}
	targetID, err := uuid.Parse(strings.TrimSpace(chi.URLParam(r, "userId")))
	if err != nil {
		respond.Error(w, apperrors.Validation("invalid userId", map[string]any{"field": "userId"}))
		return
	}
	if err := h.cases.RemoveParticipant(r.Context(), tenantID, actorID, caseID, targetID); err != nil {
		respond.Error(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *CaseHandler) FindDuplicates(w http.ResponseWriter, r *http.Request) {
	tenantID, _, ok := resolveWorkspaceContext(w, r)
	if !ok {
		return
	}
	q := r.URL.Query()
	itemType := strings.TrimSpace(q.Get("itemType"))
	itemID := strings.TrimSpace(q.Get("itemId"))
	var shipmentID *uuid.UUID
	if v := strings.TrimSpace(q.Get("shipmentId")); v != "" {
		if id, err := uuid.Parse(v); err == nil {
			shipmentID = &id
		}
	}
	refs, err := h.cases.FindDuplicateCandidates(r.Context(), tenantID, itemType, itemID, shipmentID)
	if err != nil {
		respond.Error(w, err)
		return
	}
	items := make([]map[string]any, 0, len(refs))
	for _, ref := range refs {
		items = append(items, map[string]any{
			"caseId": ref.CaseID.String(), "reference": ref.Reference,
			"title": ref.Title, "status": ref.Status,
		})
	}
	respond.JSON(w, http.StatusOK, map[string]any{"items": items})
}

func (h *CaseHandler) mutateCase(w http.ResponseWriter, r *http.Request, fn func(context.Context, uuid.UUID, uuid.UUID, uuid.UUID) (domain.OperationalCase, error)) {
	tenantID, userID, ok := resolveWorkspaceContext(w, r)
	if !ok {
		return
	}
	caseID, err := parseCaseID(r)
	if err != nil {
		respond.Error(w, err)
		return
	}
	c, err := fn(r.Context(), tenantID, userID, caseID)
	if err != nil {
		respond.Error(w, err)
		return
	}
	respond.JSON(w, http.StatusOK, toCaseResponse(c))
}

func parseCaseID(r *http.Request) (uuid.UUID, error) {
	id, err := uuid.Parse(strings.TrimSpace(chi.URLParam(r, "caseId")))
	if err != nil {
		return uuid.Nil, apperrors.Validation("invalid caseId", map[string]any{"field": "caseId"})
	}
	return id, nil
}

func parseCaseFilter(r *http.Request, userID uuid.UUID) domain.CaseListFilter {
	q := r.URL.Query()
	filter := domain.CaseListFilter{
		Status: q.Get("status"), Severity: q.Get("severity"),
		Search: q.Get("search"), Preset: q.Get("preset"),
		Page:  parseIntDefault(q.Get("page"), 1),
		Limit: parseIntDefault(q.Get("limit"), 50),
	}
	if q.Get("myCases") == "true" {
		filter.MyCases = true
	}
	if q.Get("unassigned") == "true" {
		filter.Unassigned = true
	}
	if q.Get("includeClosed") == "true" {
		filter.IncludeClosed = true
	}
	if q.Get("hasSlaBreach") == "true" {
		filter.HasSLABreach = true
	}
	if q.Get("hasSlaWarning") == "true" {
		filter.HasSLAWarning = true
	}
	if q.Get("overdueActions") == "true" || q.Get("hasOverdueActions") == "true" {
		filter.OverdueActions = true
	}
	if q.Get("hasOpenActions") == "true" {
		filter.HasOpenActions = true
	}
	switch strings.TrimSpace(q.Get("slaState")) {
	case "breached":
		filter.HasSLABreach = true
	case "warning":
		filter.HasSLAWarning = true
	}
	if v := strings.TrimSpace(q.Get("ownerUserId")); v != "" {
		if id, err := uuid.Parse(v); err == nil {
			filter.OwnerUserID = &id
		}
	}
	if v := strings.TrimSpace(q.Get("shipmentId")); v != "" {
		if id, err := uuid.Parse(v); err == nil {
			filter.ShipmentID = &id
		}
	}
	return filter
}

func toCasePageResponse(page domain.CasePage, healthMap map[uuid.UUID]domain.CaseHealth) map[string]any {
	items := make([]map[string]any, 0, len(page.Items))
	for _, c := range page.Items {
		item := toCaseResponse(c)
		if health, ok := healthMap[c.ID]; ok {
			item["health"] = toHealthResponse(health)
		}
		items = append(items, item)
	}
	return map[string]any{
		"items": items, "page": page.Page, "limit": page.Limit,
		"total": page.Total, "hasNext": page.HasNext,
	}
}

func toCaseResponse(c domain.OperationalCase) map[string]any {
	resp := map[string]any{
		"id": c.ID.String(), "reference": c.Reference, "title": c.Title, "summary": c.Summary,
		"status": c.Status, "derivedSeverity": c.DerivedSeverity, "effectiveSeverity": c.EffectiveSeverity,
		"severityOverride": c.SeverityOverride, "createdByUserId": c.CreatedByUserID.String(),
		"version":        c.Version,
		"lastActivityAt": c.LastActivityAt.UTC().Format(time.RFC3339),
		"createdAt":      c.CreatedAt.UTC().Format(time.RFC3339),
		"updatedAt":      c.UpdatedAt.UTC().Format(time.RFC3339),
	}
	if c.OwnerUserID != nil {
		resp["ownerUserId"] = c.OwnerUserID.String()
	}
	if c.ResolutionCode != nil {
		resp["resolutionCode"] = *c.ResolutionCode
	}
	if c.ResolutionSummary != nil {
		resp["resolutionSummary"] = *c.ResolutionSummary
	}
	if c.ResolvedAt != nil {
		resp["resolvedAt"] = c.ResolvedAt.UTC().Format(time.RFC3339)
	}
	if c.ClosedAt != nil {
		resp["closedAt"] = c.ClosedAt.UTC().Format(time.RFC3339)
	}
	return resp
}

func toLinkResponse(l domain.CaseLink) map[string]any {
	return map[string]any{
		"id": l.ID.String(), "entityType": l.EntityType, "entityId": l.EntityID,
		"linkedAt": l.LinkedAt.UTC().Format(time.RFC3339), "linkedByUserId": l.LinkedByUserID.String(),
	}
}

func toLinksResponse(links []domain.CaseLink) []map[string]any {
	out := make([]map[string]any, 0, len(links))
	for _, l := range links {
		out = append(out, toLinkResponse(l))
	}
	return out
}

func toParticipantsResponse(parts []domain.CaseParticipant) []map[string]any {
	out := make([]map[string]any, 0, len(parts))
	for _, p := range parts {
		out = append(out, map[string]any{
			"userId": p.UserID.String(), "role": p.Role,
			"addedAt": p.AddedAt.UTC().Format(time.RFC3339),
		})
	}
	return out
}

func toNoteResponse(n domain.CaseNote) map[string]any {
	resp := map[string]any{
		"id": n.ID.String(), "authorUserId": n.AuthorUserID.String(), "body": n.Body,
		"visibility": n.Visibility,
		"createdAt":  n.CreatedAt.UTC().Format(time.RFC3339),
		"updatedAt":  n.UpdatedAt.UTC().Format(time.RFC3339),
	}
	if n.EditedAt != nil {
		resp["editedAt"] = n.EditedAt.UTC().Format(time.RFC3339)
	}
	return resp
}

func toNotesResponse(notes []domain.CaseNote) []map[string]any {
	out := make([]map[string]any, 0, len(notes))
	for _, n := range notes {
		out = append(out, toNoteResponse(n))
	}
	return out
}

func toActionItemResponse(item domain.CaseActionItem) map[string]any {
	resp := map[string]any{
		"id": item.ID.String(), "title": item.Title, "description": item.Description,
		"status": item.Status, "createdByUserId": item.CreatedByUserID.String(),
		"createdAt": item.CreatedAt.UTC().Format(time.RFC3339),
		"updatedAt": item.UpdatedAt.UTC().Format(time.RFC3339),
	}
	if item.AssigneeUserID != nil {
		resp["assigneeUserId"] = item.AssigneeUserID.String()
	}
	if item.DueAt != nil {
		resp["dueAt"] = item.DueAt.UTC().Format(time.RFC3339)
	}
	if item.CompletedAt != nil {
		resp["completedAt"] = item.CompletedAt.UTC().Format(time.RFC3339)
	}
	return resp
}

func toActionItemsResponse(items []domain.CaseActionItem) []map[string]any {
	out := make([]map[string]any, 0, len(items))
	for _, item := range items {
		out = append(out, toActionItemResponse(item))
	}
	return out
}

func toDecisionResponse(d domain.CaseDecision) map[string]any {
	return map[string]any{
		"id": d.ID.String(), "decision": d.Decision, "rationale": d.Rationale,
		"decidedByUserId": d.DecidedByUserID.String(),
		"decidedAt":       d.DecidedAt.UTC().Format(time.RFC3339),
	}
}

func toDecisionsResponse(decisions []domain.CaseDecision) []map[string]any {
	out := make([]map[string]any, 0, len(decisions))
	for _, d := range decisions {
		out = append(out, toDecisionResponse(d))
	}
	return out
}

func toHealthResponse(h domain.CaseHealth) map[string]any {
	resp := map[string]any{
		"hasSlaBreach": h.HasSLABreach, "hasSlaWarning": h.HasSLAWarning,
		"openActionCount": h.OpenActionCount, "overdueActionCount": h.OverdueActionCount,
		"activeWorkItemCount":  h.ActiveWorkItemCount,
		"activeExceptionCount": h.ActiveExceptionCount, "activeRiskCount": h.ActiveRiskCount,
	}
	if h.NearestSLADueAt != nil {
		resp["nearestSlaDueAt"] = h.NearestSLADueAt.UTC().Format(time.RFC3339)
	}
	if h.NearestActionDueAt != nil {
		resp["nearestActionDueAt"] = h.NearestActionDueAt.UTC().Format(time.RFC3339)
	}
	if h.HighestExceptionPriority != nil {
		resp["highestExceptionPriority"] = *h.HighestExceptionPriority
	}
	if h.HighestRiskLevel != nil {
		resp["highestRiskLevel"] = *h.HighestRiskLevel
	}
	return resp
}

func toCaseTimelineResponse(events []domain.CaseEvent) []map[string]any {
	out := make([]map[string]any, 0, len(events))
	for _, e := range events {
		item := map[string]any{
			"id": e.ID, "source": e.Source, "actionType": e.ActionType,
			"occurredAt": e.OccurredAt.UTC().Format(time.RFC3339),
			"metadata":   e.Metadata,
		}
		if e.ActorUserID != nil {
			item["actorUserId"] = e.ActorUserID.String()
		}
		out = append(out, item)
	}
	return out
}

func resolveWorkspaceContext(w http.ResponseWriter, r *http.Request) (uuid.UUID, uuid.UUID, bool) {
	tenantID, err := resolveVerifiedTenant(r)
	if err != nil || tenantID == uuid.Nil {
		respond.Error(w, apperrors.Unauthorized("verified tenant context is required"))
		return uuid.Nil, uuid.Nil, false
	}
	userID, err := resolveVerifiedUser(r)
	if err != nil || userID == uuid.Nil {
		respond.Error(w, apperrors.Unauthorized("verified user context is required"))
		return uuid.Nil, uuid.Nil, false
	}
	return tenantID, userID, true
}
