package controltowerreadmodel

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
)

const (
	AutomationRulesPath           = "/internal/v1/control-tower/automation/rules"
	automationRulePathFmt           = "/internal/v1/control-tower/automation/rules/%s"
	AutomationEvaluatePath        = "/internal/v1/control-tower/automation/evaluate"
	AutomationRecommendationsPath = "/internal/v1/control-tower/automation/recommendations"
	automationRecommendationPathFmt = "/internal/v1/control-tower/automation/recommendations/%s"
	PlaybooksPath                 = "/internal/v1/control-tower/playbooks"
	playbookPathFmt               = "/internal/v1/control-tower/playbooks/%s"
	PlaybookExecutionsPath        = "/internal/v1/control-tower/playbook-executions"
	playbookExecutionPathFmt      = "/internal/v1/control-tower/playbook-executions/%s"
	guardedExecutionActionsFmt    = "/internal/v1/control-tower/automation/executions/%s/actions"
	guardedExecutionActionFmt     = "/internal/v1/control-tower/automation/executions/%s/actions/%s"
	AutomationKPIPath             = "/internal/v1/control-tower/automation/kpi"
)

type AutomationListFilter struct {
	Status       string
	TriggerType  string
	WorkItemType string
	WorkItemID   string
	ShipmentID   string
	CaseID       string
	Page         int
	Limit        int
}

func (c *Client) ProxyAutomationJSON(ctx context.Context, method, tenantID, userID, requestID, path string, body []byte) (json.RawMessage, *DependencyError) {
	return c.ProxyAutomationJSONWithPermissions(ctx, method, tenantID, userID, requestID, path, body, nil)
}

func (c *Client) ProxyAutomationJSONWithPermissions(ctx context.Context, method, tenantID, userID, requestID, path string, body []byte, permissions []string) (json.RawMessage, *DependencyError) {
	var payload json.RawMessage
	if err := c.doWorkspaceJSONWithPermissions(ctx, method, c.baseURL+path, tenantID, userID, requestID, body, permissions, &payload); err != nil {
		return nil, err
	}
	return payload, nil
}

func (c *Client) ListAutomationRules(ctx context.Context, tenantID, userID, requestID string, filter AutomationListFilter) (json.RawMessage, *DependencyError) {
	q := url.Values{}
	setIf(q, "status", filter.Status)
	setIf(q, "triggerType", filter.TriggerType)
	if filter.Page > 0 {
		q.Set("page", fmt.Sprintf("%d", filter.Page))
	}
	if filter.Limit > 0 {
		q.Set("limit", fmt.Sprintf("%d", filter.Limit))
	}
	path := AutomationRulesPath
	if enc := q.Encode(); enc != "" {
		path += "?" + enc
	}
	return c.ProxyAutomationJSON(ctx, http.MethodGet, tenantID, userID, requestID, path, nil)
}

func (c *Client) ListPlaybooks(ctx context.Context, tenantID, userID, requestID string, filter AutomationListFilter) (json.RawMessage, *DependencyError) {
	q := url.Values{}
	setIf(q, "status", filter.Status)
	if filter.Page > 0 {
	 q.Set("page", fmt.Sprintf("%d", filter.Page))
	}
	if filter.Limit > 0 {
	 q.Set("limit", fmt.Sprintf("%d", filter.Limit))
	}
	path := PlaybooksPath
	if enc := q.Encode(); enc != "" {
		path += "?" + enc
	}
	return c.ProxyAutomationJSON(ctx, http.MethodGet, tenantID, userID, requestID, path, nil)
}

func (c *Client) ListRecommendations(ctx context.Context, tenantID, userID, requestID string, filter AutomationListFilter) (json.RawMessage, *DependencyError) {
	q := url.Values{}
	setIf(q, "status", filter.Status)
	setIf(q, "workItemType", filter.WorkItemType)
	setIf(q, "workItemId", filter.WorkItemID)
	setIf(q, "shipmentId", filter.ShipmentID)
	setIf(q, "caseId", filter.CaseID)
	if filter.Page > 0 {
		q.Set("page", fmt.Sprintf("%d", filter.Page))
	}
	if filter.Limit > 0 {
		q.Set("limit", fmt.Sprintf("%d", filter.Limit))
	}
	path := AutomationRecommendationsPath
	if enc := q.Encode(); enc != "" {
		path += "?" + enc
	}
	return c.ProxyAutomationJSON(ctx, http.MethodGet, tenantID, userID, requestID, path, nil)
}

func (c *Client) ListPlaybookExecutions(ctx context.Context, tenantID, userID, requestID string, filter AutomationListFilter) (json.RawMessage, *DependencyError) {
	q := url.Values{}
	setIf(q, "status", filter.Status)
	setIf(q, "workItemType", filter.WorkItemType)
	setIf(q, "workItemId", filter.WorkItemID)
	setIf(q, "caseId", filter.CaseID)
	if filter.Page > 0 {
		q.Set("page", fmt.Sprintf("%d", filter.Page))
	}
	if filter.Limit > 0 {
		q.Set("limit", fmt.Sprintf("%d", filter.Limit))
	}
	path := PlaybookExecutionsPath
	if enc := q.Encode(); enc != "" {
		path += "?" + enc
	}
	return c.ProxyAutomationJSON(ctx, http.MethodGet, tenantID, userID, requestID, path, nil)
}

func (c *Client) GetAutomationKPI(ctx context.Context, tenantID, userID, requestID string) (json.RawMessage, *DependencyError) {
	return c.ProxyAutomationJSON(ctx, http.MethodGet, tenantID, userID, requestID, AutomationKPIPath, nil)
}

func AutomationRulePath(ruleID, suffix string) string {
	base := fmt.Sprintf(automationRulePathFmt, url.PathEscape(ruleID))
	if suffix == "" {
		return base
	}
	return base + "/" + strings.TrimPrefix(suffix, "/")
}

func AutomationRecommendationPath(recommendationID, suffix string) string {
	base := fmt.Sprintf(automationRecommendationPathFmt, url.PathEscape(recommendationID))
	if suffix == "" {
		return base
	}
	return base + "/" + strings.TrimPrefix(suffix, "/")
}

func FormatPlaybookPath(playbookID string) string {
	return fmt.Sprintf(playbookPathFmt, url.PathEscape(playbookID))
}

func PlaybookExecutionPath(executionID, suffix string) string {
	base := fmt.Sprintf(playbookExecutionPathFmt, url.PathEscape(executionID))
	if suffix == "" {
		return base
	}
	return base + "/" + strings.TrimPrefix(suffix, "/")
}

func GuardedExecutionActionsPath(executionID string) string {
	return fmt.Sprintf(guardedExecutionActionsFmt, url.PathEscape(executionID))
}

func GuardedExecutionActionPath(executionID, actionID, suffix string) string {
	base := fmt.Sprintf(guardedExecutionActionFmt, url.PathEscape(executionID), url.PathEscape(actionID))
	if suffix == "" {
		return base
	}
	return base + "/" + strings.TrimPrefix(suffix, "/")
}

func (c *Client) EvaluateAutomation(ctx context.Context, tenantID, userID, requestID string, body []byte) (json.RawMessage, *DependencyError) {
	return c.ProxyAutomationJSON(ctx, http.MethodPost, tenantID, userID, requestID, AutomationEvaluatePath, body)
}
