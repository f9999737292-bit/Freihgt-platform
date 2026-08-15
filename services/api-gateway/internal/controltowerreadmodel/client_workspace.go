package controltowerreadmodel

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	sharedmiddleware "github.com/freight-platform/shared-go/middleware"
)

const (
	listWorkItemsPath    = "/internal/v1/control-tower/work-items"
	getWorkItemPath      = "/internal/v1/control-tower/work-items/%s/%s"
	claimWorkItemPath    = "/internal/v1/control-tower/work-items/%s/%s/claim"
	assignWorkItemPath   = "/internal/v1/control-tower/work-items/%s/%s/assign"
	unassignWorkItemPath = "/internal/v1/control-tower/work-items/%s/%s/unassign"
	bulkWorkItemsPath    = "/internal/v1/control-tower/work-items/bulk-action"
	workloadPath         = "/internal/v1/control-tower/workload"
	listViewsPath        = "/internal/v1/control-tower/views"
	viewPath             = "/internal/v1/control-tower/views/%s"
	setDefaultViewPath   = "/internal/v1/control-tower/views/%s/set-default"
	listHandoffsPath     = "/internal/v1/control-tower/handoffs"
	handoffPath          = "/internal/v1/control-tower/handoffs/%s"
)

type WorkItemsFilter struct {
	ItemType          string
	WorkflowStatus    string
	Priority          string
	BusinessImpact    string
	SLAStatus         string
	EscalationLevel   string
	RiskLevel         string
	RiskStatus        string
	PredictedType     string
	ExceptionCategory string
	OwnerUserID       string
	MyWork            bool
	Unassigned        bool
	IncludeCompleted  bool
	Search            string
	Preset            string
	Page              int
	Limit             int
}

type RemoteWorkItem struct {
	ID                 string                `json:"id"`
	ItemType           string                `json:"itemType"`
	SourceID           string                `json:"sourceId"`
	ShipmentID         string                `json:"shipmentId"`
	Title              string                `json:"title"`
	Summary            string                `json:"summary"`
	WorkflowStatus     string                `json:"workflowStatus"`
	Priority           *string               `json:"priority,omitempty"`
	BusinessImpact     *string               `json:"businessImpact,omitempty"`
	ExceptionCategory  *string               `json:"exceptionCategory,omitempty"`
	SLAStatus          *string               `json:"slaStatus,omitempty"`
	SLAPhase           *string               `json:"slaPhase,omitempty"`
	SLADueAt           *string               `json:"slaDueAt,omitempty"`
	RiskLevel          *string               `json:"riskLevel,omitempty"`
	RiskScore          *int                  `json:"riskScore,omitempty"`
	RiskStatus         *string               `json:"riskStatus,omitempty"`
	PredictedType      *string               `json:"predictedExceptionType,omitempty"`
	EscalationLevel    *string               `json:"escalationLevel,omitempty"`
	Urgency            string                `json:"urgency"`
	OwnerUserID        *string               `json:"ownerUserId,omitempty"`
	CreatedAt          string                `json:"createdAt"`
	UpdatedAt          string                `json:"updatedAt"`
	ThreatenedDeadline *string               `json:"threatenedDeadlineAt,omitempty"`
	AvailableActions   []string              `json:"availableActions"`
	LinkedEventID      *string               `json:"linkedEventId,omitempty"`
	EventType          *string               `json:"eventType,omitempty"`
	Timeline           []RemoteTimelineEntry `json:"timeline,omitempty"`
}

type RemoteTimelineEntry struct {
	Source      string         `json:"source"`
	ActionType  string         `json:"actionType"`
	ActorUserID *string        `json:"actorUserId,omitempty"`
	OccurredAt  string         `json:"occurredAt"`
	Metadata    map[string]any `json:"metadata,omitempty"`
}

type RemoteWorkItemPage struct {
	Items   []RemoteWorkItem `json:"items"`
	Page    int              `json:"page"`
	Limit   int              `json:"limit"`
	Total   int              `json:"total"`
	HasNext bool             `json:"hasNext"`
}

type BulkActionItem struct {
	ItemType string `json:"itemType"`
	ItemID   string `json:"itemId"`
}

type BulkActionInput struct {
	TenantID     string
	UserID       string
	RequestID    string
	Action       string
	Items        []BulkActionItem
	TargetUserID *string
}

type BulkActionOutcome struct {
	Requested int                    `json:"requested"`
	Succeeded int                    `json:"succeeded"`
	Failed    int                    `json:"failed"`
	Results   []BulkActionResultItem `json:"results"`
}

type BulkActionResultItem struct {
	ItemType string  `json:"itemType"`
	ItemID   string  `json:"itemId"`
	Success  bool    `json:"success"`
	Error    *string `json:"error,omitempty"`
}

type AssignWorkItemInput struct {
	TenantID     string
	UserID       string
	RequestID    string
	ItemType     string
	ItemID       string
	TargetUserID string
}

type WorkItemMutationInput struct {
	TenantID  string
	UserID    string
	RequestID string
	ItemType  string
	ItemID    string
}

type RemoteSavedView struct {
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

type CreateViewInput struct {
	TenantID  string
	UserID    string
	RequestID string
	Name      string
	Scope           string
	WorkspaceScope  string
	Filters         map[string]any
	Sort      map[string]any
}

type CreateHandoffInput struct {
	TenantID  string
	UserID    string
	RequestID string
	ToUserID  string
	Title     *string
	Note      *string
	Items     []BulkActionItem
}

type RemoteHandoff struct {
	ID         string              `json:"id"`
	FromUserID string              `json:"fromUserId"`
	ToUserID   *string             `json:"toUserId,omitempty"`
	Title      *string             `json:"title,omitempty"`
	Note       *string             `json:"note,omitempty"`
	CreatedAt  string              `json:"createdAt"`
	Items      []RemoteHandoffItem `json:"items"`
}

type RemoteHandoffItem struct {
	ID         string  `json:"id"`
	ItemType   string  `json:"itemType"`
	SourceID   string  `json:"sourceId"`
	ShipmentID *string `json:"shipmentId,omitempty"`
	Outcome    string  `json:"outcome"`
	ErrorCode  *string `json:"errorCode,omitempty"`
}

type RemoteWorkloadResponse struct {
	Operators      []RemoteOperatorWorkload `json:"operators"`
	UnassignedPool int                      `json:"unassignedPool"`
	KPI            RemoteWorkspaceKPI       `json:"kpi"`
}

type RemoteOperatorWorkload struct {
	UserID          string `json:"userId"`
	ActiveWorkItems int    `json:"activeWorkItems"`
	CriticalWork    int    `json:"criticalWork"`
	Unacknowledged  int    `json:"unacknowledged"`
	P1              int    `json:"p1"`
	P2              int    `json:"p2"`
	SLABreached     int    `json:"slaBreached"`
	SLAWarning      int    `json:"slaWarning"`
	CriticalRisks   int    `json:"criticalRisks"`
	HighRisks       int    `json:"highRisks"`
}

type RemoteWorkspaceKPI struct {
	MyActiveWork     int `json:"myActiveWork"`
	MyCriticalWork   int `json:"myCriticalWork"`
	UnassignedWork   int `json:"unassignedWork"`
	TeamActiveWork   int `json:"teamActiveWork"`
	SLABreachedWork  int `json:"slaBreachedWork"`
	SLAWarningWork   int `json:"slaWarningWork"`
	CriticalRiskWork int `json:"criticalRiskWork"`
}

func (c *Client) ListWorkItems(ctx context.Context, tenantID, userID, requestID string, filter WorkItemsFilter) (*RemoteWorkItemPage, *DependencyError) {
	if c == nil {
		return nil, &DependencyError{Reason: ReasonUnknown, Err: fmt.Errorf("read model client is nil")}
	}
	q := url.Values{}
	setIf(q, "itemType", filter.ItemType)
	setIf(q, "workflowStatus", filter.WorkflowStatus)
	setIf(q, "priority", filter.Priority)
	setIf(q, "businessImpact", filter.BusinessImpact)
	setIf(q, "slaStatus", filter.SLAStatus)
	setIf(q, "escalationLevel", filter.EscalationLevel)
	setIf(q, "riskLevel", filter.RiskLevel)
	setIf(q, "riskStatus", filter.RiskStatus)
	setIf(q, "predictedExceptionType", filter.PredictedType)
	setIf(q, "exceptionCategory", filter.ExceptionCategory)
	setIf(q, "ownerUserId", filter.OwnerUserID)
	setIf(q, "search", filter.Search)
	setIf(q, "preset", filter.Preset)
	if filter.MyWork {
		q.Set("myWork", "true")
	}
	if filter.Unassigned {
		q.Set("unassigned", "true")
	}
	if filter.IncludeCompleted {
		q.Set("includeCompleted", "true")
	}
	if filter.Page > 0 {
		q.Set("page", fmt.Sprintf("%d", filter.Page))
	}
	if filter.Limit > 0 {
		q.Set("limit", fmt.Sprintf("%d", filter.Limit))
	}
	endpoint := c.baseURL + listWorkItemsPath
	if encoded := q.Encode(); encoded != "" {
		endpoint += "?" + encoded
	}
	var payload RemoteWorkItemPage
	if err := c.doWorkspaceJSON(ctx, http.MethodGet, endpoint, tenantID, userID, requestID, nil, &payload); err != nil {
		return nil, err
	}
	return &payload, nil
}

func (c *Client) GetWorkItem(ctx context.Context, tenantID, userID, requestID, itemType, itemID string) (*RemoteWorkItem, *DependencyError) {
	if c == nil {
		return nil, &DependencyError{Reason: ReasonUnknown, Err: fmt.Errorf("read model client is nil")}
	}
	endpoint := c.baseURL + fmt.Sprintf(getWorkItemPath, itemType, itemID)
	var payload RemoteWorkItem
	if err := c.doWorkspaceJSON(ctx, http.MethodGet, endpoint, tenantID, userID, requestID, nil, &payload); err != nil {
		return nil, err
	}
	return &payload, nil
}

func (c *Client) ClaimWorkItem(ctx context.Context, input WorkItemMutationInput) (*RemoteWorkItem, *DependencyError) {
	endpoint := c.baseURL + fmt.Sprintf(claimWorkItemPath, input.ItemType, input.ItemID)
	var payload RemoteWorkItem
	if err := c.doWorkspaceJSON(ctx, http.MethodPost, endpoint, input.TenantID, input.UserID, input.RequestID, nil, &payload); err != nil {
		return nil, err
	}
	return &payload, nil
}

func (c *Client) AssignWorkItem(ctx context.Context, input AssignWorkItemInput) (*RemoteWorkItem, *DependencyError) {
	body, _ := json.Marshal(map[string]string{"userId": input.TargetUserID})
	endpoint := c.baseURL + fmt.Sprintf(assignWorkItemPath, input.ItemType, input.ItemID)
	var payload RemoteWorkItem
	if err := c.doWorkspaceJSON(ctx, http.MethodPost, endpoint, input.TenantID, input.UserID, input.RequestID, body, &payload); err != nil {
		return nil, err
	}
	return &payload, nil
}

func (c *Client) UnassignWorkItem(ctx context.Context, input WorkItemMutationInput) (*RemoteWorkItem, *DependencyError) {
	endpoint := c.baseURL + fmt.Sprintf(unassignWorkItemPath, input.ItemType, input.ItemID)
	var payload RemoteWorkItem
	if err := c.doWorkspaceJSON(ctx, http.MethodPost, endpoint, input.TenantID, input.UserID, input.RequestID, nil, &payload); err != nil {
		return nil, err
	}
	return &payload, nil
}

func (c *Client) BulkWorkItemsAction(ctx context.Context, input BulkActionInput) (*BulkActionOutcome, *DependencyError) {
	bodyMap := map[string]any{"action": input.Action, "items": input.Items}
	if input.TargetUserID != nil {
		bodyMap["targetUserId"] = *input.TargetUserID
	}
	body, _ := json.Marshal(bodyMap)
	var payload BulkActionOutcome
	if err := c.doWorkspaceJSON(ctx, http.MethodPost, c.baseURL+bulkWorkItemsPath, input.TenantID, input.UserID, input.RequestID, body, &payload); err != nil {
		return nil, err
	}
	return &payload, nil
}

func (c *Client) GetWorkload(ctx context.Context, tenantID, userID, requestID string) (*RemoteWorkloadResponse, *DependencyError) {
	var payload RemoteWorkloadResponse
	if err := c.doWorkspaceJSON(ctx, http.MethodGet, c.baseURL+workloadPath, tenantID, userID, requestID, nil, &payload); err != nil {
		return nil, err
	}
	return &payload, nil
}

func (c *Client) ListViews(ctx context.Context, tenantID, userID, requestID, workspaceScope string) ([]RemoteSavedView, *DependencyError) {
	path := listViewsPath
	if strings.TrimSpace(workspaceScope) != "" {
		path += "?workspaceScope=" + url.QueryEscape(strings.TrimSpace(workspaceScope))
	}
	var payload struct {
		Items []RemoteSavedView `json:"items"`
	}
	if err := c.doWorkspaceJSON(ctx, http.MethodGet, c.baseURL+path, tenantID, userID, requestID, nil, &payload); err != nil {
		return nil, err
	}
	return payload.Items, nil
}

func (c *Client) CreateView(ctx context.Context, input CreateViewInput) (*RemoteSavedView, *DependencyError) {
	bodyMap := map[string]any{
		"name": input.Name, "scope": input.Scope, "filters": input.Filters, "sort": input.Sort,
	}
	if strings.TrimSpace(input.WorkspaceScope) != "" {
		bodyMap["workspaceScope"] = input.WorkspaceScope
	}
	body, _ := json.Marshal(bodyMap)
	var payload RemoteSavedView
	if err := c.doWorkspaceJSON(ctx, http.MethodPost, c.baseURL+listViewsPath, input.TenantID, input.UserID, input.RequestID, body, &payload); err != nil {
		return nil, err
	}
	return &payload, nil
}

func (c *Client) UpdateView(ctx context.Context, tenantID, userID, requestID, viewID string, patch map[string]any) (*RemoteSavedView, *DependencyError) {
	body, _ := json.Marshal(patch)
	var payload RemoteSavedView
	endpoint := c.baseURL + fmt.Sprintf(viewPath, viewID)
	if err := c.doWorkspaceJSON(ctx, http.MethodPatch, endpoint, tenantID, userID, requestID, body, &payload); err != nil {
		return nil, err
	}
	return &payload, nil
}

func (c *Client) DeleteView(ctx context.Context, tenantID, userID, requestID, viewID string) *DependencyError {
	endpoint := c.baseURL + fmt.Sprintf(viewPath, viewID)
	return c.doWorkspaceJSON(ctx, http.MethodDelete, endpoint, tenantID, userID, requestID, nil, nil)
}

func (c *Client) SetDefaultView(ctx context.Context, tenantID, userID, requestID, viewID string) *DependencyError {
	endpoint := c.baseURL + fmt.Sprintf(setDefaultViewPath, viewID)
	return c.doWorkspaceJSON(ctx, http.MethodPost, endpoint, tenantID, userID, requestID, nil, nil)
}

func (c *Client) CreateHandoff(ctx context.Context, input CreateHandoffInput) (map[string]any, *DependencyError) {
	body, _ := json.Marshal(map[string]any{
		"toUserId": input.ToUserID, "title": input.Title, "note": input.Note, "items": input.Items,
	})
	var payload map[string]any
	if err := c.doWorkspaceJSON(ctx, http.MethodPost, c.baseURL+listHandoffsPath, input.TenantID, input.UserID, input.RequestID, body, &payload); err != nil {
		return nil, err
	}
	return payload, nil
}

func (c *Client) ListHandoffs(ctx context.Context, tenantID, userID, requestID string, query url.Values) ([]RemoteHandoff, *DependencyError) {
	endpoint := c.baseURL + listHandoffsPath
	if encoded := query.Encode(); encoded != "" {
		endpoint += "?" + encoded
	}
	var payload struct {
		Items []RemoteHandoff `json:"items"`
	}
	if err := c.doWorkspaceJSON(ctx, http.MethodGet, endpoint, tenantID, userID, requestID, nil, &payload); err != nil {
		return nil, err
	}
	return payload.Items, nil
}

func (c *Client) GetHandoff(ctx context.Context, tenantID, userID, requestID, handoffID string) (*RemoteHandoff, *DependencyError) {
	endpoint := c.baseURL + fmt.Sprintf(handoffPath, handoffID)
	var payload RemoteHandoff
	if err := c.doWorkspaceJSON(ctx, http.MethodGet, endpoint, tenantID, userID, requestID, nil, &payload); err != nil {
		return nil, err
	}
	return &payload, nil
}

func (c *Client) doWorkspaceJSON(
	ctx context.Context,
	method, endpoint, tenantID, userID, requestID string,
	body []byte,
	out any,
) *DependencyError {
	return c.doWorkspaceJSONWithPermissions(ctx, method, endpoint, tenantID, userID, requestID, body, nil, out)
}

func (c *Client) doWorkspaceJSONWithPermissions(
	ctx context.Context,
	method, endpoint, tenantID, userID, requestID string,
	body []byte,
	permissions []string,
	out any,
) *DependencyError {
	if c == nil {
		return &DependencyError{Reason: ReasonUnknown, Err: fmt.Errorf("read model client is nil")}
	}
	start := time.Now()
	result := "SUCCESS"
	var reason FailureReason
	defer func() {
		c.metrics.ObserveRequest("WORKSPACE", result, string(reason), time.Since(start))
	}()

	var reader io.Reader
	if len(body) > 0 {
		reader = bytes.NewReader(body)
	}
	req, err := http.NewRequestWithContext(ctx, method, endpoint, reader)
	if err != nil {
		reason = ReasonUnknown
		result = "ERROR"
		return &DependencyError{Reason: reason, Err: err}
	}
	if len(body) > 0 {
		req.Header.Set("Content-Type", "application/json")
	}
	req.Header.Set("X-Tenant-ID", tenantID)
	if strings.TrimSpace(userID) != "" {
		req.Header.Set("X-User-ID", userID)
	}
	if len(permissions) > 0 {
		req.Header.Set("X-User-Permissions", strings.Join(permissions, ","))
	}
	if strings.TrimSpace(requestID) != "" {
		req.Header.Set(sharedmiddleware.RequestIDHeader, requestID)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		reason = classifyRequestError(ctx, err)
		result = "ERROR"
		return &DependencyError{Reason: reason, Err: err}
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		reason = classifyHTTPStatus(resp.StatusCode)
		result = "ERROR"
		_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 4096))
		return &DependencyError{Reason: reason, Status: resp.StatusCode}
	}
	if out == nil || method == http.MethodDelete {
		return nil
	}
	limited := io.LimitReader(resp.Body, c.maxBytes+1)
	raw, err := io.ReadAll(limited)
	if err != nil {
		reason = ReasonMalformedResponse
		result = "ERROR"
		return &DependencyError{Reason: reason, Err: err}
	}
	if int64(len(raw)) > c.maxBytes {
		reason = ReasonMalformedResponse
		result = "ERROR"
		return &DependencyError{Reason: reason, Err: fmt.Errorf("response exceeds size limit")}
	}
	if err := json.Unmarshal(raw, out); err != nil {
		reason = ReasonMalformedResponse
		result = "ERROR"
		return &DependencyError{Reason: reason, Err: err}
	}
	return nil
}

func setIf(q url.Values, key, value string) {
	if strings.TrimSpace(value) != "" {
		q.Set(key, strings.TrimSpace(value))
	}
}
