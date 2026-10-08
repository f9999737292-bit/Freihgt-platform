package source

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"math"
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
	endpoint := c.baseURL + "/internal/v1/analytics/operations-foundation-v2"
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
	return decodeSnapshot(body, tenantID)
}

type sourceDocument struct {
	TenantID                  *string          `json:"tenantId"`
	ShipmentTotal             *int64           `json:"shipmentTotal"`
	OnTimePickupDenominator   *int64           `json:"onTimePickupDenominator"`
	OnTimePickupNumerator     *int64           `json:"onTimePickupNumerator"`
	OnTimeDeliveryDenominator *int64           `json:"onTimeDeliveryDenominator"`
	OnTimeDeliveryNumerator   *int64           `json:"onTimeDeliveryNumerator"`
	ReturnCaseCount           *int64           `json:"returnCaseCount"`
	RedirectCaseCount         *int64           `json:"redirectCaseCount"`
	Carriers                  *[]sourceCarrier `json:"carriers"`
}

type sourceCarrier struct {
	CarrierCompanyID          *string `json:"carrierCompanyId"`
	OnTimePickupDenominator   *int64  `json:"onTimePickupDenominator"`
	OnTimePickupNumerator     *int64  `json:"onTimePickupNumerator"`
	OnTimeDeliveryDenominator *int64  `json:"onTimeDeliveryDenominator"`
	OnTimeDeliveryNumerator   *int64  `json:"onTimeDeliveryNumerator"`
}

func decodeSnapshot(body []byte, tenantID string) (kpi.Snapshot, error) {
	decoder := json.NewDecoder(bytes.NewReader(body))
	decoder.DisallowUnknownFields()
	var payload sourceDocument
	if err := decoder.Decode(&payload); err != nil {
		return kpi.Snapshot{}, &Error{Reason: "malformed", Err: err}
	}
	var extra struct{}
	if err := decoder.Decode(&extra); !errors.Is(err, io.EOF) {
		return kpi.Snapshot{}, &Error{Reason: "malformed", Err: err}
	}
	if payload.TenantID == nil || payload.ShipmentTotal == nil || payload.OnTimePickupDenominator == nil || payload.OnTimePickupNumerator == nil || payload.OnTimeDeliveryDenominator == nil || payload.OnTimeDeliveryNumerator == nil || payload.ReturnCaseCount == nil || payload.RedirectCaseCount == nil || payload.Carriers == nil {
		return kpi.Snapshot{}, &Error{Reason: "malformed"}
	}
	if *payload.TenantID != tenantID {
		return kpi.Snapshot{}, &Error{Reason: "wrong_tenant"}
	}
	if _, err := uuid.Parse(*payload.TenantID); err != nil {
		return kpi.Snapshot{}, &Error{Reason: "malformed", Err: err}
	}
	carriers, err := carriersFrom(*payload.Carriers, *payload.OnTimePickupDenominator, *payload.OnTimePickupNumerator, *payload.OnTimeDeliveryDenominator, *payload.OnTimeDeliveryNumerator)
	if err != nil {
		return kpi.Snapshot{}, err
	}
	if !consistent(*payload.ShipmentTotal, *payload.OnTimePickupDenominator, *payload.OnTimePickupNumerator, *payload.OnTimeDeliveryDenominator, *payload.OnTimeDeliveryNumerator, *payload.ReturnCaseCount, *payload.RedirectCaseCount) {
		return kpi.Snapshot{}, &Error{Reason: "inconsistent"}
	}
	return kpi.Snapshot{
		TenantID:                  *payload.TenantID,
		ShipmentTotal:             *payload.ShipmentTotal,
		OnTimePickupDenominator:   *payload.OnTimePickupDenominator,
		OnTimePickupNumerator:     *payload.OnTimePickupNumerator,
		OnTimeDeliveryDenominator: *payload.OnTimeDeliveryDenominator,
		OnTimeDeliveryNumerator:   *payload.OnTimeDeliveryNumerator,
		ReturnCaseCount:           *payload.ReturnCaseCount,
		RedirectCaseCount:         *payload.RedirectCaseCount,
		Carriers:                  carriers,
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

func carriersFrom(rows []sourceCarrier, pickupDenominator, pickupNumerator, deliveryDenominator, deliveryNumerator int64) ([]kpi.CarrierSnapshot, error) {
	carriers := make([]kpi.CarrierSnapshot, 0, len(rows))
	seen := make(map[string]struct{}, len(rows))
	var pickupDenominatorSum, pickupNumeratorSum, deliveryDenominatorSum, deliveryNumeratorSum int64
	for _, row := range rows {
		if row.CarrierCompanyID == nil || row.OnTimePickupDenominator == nil || row.OnTimePickupNumerator == nil || row.OnTimeDeliveryDenominator == nil || row.OnTimeDeliveryNumerator == nil {
			return nil, &Error{Reason: "malformed"}
		}
		id, err := uuid.Parse(*row.CarrierCompanyID)
		if err != nil {
			return nil, &Error{Reason: "malformed", Err: err}
		}
		if id == uuid.Nil {
			return nil, &Error{Reason: "inconsistent"}
		}
		canonical := id.String()
		if _, ok := seen[canonical]; ok {
			return nil, &Error{Reason: "inconsistent"}
		}
		seen[canonical] = struct{}{}
		if *row.OnTimePickupDenominator < 0 || *row.OnTimePickupNumerator < 0 || *row.OnTimeDeliveryDenominator < 0 || *row.OnTimeDeliveryNumerator < 0 || *row.OnTimePickupNumerator > *row.OnTimePickupDenominator || *row.OnTimeDeliveryNumerator > *row.OnTimeDeliveryDenominator {
			return nil, &Error{Reason: "inconsistent"}
		}
		pickupDenominatorSum, err = addCount(pickupDenominatorSum, *row.OnTimePickupDenominator)
		if err != nil {
			return nil, &Error{Reason: "inconsistent", Err: err}
		}
		pickupNumeratorSum, err = addCount(pickupNumeratorSum, *row.OnTimePickupNumerator)
		if err != nil {
			return nil, &Error{Reason: "inconsistent", Err: err}
		}
		deliveryDenominatorSum, err = addCount(deliveryDenominatorSum, *row.OnTimeDeliveryDenominator)
		if err != nil {
			return nil, &Error{Reason: "inconsistent", Err: err}
		}
		deliveryNumeratorSum, err = addCount(deliveryNumeratorSum, *row.OnTimeDeliveryNumerator)
		if err != nil {
			return nil, &Error{Reason: "inconsistent", Err: err}
		}
		carriers = append(carriers, kpi.CarrierSnapshot{
			CarrierCompanyID:          canonical,
			OnTimePickupDenominator:   *row.OnTimePickupDenominator,
			OnTimePickupNumerator:     *row.OnTimePickupNumerator,
			OnTimeDeliveryDenominator: *row.OnTimeDeliveryDenominator,
			OnTimeDeliveryNumerator:   *row.OnTimeDeliveryNumerator,
		})
	}
	if pickupDenominatorSum > pickupDenominator || pickupNumeratorSum > pickupNumerator || deliveryDenominatorSum > deliveryDenominator || deliveryNumeratorSum > deliveryNumerator {
		return nil, &Error{Reason: "inconsistent"}
	}
	return carriers, nil
}

func addCount(sum, next int64) (int64, error) {
	if next < 0 || sum > math.MaxInt64-next {
		return 0, errors.New("count overflow")
	}
	return sum + next, nil
}

func consistent(shipmentTotal, pickupDenominator, pickupNumerator, deliveryDenominator, deliveryNumerator, returns, redirects int64) bool {
	if shipmentTotal < 0 || pickupDenominator < 0 || pickupNumerator < 0 || deliveryDenominator < 0 || deliveryNumerator < 0 || returns < 0 || redirects < 0 {
		return false
	}
	if pickupNumerator > pickupDenominator || deliveryNumerator > deliveryDenominator {
		return false
	}
	return pickupDenominator <= shipmentTotal && deliveryDenominator <= shipmentTotal
}

func isTimeout(err error) bool {
	if errors.Is(err, context.DeadlineExceeded) {
		return true
	}
	var netErr net.Error
	return errors.As(err, &netErr) && netErr.Timeout()
}
