package controltower

import (
	"context"
	"encoding/json"
	"net/http"

	"github.com/freight-platform/api-gateway/internal/controltowerreadmodel"
)

func (s *Service) proxyAutomation(ctx context.Context, reqCtx RequestContext, method, path string, raw []byte) (json.RawMessage, error) {
	if err := s.requireReadModel(); err != nil {
		return nil, err
	}
	rmCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), s.readModelCfg.Timeout)
	defer cancel()
	payload, depErr := s.readModel.ProxyAutomationJSON(rmCtx, method, reqCtx.TenantID, reqCtx.UserID, reqCtx.RequestID, path, raw)
	if depErr != nil {
		return nil, mapWorkspaceDependencyError(depErr)
	}
	return payload, nil
}

func (s *Service) ListAutomationRules(ctx context.Context, reqCtx RequestContext, r *http.Request) (json.RawMessage, error) {
	if err := s.requireReadModel(); err != nil {
		return nil, err
	}
	q := r.URL.Query()
	filter := controltowerreadmodel.AutomationListFilter{
		Status: q.Get("status"), TriggerType: q.Get("triggerType"),
		Page: parseIntQuery(q.Get("page"), 1), Limit: parseIntQuery(q.Get("limit"), 50),
	}
	rmCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), s.readModelCfg.Timeout)
	defer cancel()
	payload, depErr := s.readModel.ListAutomationRules(rmCtx, reqCtx.TenantID, reqCtx.UserID, reqCtx.RequestID, filter)
	if depErr != nil {
		return nil, mapWorkspaceDependencyError(depErr)
	}
	return payload, nil
}

func (s *Service) ListPlaybooks(ctx context.Context, reqCtx RequestContext, r *http.Request) (json.RawMessage, error) {
	if err := s.requireReadModel(); err != nil {
		return nil, err
	}
	q := r.URL.Query()
	filter := controltowerreadmodel.AutomationListFilter{
		Status: q.Get("status"),
		Page:   parseIntQuery(q.Get("page"), 1), Limit: parseIntQuery(q.Get("limit"), 50),
	}
	rmCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), s.readModelCfg.Timeout)
	defer cancel()
	payload, depErr := s.readModel.ListPlaybooks(rmCtx, reqCtx.TenantID, reqCtx.UserID, reqCtx.RequestID, filter)
	if depErr != nil {
		return nil, mapWorkspaceDependencyError(depErr)
	}
	return payload, nil
}

func (s *Service) ListRecommendations(ctx context.Context, reqCtx RequestContext, r *http.Request) (json.RawMessage, error) {
	if err := s.requireReadModel(); err != nil {
		return nil, err
	}
	q := r.URL.Query()
	filter := controltowerreadmodel.AutomationListFilter{
		Status: q.Get("status"), WorkItemType: q.Get("workItemType"), WorkItemID: q.Get("workItemId"),
		ShipmentID: q.Get("shipmentId"), CaseID: q.Get("caseId"),
		Page: parseIntQuery(q.Get("page"), 1), Limit: parseIntQuery(q.Get("limit"), 50),
	}
	rmCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), s.readModelCfg.Timeout)
	defer cancel()
	payload, depErr := s.readModel.ListRecommendations(rmCtx, reqCtx.TenantID, reqCtx.UserID, reqCtx.RequestID, filter)
	if depErr != nil {
		return nil, mapWorkspaceDependencyError(depErr)
	}
	return payload, nil
}

func (s *Service) ListPlaybookExecutions(ctx context.Context, reqCtx RequestContext, r *http.Request) (json.RawMessage, error) {
	if err := s.requireReadModel(); err != nil {
		return nil, err
	}
	q := r.URL.Query()
	filter := controltowerreadmodel.AutomationListFilter{
		Status: q.Get("status"), WorkItemType: q.Get("workItemType"), WorkItemID: q.Get("workItemId"),
		CaseID: q.Get("caseId"),
		Page:   parseIntQuery(q.Get("page"), 1), Limit: parseIntQuery(q.Get("limit"), 50),
	}
	rmCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), s.readModelCfg.Timeout)
	defer cancel()
	payload, depErr := s.readModel.ListPlaybookExecutions(rmCtx, reqCtx.TenantID, reqCtx.UserID, reqCtx.RequestID, filter)
	if depErr != nil {
		return nil, mapWorkspaceDependencyError(depErr)
	}
	return payload, nil
}

func (s *Service) GetAutomationKPI(ctx context.Context, reqCtx RequestContext) (json.RawMessage, error) {
	if err := s.requireReadModel(); err != nil {
		return nil, err
	}
	rmCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), s.readModelCfg.Timeout)
	defer cancel()
	payload, depErr := s.readModel.GetAutomationKPI(rmCtx, reqCtx.TenantID, reqCtx.UserID, reqCtx.RequestID)
	if depErr != nil {
		return nil, mapWorkspaceDependencyError(depErr)
	}
	return payload, nil
}

func (s *Service) enrichWorkItemRecommendations(ctx context.Context, reqCtx RequestContext, itemType, itemID string, payload map[string]any) {
	if s.readModel == nil || !s.readModelCfg.Mode.Enabled() {
		return
	}
	rmCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), s.readModelCfg.Timeout)
	defer cancel()
	filter := controltowerreadmodel.AutomationListFilter{
		Status: "pending", WorkItemType: itemType, WorkItemID: itemID, Limit: 10,
	}
	raw, depErr := s.readModel.ListRecommendations(rmCtx, reqCtx.TenantID, reqCtx.UserID, reqCtx.RequestID, filter)
	if depErr != nil || len(raw) == 0 {
		return
	}
	var page struct {
		Items []json.RawMessage `json:"items"`
	}
	if err := json.Unmarshal(raw, &page); err != nil {
		return
	}
	if len(page.Items) > 0 {
		payload["recommendations"] = page.Items
	}
}

func (s *Service) enrichCaseAutomation(ctx context.Context, reqCtx RequestContext, caseID string, payload map[string]any) {
	if s.readModel == nil || !s.readModelCfg.Mode.Enabled() {
		return
	}
	rmCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), s.readModelCfg.Timeout)
	defer cancel()
	recFilter := controltowerreadmodel.AutomationListFilter{CaseID: caseID, Limit: 20}
	recRaw, _ := s.readModel.ListRecommendations(rmCtx, reqCtx.TenantID, reqCtx.UserID, reqCtx.RequestID, recFilter)
	execFilter := controltowerreadmodel.AutomationListFilter{CaseID: caseID, Limit: 20}
	execRaw, _ := s.readModel.ListPlaybookExecutions(rmCtx, reqCtx.TenantID, reqCtx.UserID, reqCtx.RequestID, execFilter)
	if len(recRaw) > 0 {
		var recPage map[string]any
		if json.Unmarshal(recRaw, &recPage) == nil {
			payload["playbookRecommendations"] = recPage["items"]
		}
	}
	if len(execRaw) > 0 {
		var execPage map[string]any
		if json.Unmarshal(execRaw, &execPage) == nil {
			payload["playbookExecutions"] = execPage["items"]
		}
	}
}

func (s *Service) fireAutomationTrigger(ctx context.Context, reqCtx RequestContext, body []byte) {
	if s.readModel == nil || !s.readModelCfg.Mode.Enabled() {
		return
	}
	parent := ctx
	if parent == nil {
		parent = context.Background()
	}
	rmCtx, cancel := context.WithTimeout(context.WithoutCancel(parent), s.readModelCfg.Timeout)
	defer cancel()
	_, _ = s.readModel.EvaluateAutomation(rmCtx, reqCtx.TenantID, reqCtx.UserID, reqCtx.RequestID, body)
}
