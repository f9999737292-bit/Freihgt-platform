package source

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/freight-platform/analytics-service/internal/config"
	"github.com/freight-platform/analytics-service/internal/kpi"
)

const callerName = "analytics-service"

type Error struct {
	Reason string
	Err    error
}

func (e *Error) Error() string {
	if e.Err != nil {
		return e.Reason + ": " + e.Err.Error()
	}
	return e.Reason
}

func (e *Error) Unwrap() error { return e.Err }

type Client struct {
	baseURL    string
	token      string
	httpClient *http.Client
}

func NewClient(cfg config.Config) *Client {
	timeout := cfg.SourceTimeout
	if timeout <= 0 {
		timeout = 5 * time.Second
	}
	return &Client{
		baseURL: strings.TrimRight(cfg.ShipmentURL, "/"),
		token:   cfg.InternalToken,
		httpClient: &http.Client{
			Timeout: timeout,
			CheckRedirect: func(*http.Request, []*http.Request) error {
				return errors.New("source redirect is not followed")
			},
		},
	}
}

func (c *Client) Fetch(ctx context.Context, tenantID string) (kpi.Snapshot, error) {
	if c == nil || c.baseURL == "" {
		return kpi.Snapshot{}, &Error{Reason: "unreachable"}
	}
	endpoint := c.baseURL + "/internal/v1/analytics/operations-foundation"
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return kpi.Snapshot{}, &Error{Reason: "unreachable", Err: err}
	}
	req.Header.Set("X-Internal-Service-Token", c.token)
	req.Header.Set("X-Internal-Service-Name", callerName)
	req.Header.Set("X-Tenant-ID", tenantID)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		reason := "unreachable"
		if isTimeout(err) {
			reason = "timeout"
		}
		return kpi.Snapshot{}, &Error{Reason: reason, Err: err}
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return kpi.Snapshot{}, &Error{Reason: "malformed", Err: err}
	}
	if resp.StatusCode != http.StatusOK {
		reason := "http_status"
		if resp.StatusCode >= 500 {
			reason = "http_5xx"
		}
		return kpi.Snapshot{}, &Error{Reason: reason}
	}
	var payload struct {
		TenantID                  string `json:"tenantId"`
		ShipmentTotal             int64  `json:"shipmentTotal"`
		OnTimeDeliveryDenominator int64  `json:"onTimeDeliveryDenominator"`
		OnTimeDeliveryNumerator   int64  `json:"onTimeDeliveryNumerator"`
		ReturnCaseCount           int64  `json:"returnCaseCount"`
		RedirectCaseCount         int64  `json:"redirectCaseCount"`
	}
	decoder := json.NewDecoder(strings.NewReader(string(body)))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&payload); err != nil {
		return kpi.Snapshot{}, &Error{Reason: "malformed", Err: err}
	}
	if payload.TenantID != tenantID {
		return kpi.Snapshot{}, &Error{Reason: "wrong_tenant"}
	}
	if _, err := uuid.Parse(payload.TenantID); err != nil {
		return kpi.Snapshot{}, &Error{Reason: "malformed", Err: err}
	}
	if !consistent(payload.ShipmentTotal, payload.OnTimeDeliveryDenominator, payload.OnTimeDeliveryNumerator, payload.ReturnCaseCount, payload.RedirectCaseCount) {
		return kpi.Snapshot{}, &Error{Reason: "inconsistent"}
	}
	return kpi.Snapshot{
		TenantID:                  payload.TenantID,
		ShipmentTotal:             payload.ShipmentTotal,
		OnTimeDeliveryDenominator: payload.OnTimeDeliveryDenominator,
		OnTimeDeliveryNumerator:   payload.OnTimeDeliveryNumerator,
		ReturnCaseCount:           payload.ReturnCaseCount,
		RedirectCaseCount:         payload.RedirectCaseCount,
	}, nil
}

func (c *Client) Ready(ctx context.Context) error {
	if c == nil || c.baseURL == "" {
		return &Error{Reason: "unreachable"}
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+"/ready", nil)
	if err != nil {
		return &Error{Reason: "unreachable", Err: err}
	}
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return &Error{Reason: "unreachable", Err: err}
	}
	defer resp.Body.Close()
	_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 1<<20))
	if resp.StatusCode != http.StatusOK {
		return &Error{Reason: "not_ready"}
	}
	return nil
}

func consistent(shipmentTotal, denominator, numerator, returns, redirects int64) bool {
	if shipmentTotal < 0 || denominator < 0 || numerator < 0 || returns < 0 || redirects < 0 {
		return false
	}
	return numerator <= denominator && denominator <= shipmentTotal
}

func isTimeout(err error) bool {
	if errors.Is(err, context.DeadlineExceeded) {
		return true
	}
	var netErr net.Error
	return errors.As(err, &netErr) && netErr.Timeout()
}
