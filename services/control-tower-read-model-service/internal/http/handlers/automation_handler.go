package handlers

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/freight-platform/control-tower-read-model-service/internal/domain"
	apperrors "github.com/freight-platform/control-tower-read-model-service/internal/platform/errors"
	"github.com/freight-platform/control-tower-read-model-service/internal/platform/respond"
	"github.com/freight-platform/control-tower-read-model-service/internal/repository"
	"github.com/freight-platform/control-tower-read-model-service/internal/service"
)

type AutomationHandler struct {
	repo     *repository.AutomationRepository
	service  *service.AutomationService
	ingress  *service.AutomationTriggerIngress
}

func NewAutomationHandler(repo *repository.AutomationRepository, svc *service.AutomationService, ingress *service.AutomationTriggerIngress) *AutomationHandler {
	return &AutomationHandler{repo: repo, service: svc, ingress: ingress}
}

func (h *AutomationHandler) ListRules(w http.ResponseWriter, r *http.Request) {
	tenantID, _, ok := resolveWorkspaceContext(w, r)
	if !ok {
		return
	}
	q := r.URL.Query()
	page, _ := parseAutomationPage(q)
	filter := repository.RuleFilter{Status: q.Get("status"), TriggerType: q.Get("triggerType"), Page: page.Page, Limit: page.Limit}
	result, err := h.repo.ListRules(r.Context(), tenantID, filter)
	if err != nil {
		respond.Error(w, err)
		return
	}
	respond.JSON(w, http.StatusOK, toRulePage(result))
}

func (h *AutomationHandler) GetRule(w http.ResponseWriter, r *http.Request) {
	tenantID, _, ok := resolveWorkspaceContext(w, r)
	if !ok {
		return
	}
	ruleID, err := parseUUIDParam(r, "ruleId")
	if err != nil {
		respond.Error(w, err)
		return
	}
	rule, err := h.repo.GetRule(r.Context(), tenantID, ruleID)
	if err != nil {
		respond.Error(w, err)
		return
	}
	respond.JSON(w, http.StatusOK, toRuleResponse(rule))
}

func (h *AutomationHandler) CreateRule(w http.ResponseWriter, r *http.Request) {
	tenantID, userID, ok := resolveWorkspaceContext(w, r)
	if !ok {
		return
	}
	var body createRuleBody
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		respond.Error(w, apperrors.Validation("invalid request body", nil))
		return
	}
	input := domain.CreateRuleInput{
		Name: body.Name, Description: body.Description, TriggerType: body.TriggerType,
		Conditions: body.Conditions, ExecutionMode: body.ExecutionMode, Priority: body.Priority,
	}
	if body.PlaybookID != nil {
		if id, err := uuid.Parse(strings.TrimSpace(*body.PlaybookID)); err == nil {
			input.PlaybookID = &id
		}
	}
	rule, err := h.repo.CreateRule(r.Context(), tenantID, userID, input)
	if err != nil {
		respond.Error(w, err)
		return
	}
	respond.JSON(w, http.StatusCreated, toRuleResponse(rule))
}

func (h *AutomationHandler) UpdateRule(w http.ResponseWriter, r *http.Request) {
	tenantID, userID, ok := resolveWorkspaceContext(w, r)
	if !ok {
		return
	}
	ruleID, err := parseUUIDParam(r, "ruleId")
	if err != nil {
		respond.Error(w, err)
		return
	}
	var body updateRuleBody
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		respond.Error(w, apperrors.Validation("invalid request body", nil))
		return
	}
	input := domain.UpdateRuleInput{
		Name: body.Name, Description: body.Description, TriggerType: body.TriggerType,
		Conditions: body.Conditions, ExecutionMode: body.ExecutionMode, Priority: body.Priority,
	}
	if body.PlaybookID != nil {
		if id, err := uuid.Parse(strings.TrimSpace(*body.PlaybookID)); err == nil {
			input.PlaybookID = &id
		}
	}
	rule, err := h.repo.UpdateRule(r.Context(), tenantID, userID, ruleID, input)
	if err != nil {
		respond.Error(w, err)
		return
	}
	respond.JSON(w, http.StatusOK, toRuleResponse(rule))
}

func (h *AutomationHandler) ActivateRule(w http.ResponseWriter, r *http.Request) {
	h.setRuleStatus(w, r, domain.RuleStatusActive)
}

func (h *AutomationHandler) DisableRule(w http.ResponseWriter, r *http.Request) {
	h.setRuleStatus(w, r, domain.RuleStatusDisabled)
}

func (h *AutomationHandler) RetireRule(w http.ResponseWriter, r *http.Request) {
	h.setRuleStatus(w, r, domain.RuleStatusRetired)
}

func (h *AutomationHandler) setRuleStatus(w http.ResponseWriter, r *http.Request, status string) {
	tenantID, userID, ok := resolveWorkspaceContext(w, r)
	if !ok {
		return
	}
	ruleID, err := parseUUIDParam(r, "ruleId")
	if err != nil {
		respond.Error(w, err)
		return
	}
	rule, err := h.repo.SetRuleStatus(r.Context(), tenantID, userID, ruleID, status)
	if err != nil {
		respond.Error(w, err)
		return
	}
	respond.JSON(w, http.StatusOK, toRuleResponse(rule))
}

func (h *AutomationHandler) DryRunRule(w http.ResponseWriter, r *http.Request) {
	tenantID, _, ok := resolveWorkspaceContext(w, r)
	if !ok {
		return
	}
	ruleID, err := parseUUIDParam(r, "ruleId")
	if err != nil {
		respond.Error(w, err)
		return
	}
	var body dryRunBody
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		respond.Error(w, apperrors.Validation("invalid request body", nil))
		return
	}
	trigger, err := body.toTrigger(tenantID)
	if err != nil {
		respond.Error(w, err)
		return
	}
	result, err := h.service.DryRunRule(r.Context(), tenantID, ruleID, trigger)
	if err != nil {
		respond.Error(w, err)
		return
	}
	respond.JSON(w, http.StatusOK, result)
}

func (h *AutomationHandler) Evaluate(w http.ResponseWriter, r *http.Request) {
	tenantID, _, ok := resolveWorkspaceContext(w, r)
	if !ok {
		return
	}
	var body evaluateBody
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		respond.Error(w, apperrors.Validation("invalid request body", nil))
		return
	}
	trigger, err := body.toTrigger(tenantID)
	if err != nil {
		respond.Error(w, err)
		return
	}
	persist := body.Persist == nil || *body.Persist
	var out service.EvaluateOutcome
	if h.ingress != nil {
		out, err = h.ingress.HandleTrigger(r.Context(), tenantID, trigger, persist)
	} else {
		out, err = h.service.EvaluateTrigger(r.Context(), tenantID, trigger, persist)
	}
	if err != nil {
		respond.Error(w, err)
		return
	}
	respond.JSON(w, http.StatusOK, out)
}

func (h *AutomationHandler) ListPlaybooks(w http.ResponseWriter, r *http.Request) {
	tenantID, _, ok := resolveWorkspaceContext(w, r)
	if !ok {
		return
	}
	q := r.URL.Query()
	page, _ := parseAutomationPage(q)
	filter := repository.PlaybookFilter{Status: q.Get("status"), Page: page.Page, Limit: page.Limit}
	result, err := h.repo.ListPlaybooks(r.Context(), tenantID, filter)
	if err != nil {
		respond.Error(w, err)
		return
	}
	items := make([]map[string]any, 0, len(result.Items))
	for _, p := range result.Items {
		stepCount, _ := h.repo.CountPlaybookSteps(r.Context(), tenantID, p.ID)
		items = append(items, toPlaybookSummary(p, stepCount))
	}
	respond.JSON(w, http.StatusOK, map[string]any{"items": items, "page": result.Page, "limit": result.Limit, "total": result.Total, "hasNext": result.HasNext})
}

func (h *AutomationHandler) GetPlaybook(w http.ResponseWriter, r *http.Request) {
	tenantID, _, ok := resolveWorkspaceContext(w, r)
	if !ok {
		return
	}
	playbookID, err := parseUUIDParam(r, "playbookId")
	if err != nil {
		respond.Error(w, err)
		return
	}
	p, pv, err := h.repo.GetPlaybook(r.Context(), tenantID, playbookID)
	if err != nil {
		respond.Error(w, err)
		return
	}
	respond.JSON(w, http.StatusOK, toPlaybookDetail(p, pv))
}

func (h *AutomationHandler) CreatePlaybook(w http.ResponseWriter, r *http.Request) {
	tenantID, userID, ok := resolveWorkspaceContext(w, r)
	if !ok {
		return
	}
	var body createPlaybookBody
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		respond.Error(w, apperrors.Validation("invalid request body", nil))
		return
	}
	p, pv, err := h.repo.CreatePlaybook(r.Context(), tenantID, userID, domain.CreatePlaybookInput{
		Name: body.Name, Description: body.Description, Steps: body.Steps,
	})
	if err != nil {
		respond.Error(w, err)
		return
	}
	respond.JSON(w, http.StatusCreated, toPlaybookDetail(p, pv))
}

func (h *AutomationHandler) UpdatePlaybook(w http.ResponseWriter, r *http.Request) {
	tenantID, userID, ok := resolveWorkspaceContext(w, r)
	if !ok {
		return
	}
	playbookID, err := parseUUIDParam(r, "playbookId")
	if err != nil {
		respond.Error(w, err)
		return
	}
	var body updatePlaybookBody
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		respond.Error(w, apperrors.Validation("invalid request body", nil))
		return
	}
	if len(body.Steps) > 0 {
		p, pv, err := h.repo.PublishPlaybookVersion(r.Context(), tenantID, userID, playbookID, body.Steps)
		if err != nil {
			respond.Error(w, err)
			return
		}
		if body.Name != nil || body.Description != nil || body.Status != nil {
			p, err = h.repo.UpdatePlaybookMeta(r.Context(), tenantID, userID, playbookID, body.Name, body.Description, body.Status)
			if err != nil {
				respond.Error(w, err)
				return
			}
			_, pv, err = h.repo.GetPlaybook(r.Context(), tenantID, playbookID)
			if err != nil {
				respond.Error(w, err)
				return
			}
		}
		respond.JSON(w, http.StatusOK, toPlaybookDetail(p, pv))
		return
	}
	p, err := h.repo.UpdatePlaybookMeta(r.Context(), tenantID, userID, playbookID, body.Name, body.Description, body.Status)
	if err != nil {
		respond.Error(w, err)
		return
	}
	_, pv, _ := h.repo.GetPlaybook(r.Context(), tenantID, playbookID)
	respond.JSON(w, http.StatusOK, toPlaybookDetail(p, pv))
}

func (h *AutomationHandler) ListRecommendations(w http.ResponseWriter, r *http.Request) {
	tenantID, _, ok := resolveWorkspaceContext(w, r)
	if !ok {
		return
	}
	q := r.URL.Query()
	page, _ := parseAutomationPage(q)
	filter := repository.RecommendationFilter{
		Status: q.Get("status"), WorkItemType: q.Get("workItemType"), WorkItemID: q.Get("workItemId"),
		Page: page.Page, Limit: page.Limit,
	}
	if sid := strings.TrimSpace(q.Get("shipmentId")); sid != "" {
		if id, err := uuid.Parse(sid); err == nil {
			filter.ShipmentID = &id
		}
	}
	if cid := strings.TrimSpace(q.Get("caseId")); cid != "" {
		if id, err := uuid.Parse(cid); err == nil {
			filter.CaseID = &id
		}
	}
	result, err := h.repo.ListRecommendations(r.Context(), tenantID, filter)
	if err != nil {
		respond.Error(w, err)
		return
	}
	respond.JSON(w, http.StatusOK, toRecommendationPage(result))
}

func (h *AutomationHandler) GetRecommendation(w http.ResponseWriter, r *http.Request) {
	tenantID, _, ok := resolveWorkspaceContext(w, r)
	if !ok {
		return
	}
	recID, err := parseUUIDParam(r, "recommendationId")
	if err != nil {
		respond.Error(w, err)
		return
	}
	rec, err := h.repo.GetRecommendation(r.Context(), tenantID, recID)
	if err != nil {
		respond.Error(w, err)
		return
	}
	respond.JSON(w, http.StatusOK, toRecommendationResponse(rec))
}

func (h *AutomationHandler) AcceptRecommendation(w http.ResponseWriter, r *http.Request) {
	tenantID, userID, ok := resolveWorkspaceContext(w, r)
	if !ok {
		return
	}
	recID, err := parseUUIDParam(r, "recommendationId")
	if err != nil {
		respond.Error(w, err)
		return
	}
	rec, exec, err := h.repo.AcceptRecommendation(r.Context(), tenantID, userID, recID)
	if err != nil {
		respond.Error(w, err)
		return
	}
	respond.JSON(w, http.StatusOK, map[string]any{
		"recommendation": toRecommendationResponse(rec),
		"execution":      toExecutionResponse(exec),
	})
}

func (h *AutomationHandler) DismissRecommendation(w http.ResponseWriter, r *http.Request) {
	tenantID, userID, ok := resolveWorkspaceContext(w, r)
	if !ok {
		return
	}
	recID, err := parseUUIDParam(r, "recommendationId")
	if err != nil {
		respond.Error(w, err)
		return
	}
	var body struct {
		Reason string `json:"reason"`
	}
	_ = json.NewDecoder(r.Body).Decode(&body)
	reason := strings.TrimSpace(body.Reason)
	if reason == "" {
		reason = domain.DismissReasonOther
	}
	rec, err := h.repo.DismissRecommendation(r.Context(), tenantID, userID, recID, reason)
	if err != nil {
		respond.Error(w, err)
		return
	}
	respond.JSON(w, http.StatusOK, toRecommendationResponse(rec))
}

func (h *AutomationHandler) ListExecutions(w http.ResponseWriter, r *http.Request) {
	tenantID, _, ok := resolveWorkspaceContext(w, r)
	if !ok {
		return
	}
	q := r.URL.Query()
	page, _ := parseAutomationPage(q)
	filter := repository.ExecutionFilter{
		Status: q.Get("status"), WorkItemType: q.Get("workItemType"), WorkItemID: q.Get("workItemId"),
		Page: page.Page, Limit: page.Limit,
	}
	if cid := strings.TrimSpace(q.Get("caseId")); cid != "" {
		if id, err := uuid.Parse(cid); err == nil {
			filter.CaseID = &id
		}
	}
	result, err := h.repo.ListExecutions(r.Context(), tenantID, filter)
	if err != nil {
		respond.Error(w, err)
		return
	}
	items := make([]map[string]any, 0, len(result.Items))
	for _, e := range result.Items {
		items = append(items, toExecutionResponse(e))
	}
	respond.JSON(w, http.StatusOK, map[string]any{"items": items, "page": result.Page, "limit": result.Limit, "total": result.Total, "hasNext": result.HasNext})
}

func (h *AutomationHandler) GetExecution(w http.ResponseWriter, r *http.Request) {
	tenantID, _, ok := resolveWorkspaceContext(w, r)
	if !ok {
		return
	}
	execID, err := parseUUIDParam(r, "executionId")
	if err != nil {
		respond.Error(w, err)
		return
	}
	exec, err := h.repo.GetExecution(r.Context(), tenantID, execID)
	if err != nil {
		respond.Error(w, err)
		return
	}
	respond.JSON(w, http.StatusOK, toExecutionResponse(exec))
}

func (h *AutomationHandler) StartExecution(w http.ResponseWriter, r *http.Request) {
	h.mutateExecution(w, r, "start")
}

func (h *AutomationHandler) CompleteExecution(w http.ResponseWriter, r *http.Request) {
	h.mutateExecution(w, r, "complete")
}

func (h *AutomationHandler) CancelExecution(w http.ResponseWriter, r *http.Request) {
	h.mutateExecution(w, r, "cancel")
}

func (h *AutomationHandler) StartExecutionStep(w http.ResponseWriter, r *http.Request) {
	h.mutateExecutionStep(w, r, "start")
}

func (h *AutomationHandler) CompleteExecutionStep(w http.ResponseWriter, r *http.Request) {
	h.mutateExecutionStep(w, r, "complete")
}

func (h *AutomationHandler) SkipExecutionStep(w http.ResponseWriter, r *http.Request) {
	h.mutateExecutionStep(w, r, "skip")
}

func (h *AutomationHandler) GetAutomationKPI(w http.ResponseWriter, r *http.Request) {
	tenantID, _, ok := resolveWorkspaceContext(w, r)
	if !ok {
		return
	}
	kpi, err := h.repo.GetKPI(r.Context(), tenantID)
	if err != nil {
		respond.Error(w, err)
		return
	}
	respond.JSON(w, http.StatusOK, kpi)
}

func (h *AutomationHandler) mutateExecution(w http.ResponseWriter, r *http.Request, action string) {
	tenantID, userID, ok := resolveWorkspaceContext(w, r)
	if !ok {
		return
	}
	execID, err := parseUUIDParam(r, "executionId")
	if err != nil {
		respond.Error(w, err)
		return
	}
	var exec domain.PlaybookExecution
	switch action {
	case "start":
		exec, err = h.repo.StartExecution(r.Context(), tenantID, userID, execID)
	case "complete":
		exec, err = h.repo.CompleteExecution(r.Context(), tenantID, userID, execID)
	case "cancel":
		exec, err = h.repo.CancelExecution(r.Context(), tenantID, userID, execID)
	}
	if err != nil {
		respond.Error(w, err)
		return
	}
	respond.JSON(w, http.StatusOK, toExecutionResponse(exec))
}

func (h *AutomationHandler) mutateExecutionStep(w http.ResponseWriter, r *http.Request, action string) {
	tenantID, userID, ok := resolveWorkspaceContext(w, r)
	if !ok {
		return
	}
	execID, err := parseUUIDParam(r, "executionId")
	if err != nil {
		respond.Error(w, err)
		return
	}
	stepID, err := parseUUIDParam(r, "stepId")
	if err != nil {
		respond.Error(w, err)
		return
	}
	var exec domain.PlaybookExecution
	switch action {
	case "start":
		exec, err = h.repo.StartExecutionStep(r.Context(), tenantID, userID, execID, stepID)
	case "complete":
		exec, err = h.repo.CompleteExecutionStep(r.Context(), tenantID, userID, execID, stepID)
	case "skip":
		var body struct {
			Reason string `json:"reason"`
		}
		_ = json.NewDecoder(r.Body).Decode(&body)
		exec, err = h.repo.SkipExecutionStep(r.Context(), tenantID, userID, execID, stepID, body.Reason)
	}
	if err != nil {
		respond.Error(w, err)
		return
	}
	respond.JSON(w, http.StatusOK, toExecutionResponse(exec))
}

type pageParams struct {
	Page  int
	Limit int
}

func parseAutomationPage(q map[string][]string) (pageParams, error) {
	p := pageParams{Page: 1, Limit: 50}
	if v := strings.TrimSpace(firstVal(q, "page")); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			p.Page = n
		}
	}
	if v := strings.TrimSpace(firstVal(q, "limit")); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 && n <= 100 {
			p.Limit = n
		}
	}
	return p, nil
}

func firstVal(q map[string][]string, key string) string {
	if vals, ok := q[key]; ok && len(vals) > 0 {
		return vals[0]
	}
	return ""
}

func parseUUIDParam(r *http.Request, name string) (uuid.UUID, error) {
	raw := strings.TrimSpace(chi.URLParam(r, name))
	id, err := uuid.Parse(raw)
	if err != nil || id == uuid.Nil {
		return uuid.Nil, apperrors.Validation("invalid id", map[string]any{"field": name})
	}
	return id, nil
}

type createRuleBody struct {
	Name          string               `json:"name"`
	Description   string               `json:"description"`
	TriggerType   string               `json:"triggerType"`
	Conditions    domain.ConditionGroup `json:"conditions"`
	PlaybookID    *string              `json:"playbookId"`
	ExecutionMode string               `json:"executionMode"`
	Priority      int                  `json:"priority"`
}

type updateRuleBody struct {
	Name          *string               `json:"name"`
	Description   *string               `json:"description"`
	TriggerType   *string               `json:"triggerType"`
	Conditions    *domain.ConditionGroup `json:"conditions"`
	PlaybookID    *string               `json:"playbookId"`
	ExecutionMode *string               `json:"executionMode"`
	Priority      *int                  `json:"priority"`
	Status        *string               `json:"status"`
}

type createPlaybookBody struct {
	Name        string                    `json:"name"`
	Description string                    `json:"description"`
	Steps       []domain.PlaybookStepInput `json:"steps"`
}

type updatePlaybookBody struct {
	Name        *string                    `json:"name"`
	Description *string                    `json:"description"`
	Status      *string                    `json:"status"`
	Steps       []domain.PlaybookStepInput `json:"steps"`
}

type dryRunBody struct {
	evaluateBody
}

type evaluateBody struct {
	TriggerType  string                  `json:"triggerType"`
	TriggerID    string                  `json:"triggerId"`
	ShipmentID   *string                 `json:"shipmentId"`
	WorkItemType string                  `json:"workItemType"`
	WorkItemID   string                  `json:"workItemId"`
	CaseID       *string                 `json:"caseId"`
	RiskID       string                  `json:"riskId"`
	ExceptionID  string                  `json:"exceptionId"`
	Attributes   domain.TriggerAttributes `json:"attributes"`
	CorrelationID string                 `json:"correlationId"`
	CausationID   string                 `json:"causationId"`
	Persist      *bool                   `json:"persist"`
}

func (b evaluateBody) toTrigger(tenantID uuid.UUID) (domain.AutomationTrigger, error) {
	trigger := domain.AutomationTrigger{
		TriggerID: b.TriggerID, TriggerType: b.TriggerType, TenantID: tenantID,
		WorkItemType: b.WorkItemType, WorkItemID: b.WorkItemID,
		RiskID: b.RiskID, ExceptionID: b.ExceptionID,
		Attributes: b.Attributes, CorrelationID: b.CorrelationID, CausationID: b.CausationID,
	}
	if b.ShipmentID != nil {
		if id, err := uuid.Parse(strings.TrimSpace(*b.ShipmentID)); err == nil {
			trigger.ShipmentID = &id
		}
	}
	if b.CaseID != nil {
		if id, err := uuid.Parse(strings.TrimSpace(*b.CaseID)); err == nil {
			trigger.CaseID = &id
		}
	}
	if trigger.TriggerType == "" {
		return trigger, apperrors.Validation("triggerType is required", map[string]any{"field": "triggerType"})
	}
	return trigger, nil
}

func toRulePage(page domain.Page[domain.AutomationRule]) map[string]any {
	items := make([]map[string]any, 0, len(page.Items))
	for _, r := range page.Items {
		items = append(items, toRuleResponse(r))
	}
	return map[string]any{"items": items, "page": page.Page, "limit": page.Limit, "total": page.Total, "hasNext": page.HasNext}
}

func toRuleResponse(r domain.AutomationRule) map[string]any {
	resp := map[string]any{
		"id": r.ID.String(), "tenantId": r.TenantID.String(), "name": r.Name, "description": r.Description,
		"status": r.Status, "triggerType": r.TriggerType, "conditions": r.Conditions,
		"conditionSchemaVersion": r.ConditionSchemaVersion, "executionMode": r.ExecutionMode,
		"priority": r.Priority, "version": r.Version,
		"createdByUserId": r.CreatedByUserID.String(), "updatedByUserId": r.UpdatedByUserID.String(),
		"createdAt": r.CreatedAt.UTC().Format(timeRFC3339), "updatedAt": r.UpdatedAt.UTC().Format(timeRFC3339),
	}
	if r.PlaybookID != nil {
		resp["playbookId"] = r.PlaybookID.String()
	}
	return resp
}

func toPlaybookSummary(p domain.OperationalPlaybook, stepCount int) map[string]any {
	return map[string]any{
		"id": p.ID.String(), "name": p.Name, "description": p.Description, "status": p.Status,
		"currentVersion": p.CurrentVersion, "stepCount": stepCount,
		"createdAt": p.CreatedAt.UTC().Format(timeRFC3339), "updatedAt": p.UpdatedAt.UTC().Format(timeRFC3339),
	}
}

func toPlaybookDetail(p domain.OperationalPlaybook, pv domain.PlaybookVersion) map[string]any {
	steps := make([]map[string]any, 0, len(pv.Steps))
	for _, s := range pv.Steps {
		steps = append(steps, map[string]any{
			"id": s.ID.String(), "sequence": s.Sequence, "title": s.Title, "description": s.Description,
			"stepType": s.StepType, "required": s.Required, "actionCode": s.ActionCode,
			"estimatedDurationMinutes": s.EstimatedDurationMinutes,
		})
	}
	return map[string]any{
		"id": p.ID.String(), "name": p.Name, "description": p.Description, "status": p.Status,
		"currentVersion": p.CurrentVersion, "steps": steps,
		"createdAt": p.CreatedAt.UTC().Format(timeRFC3339), "updatedAt": p.UpdatedAt.UTC().Format(timeRFC3339),
	}
}

func toRecommendationPage(page domain.Page[domain.AutomationRecommendation]) map[string]any {
	items := make([]map[string]any, 0, len(page.Items))
	for _, r := range page.Items {
		items = append(items, toRecommendationResponse(r))
	}
	return map[string]any{"items": items, "page": page.Page, "limit": page.Limit, "total": page.Total, "hasNext": page.HasNext}
}

func toRecommendationResponse(r domain.AutomationRecommendation) map[string]any {
	resp := map[string]any{
		"id": r.ID.String(), "ruleId": r.RuleID.String(), "ruleVersion": r.RuleVersion, "ruleName": r.RuleName,
		"playbookId": r.PlaybookID.String(), "playbookVersion": r.PlaybookVersion, "playbookName": r.PlaybookName,
		"triggerId": r.TriggerID, "triggerType": r.TriggerType, "status": r.Status,
		"matchedConditions": r.MatchExplanation, "createdAt": r.CreatedAt.UTC().Format(timeRFC3339),
		"workItemType": r.WorkItemType, "workItemId": r.WorkItemID,
		"riskId": r.RiskID, "exceptionId": r.ExceptionID,
	}
	if r.ShipmentID != nil {
		resp["shipmentId"] = r.ShipmentID.String()
	}
	if r.CaseID != nil {
		resp["caseId"] = r.CaseID.String()
	}
	if r.AcceptedByUserID != nil {
		resp["acceptedByUserId"] = r.AcceptedByUserID.String()
	}
	if r.AcceptedAt != nil {
		resp["acceptedAt"] = r.AcceptedAt.UTC().Format(timeRFC3339)
	}
	if r.DismissedByUserID != nil {
		resp["dismissedByUserId"] = r.DismissedByUserID.String()
	}
	if r.DismissedAt != nil {
		resp["dismissedAt"] = r.DismissedAt.UTC().Format(timeRFC3339)
	}
	if r.DismissReason != "" {
		resp["dismissReason"] = r.DismissReason
	}
	return resp
}

func toExecutionResponse(e domain.PlaybookExecution) map[string]any {
	steps := make([]map[string]any, 0, len(e.Steps))
	done := 0
	for _, s := range e.Steps {
		if s.Status == domain.ExecutionStepStatusDone || s.Status == domain.ExecutionStepStatusSkipped {
			done++
		}
		steps = append(steps, map[string]any{
			"id": s.ID.String(), "sequence": s.Sequence, "title": s.Title, "description": s.Description,
			"stepType": s.StepType, "required": s.Required, "actionCode": s.ActionCode, "status": s.Status,
			"skipReason": s.SkipReason,
		})
	}
	resp := map[string]any{
		"id": e.ID.String(), "playbookId": e.PlaybookID.String(), "playbookVersion": e.PlaybookVersion,
		"playbookName": e.PlaybookName, "ownerUserId": e.OwnerUserID.String(), "status": e.Status,
		"steps": steps, "progressDone": done, "progressTotal": len(steps),
		"createdAt": e.CreatedAt.UTC().Format(timeRFC3339), "updatedAt": e.UpdatedAt.UTC().Format(timeRFC3339),
	}
	if e.RecommendationID != nil {
		resp["recommendationId"] = e.RecommendationID.String()
	}
	if e.ShipmentID != nil {
		resp["shipmentId"] = e.ShipmentID.String()
	}
	if e.CaseID != nil {
		resp["caseId"] = e.CaseID.String()
	}
	if e.WorkItemID != "" {
		resp["workItemType"] = e.WorkItemType
		resp["workItemId"] = e.WorkItemID
	}
	if e.StartedAt != nil {
		resp["startedAt"] = e.StartedAt.UTC().Format(timeRFC3339)
	}
	if e.CompletedAt != nil {
		resp["completedAt"] = e.CompletedAt.UTC().Format(timeRFC3339)
	}
	return resp
}

const timeRFC3339 = "2006-01-02T15:04:05Z07:00"
