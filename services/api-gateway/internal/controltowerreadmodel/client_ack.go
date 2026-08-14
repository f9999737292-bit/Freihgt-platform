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
	acknowledgeCriticalEventPath = "/internal/v1/control-tower/critical-events/%s/acknowledge"
	lookupAcknowledgementsPath   = "/internal/v1/control-tower/critical-events/acknowledgements/lookup"
)

type acknowledgeRequestBody struct {
	ShipmentID string `json:"shipmentId"`
	EventType  string `json:"eventType"`
	OccurredAt string `json:"occurredAt"`
	Source     string `json:"source"`
}

type lookupRequestBody struct {
	EventIDs []string `json:"eventIds"`
}

type lookupResponseBody struct {
	Items []RemoteAcknowledgementLookupItem `json:"items"`
}

func (c *Client) AcknowledgeCriticalEvent(
	ctx context.Context,
	input AcknowledgeCriticalEventInput,
) (*RemoteAcknowledgement, *DependencyError) {
	if c == nil {
		return nil, &DependencyError{Reason: ReasonUnknown, Err: fmt.Errorf("read model client is nil")}
	}

	start := time.Now()
	result := "SUCCESS"
	var reason FailureReason
	defer func() {
		c.metrics.ObserveRequest("ACK", result, string(reason), time.Since(start))
	}()

	source := strings.TrimSpace(input.Source)
	if source == "" {
		source = "control-tower"
	}

	body, err := json.Marshal(acknowledgeRequestBody{
		ShipmentID: input.ShipmentID,
		EventType:  input.EventType,
		OccurredAt: input.OccurredAt.UTC().Format(time.RFC3339),
		Source:     source,
	})
	if err != nil {
		reason = ReasonUnknown
		result = "ERROR"
		return nil, &DependencyError{Reason: reason, Err: err}
	}

	endpoint := c.baseURL + fmt.Sprintf(acknowledgeCriticalEventPath, input.EventID)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		reason = ReasonUnknown
		result = "ERROR"
		return nil, &DependencyError{Reason: reason, Err: err}
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Tenant-ID", input.TenantID)
	req.Header.Set("X-User-ID", input.UserID)
	if strings.TrimSpace(input.RequestID) != "" {
		req.Header.Set(sharedmiddleware.RequestIDHeader, input.RequestID)
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

	payload, err := decodeRemoteAcknowledgement(resp.Body)
	if err != nil {
		reason = ReasonMalformedResponse
		result = "ERROR"
		return nil, &DependencyError{Reason: reason, Err: err}
	}
	return payload, nil
}

func (c *Client) LookupAcknowledgements(
	ctx context.Context,
	tenantID, requestID string,
	eventIDs []string,
) (map[string]RemoteAcknowledgementLookupItem, *DependencyError) {
	if c == nil {
		return nil, &DependencyError{Reason: ReasonUnknown, Err: fmt.Errorf("read model client is nil")}
	}
	if len(eventIDs) == 0 {
		return map[string]RemoteAcknowledgementLookupItem{}, nil
	}

	start := time.Now()
	result := "SUCCESS"
	var reason FailureReason
	defer func() {
		c.metrics.ObserveRequest("ACK_LOOKUP", result, string(reason), time.Since(start))
	}()

	body, err := json.Marshal(lookupRequestBody{EventIDs: eventIDs})
	if err != nil {
		reason = ReasonUnknown
		result = "ERROR"
		return nil, &DependencyError{Reason: reason, Err: err}
	}

	endpoint := c.baseURL + lookupAcknowledgementsPath
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

	limited := io.LimitReader(resp.Body, c.maxBytes+1)
	raw, err := io.ReadAll(limited)
	if err != nil {
		reason = ReasonMalformedResponse
		result = "ERROR"
		return nil, &DependencyError{Reason: reason, Err: err}
	}
	if int64(len(raw)) > c.maxBytes {
		reason = ReasonMalformedResponse
		result = "ERROR"
		return nil, &DependencyError{Reason: reason, Err: fmt.Errorf("response exceeds size limit")}
	}

	var payload struct {
		Items []struct {
			EventID              string `json:"eventId"`
			AcknowledgedAt       string `json:"acknowledgedAt"`
			AcknowledgedByUserID string `json:"acknowledgedByUserId"`
		} `json:"items"`
	}
	if err := json.Unmarshal(raw, &payload); err != nil {
		reason = ReasonMalformedResponse
		result = "ERROR"
		return nil, &DependencyError{Reason: reason, Err: err}
	}

	out := make(map[string]RemoteAcknowledgementLookupItem, len(payload.Items))
	for _, item := range payload.Items {
		acknowledgedAt, err := time.Parse(time.RFC3339, item.AcknowledgedAt)
		if err != nil {
			reason = ReasonMalformedResponse
			result = "ERROR"
			return nil, &DependencyError{Reason: reason, Err: err}
		}
		out[item.EventID] = RemoteAcknowledgementLookupItem{
			EventID:              item.EventID,
			AcknowledgedAt:       acknowledgedAt.UTC(),
			AcknowledgedByUserID: item.AcknowledgedByUserID,
		}
	}
	return out, nil
}

func decodeRemoteAcknowledgement(body io.Reader) (*RemoteAcknowledgement, error) {
	limited := io.LimitReader(body, 64*1024)
	raw, err := io.ReadAll(limited)
	if err != nil {
		return nil, err
	}

	var wire struct {
		EventID        string `json:"eventId"`
		ShipmentID     string `json:"shipmentId"`
		EventType      string `json:"eventType"`
		OccurredAt     string `json:"occurredAt"`
		Source         string `json:"source"`
		Status         string `json:"status"`
		AcknowledgedAt string `json:"acknowledgedAt"`
		AcknowledgedBy struct {
			UserID string `json:"userId"`
		} `json:"acknowledgedBy"`
	}
	if err := json.Unmarshal(raw, &wire); err != nil {
		return nil, err
	}

	occurredAt, err := time.Parse(time.RFC3339, wire.OccurredAt)
	if err != nil {
		return nil, fmt.Errorf("invalid occurredAt: %w", err)
	}
	acknowledgedAt, err := time.Parse(time.RFC3339, wire.AcknowledgedAt)
	if err != nil {
		return nil, fmt.Errorf("invalid acknowledgedAt: %w", err)
	}

	return &RemoteAcknowledgement{
		EventID:        wire.EventID,
		ShipmentID:     wire.ShipmentID,
		EventType:      wire.EventType,
		OccurredAt:     occurredAt.UTC(),
		Source:         wire.Source,
		Status:         wire.Status,
		AcknowledgedAt: acknowledgedAt.UTC(),
		AcknowledgedBy: wire.AcknowledgedBy,
	}, nil
}
