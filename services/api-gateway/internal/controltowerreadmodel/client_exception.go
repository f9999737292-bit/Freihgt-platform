package controltowerreadmodel

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	sharedmiddleware "github.com/freight-platform/shared-go/middleware"
)

const (
	ensureExceptionWorkflowsPath   = "/internal/v1/control-tower/critical-events/workflows/ensure"
	listOpenWorkflowsBySourcePath  = "/internal/v1/control-tower/critical-events/workflows/list-open"
	updateExceptionPath            = "/internal/v1/control-tower/critical-events/%s/exception"
)

type EnsureExceptionSeed struct {
	EventID    string
	ShipmentID string
	EventType  string
	Source     string
	OccurredAt time.Time
	Severity   string
}

type UpdateExceptionInput struct {
	TenantID       string
	UserID         string
	RequestID      string
	EventID        string
	Priority       *string
	Category       *string
	BusinessImpact *string
}

type RemoteExceptionDetails struct {
	Priority          string             `json:"priority"`
	ExceptionCategory string             `json:"exceptionCategory"`
	BusinessImpact    string             `json:"businessImpact"`
	SLA               RemoteExceptionSLA `json:"sla"`
	Escalation        RemoteEscalation   `json:"escalation"`
}

type RemoteExceptionSLA struct {
	Phase            string `json:"phase"`
	Status           string `json:"status"`
	AcknowledgeDueAt string `json:"acknowledgeDueAt"`
	AssignmentDueAt  string `json:"assignmentDueAt"`
	ResolutionDueAt  string `json:"resolutionDueAt"`
	RemainingSeconds *int64 `json:"remainingSeconds,omitempty"`
}

type RemoteEscalation struct {
	Level string `json:"level"`
}

func (c *Client) EnsureExceptionWorkflows(
	ctx context.Context,
	tenantID, requestID string,
	seeds []EnsureExceptionSeed,
) ([]string, *DependencyError) {
	if c == nil || len(seeds) == 0 {
		return nil, nil
	}

	events := make([]map[string]string, 0, len(seeds))
	for _, seed := range seeds {
		events = append(events, map[string]string{
			"eventId":    seed.EventID,
			"shipmentId": seed.ShipmentID,
			"eventType":  seed.EventType,
			"source":     seed.Source,
			"occurredAt": seed.OccurredAt.UTC().Format(time.RFC3339),
			"severity":   seed.Severity,
		})
	}
	body, err := json.Marshal(map[string]any{"events": events})
	if err != nil {
		return nil, &DependencyError{Reason: ReasonUnknown, Err: err}
	}

	endpoint := c.baseURL + ensureExceptionWorkflowsPath
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return nil, &DependencyError{Reason: ReasonUnknown, Err: err}
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Tenant-ID", tenantID)
	if strings.TrimSpace(requestID) != "" {
		req.Header.Set(sharedmiddleware.RequestIDHeader, requestID)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, &DependencyError{Reason: classifyRequestError(ctx, err), Err: err}
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 4096))
		return nil, &DependencyError{Reason: classifyHTTPStatus(resp.StatusCode), Status: resp.StatusCode}
	}
	var payload struct {
		CreatedEventIDs []string `json:"createdEventIds"`
	}
	if err := decodeJSON(resp.Body, c.maxBytes, &payload); err != nil {
		return nil, &DependencyError{Reason: ReasonMalformedResponse, Err: err}
	}
	return payload.CreatedEventIDs, nil
}

func (c *Client) UpdateException(
	ctx context.Context,
	input UpdateExceptionInput,
) (*RemoteWorkflow, *DependencyError) {
	body, err := json.Marshal(map[string]any{
		"priority":       input.Priority,
		"category":       input.Category,
		"businessImpact": input.BusinessImpact,
	})
	if err != nil {
		return nil, &DependencyError{Reason: ReasonUnknown, Err: err}
	}
	endpoint := c.baseURL + fmt.Sprintf(updateExceptionPath, input.EventID)
	req, err := http.NewRequestWithContext(ctx, http.MethodPatch, endpoint, bytes.NewReader(body))
	if err != nil {
		return nil, &DependencyError{Reason: ReasonUnknown, Err: err}
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Tenant-ID", input.TenantID)
	req.Header.Set("X-User-ID", input.UserID)
	if strings.TrimSpace(input.RequestID) != "" {
		req.Header.Set(sharedmiddleware.RequestIDHeader, input.RequestID)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, &DependencyError{Reason: classifyRequestError(ctx, err), Err: err}
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 4096))
		return nil, &DependencyError{Reason: classifyHTTPStatus(resp.StatusCode), Status: resp.StatusCode}
	}
	var payload RemoteWorkflow
	if err := decodeJSON(resp.Body, c.maxBytes, &payload); err != nil {
		return nil, &DependencyError{Reason: ReasonMalformedResponse, Err: err}
	}
	return &payload, nil
}

type RemoteOpenWorkflow struct {
	EventID           string
	ShipmentID        string
	EventType         string
	Source            string
	OccurredAt        time.Time
	Status            string
	Priority          string
	ExceptionCategory string
	BusinessImpact    string
}

func (c *Client) ListOpenWorkflowsBySource(
	ctx context.Context,
	tenantID, requestID, source string,
) ([]RemoteOpenWorkflow, *DependencyError) {
	if c == nil {
		return nil, &DependencyError{Reason: ReasonUnknown, Err: fmt.Errorf("read model client is nil")}
	}

	start := time.Now()
	result := "SUCCESS"
	var reason FailureReason
	defer func() {
		c.metrics.ObserveRequest("WORKFLOW_LIST_OPEN", result, string(reason), time.Since(start))
	}()

	body, err := json.Marshal(map[string]string{"source": strings.TrimSpace(source)})
	if err != nil {
		reason = ReasonUnknown
		result = "ERROR"
		return nil, &DependencyError{Reason: reason, Err: err}
	}

	endpoint := c.baseURL + listOpenWorkflowsBySourcePath
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		reason = ReasonUnknown
		result = "ERROR"
		return nil, &DependencyError{Reason: reason, Err: err}
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Tenant-ID", tenantID)
	if strings.TrimSpace(requestID) != "" {
		req.Header.Set(sharedmiddleware.RequestIDHeader, requestID)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		reason = classifyRequestError(ctx, err)
		result = "ERROR"
		return nil, &DependencyError{Reason: reason, Err: err}
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		reason = classifyHTTPStatus(resp.StatusCode)
		result = "ERROR"
		_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 4096))
		return nil, &DependencyError{Reason: reason, Status: resp.StatusCode}
	}

	var payload struct {
		Items []struct {
			EventID           string `json:"eventId"`
			ShipmentID        string `json:"shipmentId"`
			EventType         string `json:"eventType"`
			Source            string `json:"source"`
			OccurredAt        string `json:"occurredAt"`
			Status            string `json:"status"`
			Priority          string `json:"priority"`
			ExceptionCategory string `json:"exceptionCategory"`
			BusinessImpact    string `json:"businessImpact"`
		} `json:"items"`
	}
	if err := decodeJSON(resp.Body, c.maxBytes, &payload); err != nil {
		reason = ReasonMalformedResponse
		result = "ERROR"
		return nil, &DependencyError{Reason: reason, Err: err}
	}

	out := make([]RemoteOpenWorkflow, 0, len(payload.Items))
	for _, item := range payload.Items {
		occurredAt, _ := time.Parse(time.RFC3339, item.OccurredAt)
		out = append(out, RemoteOpenWorkflow{
			EventID:           item.EventID,
			ShipmentID:        item.ShipmentID,
			EventType:         item.EventType,
			Source:            item.Source,
			OccurredAt:        occurredAt.UTC(),
			Status:            item.Status,
			Priority:          item.Priority,
			ExceptionCategory: item.ExceptionCategory,
			BusinessImpact:    item.BusinessImpact,
		})
	}
	return out, nil
}
