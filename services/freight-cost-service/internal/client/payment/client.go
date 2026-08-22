package payment

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
	"github.com/shopspring/decimal"

	"github.com/freight-platform/freight-cost-service/internal/domain"
	apperrors "github.com/freight-platform/freight-cost-service/internal/platform/errors"
	fcmetrics "github.com/freight-platform/freight-cost-service/internal/platform/metrics"
	"github.com/freight-platform/shared-go/internalauth"
)

const sourceService = "payment-service"

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

type ObligationFact struct {
	ObligationID      uuid.UUID
	BillingRegisterID uuid.UUID
	TransportOrderID  uuid.UUID
	TenantID          uuid.UUID
	BuyerCompanyID    uuid.UUID
	CarrierCompanyID  uuid.UUID
	Version           int64
	PaidAmount        *decimal.Decimal
	OriginalAmount    *decimal.Decimal
	CurrencyCode      string
	Status            string
	UpdatedAt         time.Time
}

type obligationResponse struct {
	ObligationID      string  `json:"obligation_id"`
	BillingRegisterID string  `json:"billing_register_id"`
	TransportOrderID  string  `json:"transport_order_id"`
	TenantID          string  `json:"tenant_id"`
	BuyerCompanyID    string  `json:"buyer_company_id"`
	CarrierCompanyID  string  `json:"carrier_company_id"`
	Version           int64   `json:"version"`
	PaidAmount        *string `json:"paid_amount"`
	OriginalAmount    *string `json:"original_amount"`
	CurrencyCode      string  `json:"currency_code"`
	Status            string  `json:"status"`
	UpdatedAt         string  `json:"updated_at"`
}

func (c *Client) GetObligationByBillingRegister(ctx context.Context, tenantID, billingRegisterID uuid.UUID) (*ObligationFact, error) {
	if c == nil || c.baseURL == "" {
		return nil, apperrors.Unavailable("payment service url is not configured", errors.New("missing base url"))
	}
	url := fmt.Sprintf("%s/internal/v1/payment-obligations/by-billing-register/%s", c.baseURL, billingRegisterID)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, apperrors.Internal("create payment request", err)
	}
	req.Header.Set("X-Tenant-ID", tenantID.String())
	if c.token != "" {
		req.Header.Set(internalauth.HeaderName, c.token)
	}
	resp, err := c.client.Do(req)
	if err != nil {
		c.observe("get_obligation_by_register", "error", string(apperrors.CodeUnavailable))
		return nil, apperrors.Unavailable("payment service unavailable", err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		c.observe("get_obligation_by_register", "error", string(apperrors.CodeUnavailable))
		return nil, apperrors.Unavailable("payment service unavailable", err)
	}
	switch resp.StatusCode {
	case http.StatusOK:
		c.observe("get_obligation_by_register", "success", "")
	case http.StatusNotFound:
		c.observe("get_obligation_by_register", "not_found", string(apperrors.CodeNotFound))
		return nil, apperrors.NotFound("payment obligation not found")
	default:
		if resp.StatusCode >= 500 {
			c.observe("get_obligation_by_register", "error", string(apperrors.CodeUnavailable))
			return nil, apperrors.Unavailable("payment service unavailable", fmt.Errorf("status %d", resp.StatusCode))
		}
		c.observe("get_obligation_by_register", "error", string(apperrors.CodeBadGateway))
		return nil, apperrors.BadGateway("payment service returned unexpected status", fmt.Errorf("status %d", resp.StatusCode))
	}
	var payload obligationResponse
	if err := json.Unmarshal(body, &payload); err != nil {
		return nil, apperrors.BadGateway("invalid payment obligation response", err)
	}
	return mapObligation(payload, tenantID, billingRegisterID)
}

func mapObligation(payload obligationResponse, requestedTenantID, requestedRegisterID uuid.UUID) (*ObligationFact, error) {
	registerID, err := parseUUID("billing_register_id", payload.BillingRegisterID)
	if err != nil {
		return nil, err
	}
	if registerID != requestedRegisterID {
		return nil, apperrors.BadGateway("billing register mismatch", fmt.Errorf("expected %s", requestedRegisterID))
	}
	tenantID, err := parseUUID("tenant_id", payload.TenantID)
	if err != nil {
		return nil, err
	}
	if tenantID != requestedTenantID {
		return nil, apperrors.BadGateway("tenant mismatch", fmt.Errorf("expected %s", requestedTenantID))
	}
	obligationID, err := parseUUID("obligation_id", payload.ObligationID)
	if err != nil {
		return nil, err
	}
	transportOrderID, err := parseUUID("transport_order_id", payload.TransportOrderID)
	if err != nil {
		return nil, err
	}
	buyerID, err := parseUUID("buyer_company_id", payload.BuyerCompanyID)
	if err != nil {
		return nil, err
	}
	carrierID, err := parseUUID("carrier_company_id", payload.CarrierCompanyID)
	if err != nil {
		return nil, err
	}
	if err := domain.ValidateCurrencyCode(payload.CurrencyCode); err != nil {
		return nil, apperrors.BadGateway("invalid currency", err)
	}
	updatedAt, err := time.Parse(time.RFC3339, strings.TrimSpace(payload.UpdatedAt))
	if err != nil {
		return nil, apperrors.BadGateway("invalid updated_at", err)
	}
	fact := &ObligationFact{
		ObligationID:      obligationID,
		BillingRegisterID: registerID,
		TransportOrderID:  transportOrderID,
		TenantID:          tenantID,
		BuyerCompanyID:    buyerID,
		CarrierCompanyID:  carrierID,
		Version:           payload.Version,
		CurrencyCode:      strings.ToUpper(strings.TrimSpace(payload.CurrencyCode)),
		Status:            strings.TrimSpace(payload.Status),
		UpdatedAt:         updatedAt.UTC(),
	}
	if payload.PaidAmount != nil {
		amount, err := domain.ParseMoneyAmount(*payload.PaidAmount)
		if err != nil {
			return nil, apperrors.BadGateway("invalid paid_amount", err)
		}
		fact.PaidAmount = &amount
	}
	if payload.OriginalAmount != nil {
		amount, err := domain.ParseMoneyAmount(*payload.OriginalAmount)
		if err != nil {
			return nil, apperrors.BadGateway("invalid original_amount", err)
		}
		fact.OriginalAmount = &amount
	}
	return fact, nil
}

func parseUUID(field, raw string) (uuid.UUID, error) {
	id, err := uuid.Parse(strings.TrimSpace(raw))
	if err != nil || id == uuid.Nil {
		return uuid.Nil, apperrors.BadGateway(fmt.Sprintf("invalid %s", field), fmt.Errorf("value %q", raw))
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
