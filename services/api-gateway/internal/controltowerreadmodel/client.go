package controltowerreadmodel

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

const statusSummaryPath = "/internal/v1/control-tower/status-summary"

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

func (c *Client) FetchStatusSummary(ctx context.Context, mode Mode, tenantID, requestID string) (*RemoteStatusSummary, *DependencyError) {
	if mode == ModeDisabled {
		return nil, nil
	}
	start := time.Now()
	result := "success"
	var reason FailureReason

	defer func() {
		c.metrics.ObserveRequest(string(mode), result, string(reason), time.Since(start))
	}()

	endpoint := c.baseURL + statusSummaryPath
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		reason = ReasonUnknown
		result = "error"
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
		reason = classifyHTTPStatus(resp.StatusCode)
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

	var payload RemoteStatusSummary
	if err := json.Unmarshal(body, &payload); err != nil {
		reason = ReasonMalformedResponse
		result = "error"
		return nil, &DependencyError{Reason: reason, Err: err}
	}
	if err := validateStatusSummary(&payload); err != nil {
		reason = ReasonInvalidContract
		result = "error"
		return nil, &DependencyError{Reason: reason, Err: err}
	}
	return &payload, nil
}

func validateStatusSummary(payload *RemoteStatusSummary) error {
	if payload == nil {
		return fmt.Errorf("empty payload")
	}
	if payload.TotalShipments < 0 {
		return fmt.Errorf("totalShipments must be >= 0")
	}
	if payload.IncompleteProjections < 0 {
		return fmt.Errorf("incompleteProjections must be >= 0")
	}
	if payload.IncompleteProjections > payload.TotalShipments {
		return fmt.Errorf("incompleteProjections exceeds totalShipments")
	}
	if payload.ByStatus == nil {
		payload.ByStatus = map[string]int64{}
	}
	var sum int64
	for status, count := range payload.ByStatus {
		if !IsKnownShipmentStatus(status) {
			return fmt.Errorf("unknown status key %q", status)
		}
		if count < 0 {
			return fmt.Errorf("negative count for status %q", status)
		}
		sum += count
	}
	if sum > payload.TotalShipments {
		return fmt.Errorf("status counts exceed totalShipments")
	}
	return nil
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

func legacyFromInput(input LegacyStatusInput) StatusSummary {
	byStatus := map[string]int64{}
	var counted int64
	for k, v := range input.ByStatus {
		byStatus[k] = v
		counted += v
	}
	if input.CountedShipments > 0 {
		counted = input.CountedShipments
	}
	return StatusSummary{
		TotalShipments:        input.TotalShipments,
		CountedShipments:      counted,
		ByStatus:              byStatus,
		IncompleteProjections: 0,
		Source:                SourceLegacy,
		LimitedDataset:        input.LimitedDataset,
	}
}

func readModelFromInternal(payload *RemoteStatusSummary) StatusSummary {
	return StatusSummary{
		TotalShipments:        payload.TotalShipments,
		CountedShipments:      payload.TotalShipments,
		ByStatus:              cloneCounts(payload.ByStatus),
		IncompleteProjections: payload.IncompleteProjections,
		Source:                SourceReadModel,
		LimitedDataset:        false,
	}
}

func cloneCounts(in map[string]int64) map[string]int64 {
	out := make(map[string]int64, len(in))
	for k, v := range in {
		out[k] = v
	}
	return out
}
