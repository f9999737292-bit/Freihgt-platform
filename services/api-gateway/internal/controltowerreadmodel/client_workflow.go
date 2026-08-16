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

type assignRequestBody struct {
	UserID string `json:"userId"`
}

type resolveRequestBody struct {
	ResolutionCode string  `json:"resolutionCode"`
	Comment        *string `json:"comment,omitempty"`
}

func (c *Client) AssignCriticalEvent(
	ctx context.Context,
	input AssignCriticalEventInput,
) (*RemoteWorkflow, *DependencyError) {
	body, err := json.Marshal(assignRequestBody{UserID: input.AssignedToUser})
	if err != nil {
		return nil, &DependencyError{Reason: ReasonUnknown, Err: err}
	}
	endpoint := c.baseURL + fmt.Sprintf(assignCriticalEventPath, input.EventID)
	return c.postWorkflowMutation(ctx, "ASSIGN", endpoint, input.TenantID, input.UserID, input.RequestID, body)
}

func (c *Client) ResolveCriticalEvent(
	ctx context.Context,
	input ResolveCriticalEventInput,
) (*RemoteWorkflow, *DependencyError) {
	body, err := json.Marshal(resolveRequestBody{
		ResolutionCode: input.ResolutionCode,
		Comment:        input.ResolutionComment,
	})
	if err != nil {
		return nil, &DependencyError{Reason: ReasonUnknown, Err: err}
	}
	endpoint := c.baseURL + fmt.Sprintf(resolveCriticalEventPath, input.EventID)
	return c.postWorkflowMutation(ctx, "RESOLVE", endpoint, input.TenantID, input.UserID, input.RequestID, body)
}

func (c *Client) ReopenCriticalEvent(
	ctx context.Context,
	input ReopenCriticalEventInput,
) (*RemoteWorkflow, *DependencyError) {
	endpoint := c.baseURL + fmt.Sprintf(reopenCriticalEventPath, input.EventID)
	return c.postWorkflowMutation(ctx, "REOPEN", endpoint, input.TenantID, input.UserID, input.RequestID, nil)
}

func (c *Client) ListCriticalEventActions(
	ctx context.Context,
	tenantID, userID, requestID, eventID string,
) ([]RemoteWorkflowAction, *DependencyError) {
	if c == nil {
		return nil, &DependencyError{Reason: ReasonUnknown, Err: fmt.Errorf("read model client is nil")}
	}

	start := time.Now()
	result := "SUCCESS"
	var reason FailureReason
	defer func() {
		c.metrics.ObserveRequest("WORKFLOW_ACTIONS", result, string(reason), time.Since(start))
	}()

	endpoint := c.baseURL + fmt.Sprintf(listCriticalEventActions, eventID)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		reason = ReasonUnknown
		result = "ERROR"
		return nil, &DependencyError{Reason: reason, Err: err}
	}
	req.Header.Set("X-Tenant-ID", tenantID)
	req.Header.Set("X-User-ID", userID)
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
		Items []RemoteWorkflowAction `json:"items"`
	}
	if err := decodeJSON(resp.Body, c.maxBytes, &payload); err != nil {
		reason = ReasonMalformedResponse
		result = "ERROR"
		return nil, &DependencyError{Reason: reason, Err: err}
	}
	return payload.Items, nil
}

func (c *Client) LookupWorkflows(
	ctx context.Context,
	tenantID, requestID string,
	eventIDs []string,
) (map[string]RemoteWorkflowLookupItem, *DependencyError) {
	if c == nil {
		return nil, &DependencyError{Reason: ReasonUnknown, Err: fmt.Errorf("read model client is nil")}
	}
	if len(eventIDs) == 0 {
		return map[string]RemoteWorkflowLookupItem{}, nil
	}

	start := time.Now()
	result := "SUCCESS"
	var reason FailureReason
	defer func() {
		c.metrics.ObserveRequest("WORKFLOW_LOOKUP", result, string(reason), time.Since(start))
	}()

	body, err := json.Marshal(map[string][]string{"eventIds": eventIDs})
	if err != nil {
		reason = ReasonUnknown
		result = "ERROR"
		return nil, &DependencyError{Reason: reason, Err: err}
	}

	endpoint := c.baseURL + lookupWorkflowsPath
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
			EventID string `json:"eventId"`
			Status  string `json:"status"`
			RemoteWorkflowSummary
			Exception RemoteExceptionDetails `json:"exception"`
		} `json:"items"`
	}
	if err := decodeJSON(resp.Body, c.maxBytes, &payload); err != nil {
		reason = ReasonMalformedResponse
		result = "ERROR"
		return nil, &DependencyError{Reason: reason, Err: err}
	}

	out := make(map[string]RemoteWorkflowLookupItem, len(payload.Items))
	for _, item := range payload.Items {
		lookup := RemoteWorkflowLookupItem{
			EventID:               item.EventID,
			Status:                item.Status,
			RemoteWorkflowSummary: item.RemoteWorkflowSummary,
			Exception:             item.Exception,
		}
		if item.Acknowledgement != nil {
			if ackAt, err := time.Parse(time.RFC3339, item.Acknowledgement.AcknowledgedAt); err == nil {
				lookup.AcknowledgedAt = ackAt.UTC()
			}
			lookup.AcknowledgedByUserID = item.Acknowledgement.UserID
		}
		out[item.EventID] = lookup
	}
	return out, nil
}

func (c *Client) postWorkflowMutation(
	ctx context.Context,
	metricLabel string,
	endpoint string,
	tenantID string,
	userID string,
	requestID string,
	body []byte,
) (*RemoteWorkflow, *DependencyError) {
	if c == nil {
		return nil, &DependencyError{Reason: ReasonUnknown, Err: fmt.Errorf("read model client is nil")}
	}

	start := time.Now()
	result := "SUCCESS"
	var reason FailureReason
	defer func() {
		c.metrics.ObserveRequest(metricLabel, result, string(reason), time.Since(start))
	}()

	var reader io.Reader
	if len(body) > 0 {
		reader = bytes.NewReader(body)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, reader)
	if err != nil {
		reason = ReasonUnknown
		result = "ERROR"
		return nil, &DependencyError{Reason: reason, Err: err}
	}
	if len(body) > 0 {
		req.Header.Set("Content-Type", "application/json")
	}
	req.Header.Set("X-Tenant-ID", tenantID)
	req.Header.Set("X-User-ID", userID)
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

	var payload RemoteWorkflow
	if err := decodeJSON(resp.Body, c.maxBytes, &payload); err != nil {
		reason = ReasonMalformedResponse
		result = "ERROR"
		return nil, &DependencyError{Reason: reason, Err: err}
	}
	return &payload, nil
}

func decodeJSON(body io.Reader, maxBytes int64, dest any) error {
	limited := io.LimitReader(body, maxBytes+1)
	raw, err := io.ReadAll(limited)
	if err != nil {
		return err
	}
	if int64(len(raw)) > maxBytes {
		return fmt.Errorf("response exceeds size limit")
	}
	return json.Unmarshal(raw, dest)
}
