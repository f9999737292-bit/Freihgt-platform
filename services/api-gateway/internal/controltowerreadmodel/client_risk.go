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
	syncRisksPath       = "/internal/v1/control-tower/risks/sync"
	listRisksPath       = "/internal/v1/control-tower/risks"
	riskKPIPath         = "/internal/v1/control-tower/risks/kpi"
	getRiskPath         = "/internal/v1/control-tower/risks/%s"
	acknowledgeRiskPath = "/internal/v1/control-tower/risks/%s/acknowledge"
	mitigateRiskPath    = "/internal/v1/control-tower/risks/%s/mitigate"
)

type SyncRiskSignal struct {
	Code           string         `json:"signalCode"`
	Severity       string         `json:"severity"`
	Weight         int            `json:"weight"`
	ObservedAt     string         `json:"observedAt"`
	Source         string         `json:"source"`
	Value          map[string]any `json:"value,omitempty"`
	ExplanationKey string         `json:"explanationKey"`
}

type SyncRiskEvaluation struct {
	RiskKey                string           `json:"riskKey"`
	ShipmentID             string           `json:"shipmentId"`
	PredictedExceptionType string           `json:"predictedExceptionType"`
	Score                  int              `json:"score"`
	RiskLevel              string           `json:"riskLevel"`
	EvaluatedAt            string           `json:"evaluatedAt"`
	NextEvaluationAt       string           `json:"nextEvaluationAt"`
	ThreatenedDeadlineAt   *string          `json:"threatenedDeadlineAt,omitempty"`
	SignalsHash            string           `json:"signalsHash"`
	Signals                []SyncRiskSignal `json:"signals"`
}

type MaterializeRiskInput struct {
	RiskKey        string `json:"riskKey"`
	ShipmentID     string `json:"shipmentId"`
	PredictedType  string `json:"predictedType"`
	ActualEventID  string `json:"actualEventId"`
	MaterializedAt string `json:"materializedAt"`
}

type SyncRisksInput struct {
	TenantID         string
	RequestID        string
	Evaluations      []SyncRiskEvaluation
	Materializations []MaterializeRiskInput
}

type RemoteRiskSignal struct {
	SignalCode     string         `json:"signalCode"`
	Severity       string         `json:"severity"`
	Weight         int            `json:"weight"`
	ObservedAt     string         `json:"observedAt"`
	Source         string         `json:"source"`
	Value          map[string]any `json:"value,omitempty"`
	ExplanationKey string         `json:"explanationKey"`
}

type RemoteRiskAction struct {
	ActionType  string         `json:"actionType"`
	ActorUserID *string        `json:"actorUserId,omitempty"`
	OccurredAt  string         `json:"occurredAt"`
	Metadata    map[string]any `json:"metadata,omitempty"`
}

type RemoteShipmentRisk struct {
	RiskID                 string             `json:"riskId"`
	ShipmentID             string             `json:"shipmentId"`
	PredictedExceptionType string             `json:"predictedExceptionType"`
	Score                  int                `json:"score"`
	Level                  string             `json:"level"`
	Status                 string             `json:"status"`
	FirstDetectedAt        string             `json:"firstDetectedAt"`
	EvaluatedAt            string             `json:"evaluatedAt"`
	NextEvaluationAt       *string            `json:"nextEvaluationAt,omitempty"`
	ThreatenedDeadlineAt   *string            `json:"threatenedDeadlineAt,omitempty"`
	ClearedAt              *string            `json:"clearedAt,omitempty"`
	ClearReason            *string            `json:"clearReason,omitempty"`
	MaterializedAt         *string            `json:"materializedAt,omitempty"`
	ActualEventID          *string            `json:"actualEventId,omitempty"`
	MitigationCode         *string            `json:"mitigationCode,omitempty"`
	MitigationComment      *string            `json:"mitigationComment,omitempty"`
	Signals                []RemoteRiskSignal `json:"signals"`
	Actions                []RemoteRiskAction `json:"actions,omitempty"`
}

type RemoteRiskKPI struct {
	ActiveRisks        int64 `json:"active"`
	CriticalRisks      int64 `json:"critical"`
	HighRisks          int64 `json:"high"`
	DeliveryDelayRisks int64 `json:"delivery_delay"`
	PickupDelayRisks   int64 `json:"pickup_delay"`
	SlotMissRisks      int64 `json:"slot_miss"`
	MitigatingRisks    int64 `json:"mitigating"`
	RisksCleared       int64 `json:"cleared"`
	RisksMaterialized  int64 `json:"materialized"`
}

type ListRisksFilter struct {
	Level             string
	Status            string
	PredictedType     string
	ShipmentID        string
	ActiveOnly        bool
	MitigatingOnly    bool
	NonMitigatingOnly bool
}

type MitigateRiskInput struct {
	TenantID          string
	UserID            string
	RequestID         string
	RiskKey           string
	MitigationCode    string
	MitigationComment *string
}

type AcknowledgeRiskInput struct {
	TenantID  string
	UserID    string
	RequestID string
	RiskKey   string
}

func (c *Client) SyncRisks(ctx context.Context, input SyncRisksInput) *DependencyError {
	if c == nil || len(input.Evaluations) == 0 && len(input.Materializations) == 0 {
		return nil
	}

	body, err := json.Marshal(map[string]any{
		"evaluations":      input.Evaluations,
		"materializations": input.Materializations,
		"clears":           []any{},
	})
	if err != nil {
		return &DependencyError{Reason: ReasonUnknown, Err: err}
	}

	endpoint := c.baseURL + syncRisksPath
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return &DependencyError{Reason: ReasonUnknown, Err: err}
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Tenant-ID", input.TenantID)
	if strings.TrimSpace(input.RequestID) != "" {
		req.Header.Set(sharedmiddleware.RequestIDHeader, input.RequestID)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return &DependencyError{Reason: classifyRequestError(ctx, err), Err: err}
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 4096))
		return &DependencyError{Reason: classifyHTTPStatus(resp.StatusCode), Status: resp.StatusCode}
	}
	return nil
}

func (c *Client) ListRisks(ctx context.Context, tenantID, requestID string, filter ListRisksFilter) ([]RemoteShipmentRisk, *DependencyError) {
	if c == nil {
		return nil, &DependencyError{Reason: ReasonUnknown, Err: fmt.Errorf("read model client is nil")}
	}

	endpoint := c.baseURL + listRisksPath
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, &DependencyError{Reason: ReasonUnknown, Err: err}
	}
	q := req.URL.Query()
	if filter.Level != "" {
		q.Set("level", filter.Level)
	}
	if filter.Status != "" {
		q.Set("status", filter.Status)
	}
	if filter.PredictedType != "" {
		q.Set("predictedType", filter.PredictedType)
	}
	if filter.ShipmentID != "" {
		q.Set("shipmentId", filter.ShipmentID)
	}
	if filter.ActiveOnly {
		q.Set("activeOnly", "true")
	}
	if filter.MitigatingOnly {
		q.Set("mitigatingOnly", "true")
	}
	req.URL.RawQuery = q.Encode()

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
		Items []RemoteShipmentRisk `json:"items"`
	}
	if err := decodeJSON(resp.Body, c.maxBytes, &payload); err != nil {
		return nil, &DependencyError{Reason: ReasonMalformedResponse, Err: err}
	}
	return payload.Items, nil
}

func (c *Client) GetRisk(ctx context.Context, tenantID, requestID, riskKey string) (*RemoteShipmentRisk, *DependencyError) {
	if c == nil {
		return nil, &DependencyError{Reason: ReasonUnknown, Err: fmt.Errorf("read model client is nil")}
	}

	endpoint := c.baseURL + fmt.Sprintf(getRiskPath, riskKey)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, &DependencyError{Reason: ReasonUnknown, Err: err}
	}
	req.Header.Set("X-Tenant-ID", tenantID)
	if strings.TrimSpace(requestID) != "" {
		req.Header.Set(sharedmiddleware.RequestIDHeader, requestID)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, &DependencyError{Reason: classifyRequestError(ctx, err), Err: err}
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusNotFound {
		return nil, &DependencyError{Reason: ReasonNon2XX, Status: resp.StatusCode}
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 4096))
		return nil, &DependencyError{Reason: classifyHTTPStatus(resp.StatusCode), Status: resp.StatusCode}
	}

	var payload RemoteShipmentRisk
	if err := decodeJSON(resp.Body, c.maxBytes, &payload); err != nil {
		return nil, &DependencyError{Reason: ReasonMalformedResponse, Err: err}
	}
	return &payload, nil
}

func (c *Client) GetRiskKPI(ctx context.Context, tenantID, requestID string) (*RemoteRiskKPI, *DependencyError) {
	if c == nil {
		return nil, &DependencyError{Reason: ReasonUnknown, Err: fmt.Errorf("read model client is nil")}
	}

	endpoint := c.baseURL + riskKPIPath
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, &DependencyError{Reason: ReasonUnknown, Err: err}
	}
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

	var payload RemoteRiskKPI
	if err := decodeJSON(resp.Body, c.maxBytes, &payload); err != nil {
		return nil, &DependencyError{Reason: ReasonMalformedResponse, Err: err}
	}
	return &payload, nil
}

func (c *Client) AcknowledgeRisk(ctx context.Context, input AcknowledgeRiskInput) (*RemoteShipmentRisk, *DependencyError) {
	endpoint := c.baseURL + fmt.Sprintf(acknowledgeRiskPath, input.RiskKey)
	return c.postRiskMutation(ctx, endpoint, input.TenantID, input.UserID, input.RequestID, nil)
}

func (c *Client) MitigateRisk(ctx context.Context, input MitigateRiskInput) (*RemoteShipmentRisk, *DependencyError) {
	body, err := json.Marshal(map[string]any{
		"mitigationCode": input.MitigationCode,
		"comment":        input.MitigationComment,
	})
	if err != nil {
		return nil, &DependencyError{Reason: ReasonUnknown, Err: err}
	}
	endpoint := c.baseURL + fmt.Sprintf(mitigateRiskPath, input.RiskKey)
	return c.postRiskMutation(ctx, endpoint, input.TenantID, input.UserID, input.RequestID, body)
}

func (c *Client) postRiskMutation(
	ctx context.Context,
	endpoint, tenantID, userID, requestID string,
	body []byte,
) (*RemoteShipmentRisk, *DependencyError) {
	if c == nil {
		return nil, &DependencyError{Reason: ReasonUnknown, Err: fmt.Errorf("read model client is nil")}
	}

	var reader io.Reader
	if body != nil {
		reader = bytes.NewReader(body)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, reader)
	if err != nil {
		return nil, &DependencyError{Reason: ReasonUnknown, Err: err}
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	req.Header.Set("X-Tenant-ID", tenantID)
	req.Header.Set("X-User-ID", userID)
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

	var payload RemoteShipmentRisk
	if err := decodeJSON(resp.Body, c.maxBytes, &payload); err != nil {
		return nil, &DependencyError{Reason: ReasonMalformedResponse, Err: err}
	}
	return &payload, nil
}

func FormatRiskTime(t time.Time) string {
	return t.UTC().Format(time.RFC3339)
}

func FormatRiskTimePtr(t *time.Time) *string {
	if t == nil {
		return nil
	}
	formatted := t.UTC().Format(time.RFC3339)
	return &formatted
}
