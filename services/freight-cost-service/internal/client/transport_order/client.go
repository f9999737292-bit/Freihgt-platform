package transport_order

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/freight-platform/freight-cost-service/internal/domain"
	apperrors "github.com/freight-platform/freight-cost-service/internal/platform/errors"
	fcmetrics "github.com/freight-platform/freight-cost-service/internal/platform/metrics"
	"github.com/freight-platform/freight-cost-service/internal/provider"
	"github.com/freight-platform/shared-go/internalauth"
)

const sourceService = "transport-order-service"

type Client struct {
	baseURL string
	token   string
	client  *http.Client
	metrics *fcmetrics.Metrics
}

func NewClient(baseURL, token string, metrics *fcmetrics.Metrics) *Client {
	return &Client{
		baseURL: strings.TrimRight(baseURL, "/"),
		token:   token,
		client:  &http.Client{Timeout: 10 * time.Second},
		metrics: metrics,
	}
}

type rateSnapshotResponse struct {
	TransportOrderID    string `json:"transport_order_id"`
	TenantID            string `json:"tenant_id"`
	BuyerCompanyID      string `json:"buyer_company_id"`
	CarrierCompanyID    string `json:"carrier_company_id"`
	SnapshotID          string `json:"snapshot_id"`
	CurrencyCode        string `json:"currency_code"`
	TotalAmount         string `json:"total_amount"`
	PricingSource       string `json:"pricing_source"`
	PricingModelVersion string `json:"pricing_model_version"`
	ResolvedAt          string `json:"resolved_at"`
}

func (c *Client) GetRateSnapshot(ctx context.Context, tenantID, transportOrderID uuid.UUID) (*provider.RateSnapshotFact, error) {
	const operation = "get_rate_snapshot"
	if c == nil || c.baseURL == "" {
		return nil, apperrors.Unavailable("transport order service url is not configured", fmt.Errorf("missing base url"))
	}

	url := fmt.Sprintf("%s/internal/v1/transport-orders/%s/rate-snapshot", c.baseURL, transportOrderID.String())
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, apperrors.Internal("create transport order request", err)
	}
	req.Header.Set("X-Tenant-ID", tenantID.String())
	if c.token != "" {
		req.Header.Set(internalauth.HeaderName, c.token)
	}

	resp, err := c.client.Do(req)
	if err != nil {
		c.observe(operation, "error", string(apperrors.CodeUnavailable))
		return nil, apperrors.Unavailable("transport order service unavailable", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		c.observe(operation, "error", string(apperrors.CodeUnavailable))
		return nil, apperrors.Unavailable("transport order service unavailable", err)
	}

	switch resp.StatusCode {
	case http.StatusOK:
	case http.StatusNotFound:
		c.observe(operation, "not_found", string(apperrors.CodeNotFound))
		return nil, apperrors.NotFound("transport order not found")
	case http.StatusConflict:
		c.observe(operation, "conflict", string(apperrors.CodeConflict))
		return nil, apperrors.Conflict("transport order is unpriced", map[string]any{"field": "pricing_model_version"})
	case http.StatusUnauthorized:
		c.observe(operation, "error", string(apperrors.CodeUnauthorized))
		return nil, apperrors.Unavailable("transport order service unauthorized", errors.New("downstream unauthorized"))
	default:
		if resp.StatusCode >= 500 {
			c.observe(operation, "error", string(apperrors.CodeUnavailable))
			return nil, apperrors.Unavailable("transport order service unavailable", fmt.Errorf("status %d", resp.StatusCode))
		}
		c.observe(operation, "error", string(apperrors.CodeBadGateway))
		return nil, apperrors.BadGateway("transport order service returned unexpected status", fmt.Errorf("status %d", resp.StatusCode))
	}

	var payload rateSnapshotResponse
	if err := json.Unmarshal(body, &payload); err != nil {
		c.observe(operation, "error", string(apperrors.CodeBadGateway))
		return nil, apperrors.BadGateway("invalid transport order response", err)
	}

	fact, err := mapRateSnapshot(payload, tenantID, transportOrderID)
	if err != nil {
		c.observe(operation, "error", string(apperrors.CodeBadGateway))
		return nil, err
	}
	c.observe(operation, "success", "")
	return fact, nil
}

func mapRateSnapshot(payload rateSnapshotResponse, requestedTenantID, requestedTransportOrderID uuid.UUID) (*provider.RateSnapshotFact, error) {
	transportOrderID, err := parseNonZeroUUID("transport_order_id", payload.TransportOrderID)
	if err != nil {
		return nil, err
	}
	if transportOrderID != requestedTransportOrderID {
		return nil, apperrors.BadGateway("downstream transport order id does not match request", fmt.Errorf("expected %s", requestedTransportOrderID))
	}

	tenantID, err := parseNonZeroUUID("tenant_id", payload.TenantID)
	if err != nil {
		return nil, err
	}
	if tenantID != requestedTenantID {
		return nil, apperrors.NotFound("transport order not found")
	}

	buyerCompanyID, err := parseNonZeroUUID("buyer_company_id", payload.BuyerCompanyID)
	if err != nil {
		return nil, err
	}
	carrierCompanyID, err := parseNonZeroUUID("carrier_company_id", payload.CarrierCompanyID)
	if err != nil {
		return nil, err
	}
	snapshotID, err := parseNonZeroUUID("snapshot_id", payload.SnapshotID)
	if err != nil {
		return nil, err
	}

	if strings.TrimSpace(payload.PricingModelVersion) != domain.PricingModelVersionSnapshot {
		return nil, apperrors.BadGateway("invalid pricing model version in downstream response", fmt.Errorf("got %q", payload.PricingModelVersion))
	}
	if strings.TrimSpace(payload.PricingSource) == "" {
		return nil, apperrors.BadGateway("missing pricing source in downstream response", fmt.Errorf("empty pricing_source"))
	}

	if err := domain.ValidateCurrencyCode(payload.CurrencyCode); err != nil {
		return nil, apperrors.BadGateway("invalid currency code in downstream response", err)
	}
	totalAmount, err := domain.ParseMoneyAmount(payload.TotalAmount)
	if err != nil {
		return nil, apperrors.BadGateway("invalid total amount in downstream response", err)
	}
	if totalAmount.IsNegative() {
		return nil, apperrors.BadGateway("negative total amount in downstream response", fmt.Errorf("amount %s", totalAmount))
	}

	resolvedAt, err := time.Parse(time.RFC3339, strings.TrimSpace(payload.ResolvedAt))
	if err != nil {
		return nil, apperrors.BadGateway("invalid resolved_at in downstream response", err)
	}

	return &provider.RateSnapshotFact{
		TransportOrderID:    transportOrderID,
		TenantID:            tenantID,
		BuyerCompanyID:      buyerCompanyID,
		CarrierCompanyID:    carrierCompanyID,
		SnapshotID:          snapshotID,
		CurrencyCode:        strings.ToUpper(strings.TrimSpace(payload.CurrencyCode)),
		TotalAmount:         totalAmount,
		PricingSource:       strings.TrimSpace(payload.PricingSource),
		PricingModelVersion: domain.PricingModelVersionSnapshot,
		ResolvedAt:          resolvedAt.UTC(),
	}, nil
}

func parseNonZeroUUID(field, raw string) (uuid.UUID, error) {
	id, err := uuid.Parse(strings.TrimSpace(raw))
	if err != nil || id == uuid.Nil {
		return uuid.Nil, apperrors.BadGateway(
			fmt.Sprintf("invalid %s in downstream response", field),
			fmt.Errorf("value %q", raw),
		)
	}
	return id, nil
}

func (c *Client) observe(operation, result, errorCode string) {
	if c.metrics == nil {
		return
	}
	c.metrics.ObserveSourceRequest(sourceService, operation, result)
	if errorCode != "" {
		c.metrics.ObserveSourceError(sourceService, errorCode)
	}
}
