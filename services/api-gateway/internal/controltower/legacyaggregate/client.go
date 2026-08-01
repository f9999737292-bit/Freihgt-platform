package legacyaggregate

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	sharedmiddleware "github.com/freight-platform/shared-go/middleware"
)

const statusSummaryPath = "/internal/v1/shipments/status-summary"

type Config struct {
	BaseURL          string
	Timeout          time.Duration
	MaxResponseBytes int64
}

type Summary struct {
	TotalShipments   int64
	CountedShipments int64
	ByStatus         map[string]int64
	Complete         bool
}

type FailureReason string

const (
	ReasonTimeout                       FailureReason = "TIMEOUT"
	ReasonNetworkError                  FailureReason = "NETWORK_ERROR"
	ReasonNon2XX                        FailureReason = "NON_2XX"
	ReasonMalformedResponse             FailureReason = "MALFORMED_RESPONSE"
	ReasonInvalidContract               FailureReason = "INVALID_CONTRACT"
	ReasonIncomplete                    FailureReason = "FULL_LEGACY_AGGREGATE_INCOMPLETE"
	ReasonCancelled                     FailureReason = "CANCELLED"
	ReasonUnknown                       FailureReason = "UNKNOWN"
	FallbackReasonFullLegacyUnavailable               = "FULL_LEGACY_AGGREGATE_UNAVAILABLE"
	FallbackReasonFullLegacyIncomplete                = "FULL_LEGACY_AGGREGATE_INCOMPLETE"
)

type DependencyError struct {
	Reason FailureReason
	Status int
	Err    error
}

func (e *DependencyError) Error() string {
	if e == nil {
		return ""
	}
	if e.Err != nil {
		return fmt.Sprintf("%s: %v", e.Reason, e.Err)
	}
	return string(e.Reason)
}

type Client struct {
	httpClient *http.Client
	baseURL    string
	maxBytes   int64
	metrics    *Metrics
}

func NewClient(httpClient *http.Client, cfg Config, metrics *Metrics) *Client {
	if httpClient == nil {
		httpClient = &http.Client{Timeout: cfg.Timeout}
	}
	if metrics == nil {
		metrics = NewMetrics()
	}
	maxBytes := cfg.MaxResponseBytes
	if maxBytes <= 0 {
		maxBytes = 256 * 1024
	}
	return &Client{
		httpClient: httpClient,
		baseURL:    strings.TrimRight(cfg.BaseURL, "/"),
		maxBytes:   maxBytes,
		metrics:    metrics,
	}
}

type remoteSummary struct {
	TotalShipments   int64            `json:"totalShipments"`
	CountedShipments int64            `json:"countedShipments"`
	ByStatus         map[string]int64 `json:"byStatus"`
	Complete         bool             `json:"complete"`
}

func (c *Client) FetchStatusSummary(ctx context.Context, mode, tenantID, requestID string) (*Summary, *DependencyError) {
	start := time.Now()
	result := "success"
	reason := ReasonUnknown
	fallbackLevel := FallbackLevelFullAggregate

	defer func() {
		c.metrics.ObserveRequest(mode, result, string(reason), fallbackLevel, time.Since(start))
	}()

	endpoint := c.baseURL + statusSummaryPath
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		reason = ReasonUnknown
		result = "error"
		c.metrics.ObserveError(result, string(reason))
		return nil, &DependencyError{Reason: reason, Err: err}
	}
	req.Header.Set("X-Tenant-ID", tenantID)
	if strings.TrimSpace(requestID) != "" {
		req.Header.Set(sharedmiddleware.RequestIDHeader, requestID)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		reason = classifyRequestError(ctx, err)
		result = "error"
		return nil, &DependencyError{Reason: reason, Err: err}
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		reason = ReasonNon2XX
		result = "error"
		_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 4096))
		return nil, &DependencyError{Reason: reason, Status: resp.StatusCode}
	}

	limited := io.LimitReader(resp.Body, c.maxBytes+1)
	body, err := io.ReadAll(limited)
	if err != nil {
		reason = ReasonMalformedResponse
		result = "error"
		return nil, &DependencyError{Reason: reason, Err: err}
	}
	if int64(len(body)) > c.maxBytes {
		reason = ReasonMalformedResponse
		result = "error"
		return nil, &DependencyError{Reason: reason, Err: fmt.Errorf("response exceeds size limit")}
	}

	var payload remoteSummary
	if err := json.Unmarshal(body, &payload); err != nil {
		reason = ReasonMalformedResponse
		result = "error"
		return nil, &DependencyError{Reason: reason, Err: err}
	}
	contract := aggregateFromRemote(&payload)
	if err := ValidateAggregateContract(contract); err != nil {
		reason = ReasonInvalidContract
		result = "error"
		c.metrics.ObserveError(result, string(reason))
		return nil, &DependencyError{Reason: reason, Err: err}
	}
	if err := ValidateCompleteLegacyAggregate(contract); err != nil {
		reason = ReasonIncomplete
		result = "error"
		c.metrics.ObserveError(result, string(reason))
		return nil, &DependencyError{Reason: reason, Err: err}
	}

	summary := &Summary{
		TotalShipments:   payload.TotalShipments,
		CountedShipments: payload.CountedShipments,
		ByStatus:         cloneCounts(payload.ByStatus),
		Complete:         true,
	}
	fallbackLevel = FallbackLevelFullAggregate
	return summary, nil
}

func aggregateFromRemote(payload *remoteSummary) AggregateSummary {
	if payload == nil {
		return AggregateSummary{}
	}
	byStatus := payload.ByStatus
	if byStatus == nil {
		byStatus = map[string]int64{}
	}
	return AggregateSummary{
		TotalShipments:   payload.TotalShipments,
		CountedShipments: payload.CountedShipments,
		ByStatus:         byStatus,
		Complete:         payload.Complete,
	}
}

func classifyRequestError(ctx context.Context, err error) FailureReason {
	if errors.Is(ctx.Err(), context.Canceled) || errors.Is(err, context.Canceled) {
		return ReasonCancelled
	}
	if errors.Is(ctx.Err(), context.DeadlineExceeded) || errors.Is(err, context.DeadlineExceeded) {
		return ReasonTimeout
	}
	return ReasonNetworkError
}

func cloneCounts(in map[string]int64) map[string]int64 {
	out := make(map[string]int64, len(in))
	for k, v := range in {
		out[k] = v
	}
	return out
}
