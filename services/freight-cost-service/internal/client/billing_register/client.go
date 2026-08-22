package billing_register

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

const sourceService = "billing-register-service"

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

type ApprovedAccessorialFact struct {
	AccessorialID uuid.UUID
	ChargeCode    string
	Amount        decimal.Decimal
}

type SettlementFact struct {
	SettlementID        uuid.UUID
	TransportOrderID    uuid.UUID
	TenantID            uuid.UUID
	BuyerCompanyID      uuid.UUID
	CarrierCompanyID    uuid.UUID
	ShipmentID          *uuid.UUID
	Status              string
	OpenDisputeCount    int
	Version             int64
	BillingLinkRevision int64
	BillingLinkState    string
	CurrencyCode                    string
	BaseFreightAmount               *decimal.Decimal
	AccrualAmountExVAT              *decimal.Decimal
	TotalWithoutVAT                 *decimal.Decimal
	ProposedAccessorialTotalExVAT   *decimal.Decimal
	ProposedAccessorialSourceStatus string
	ApprovedAccessorials            []ApprovedAccessorialFact
	RateSnapshotID                  *uuid.UUID
	UpdatedAt                       time.Time
}

type BillingLinkFact struct {
	SettlementID        uuid.UUID
	TransportOrderID    uuid.UUID
	TenantID            uuid.UUID
	BillingLinkRevision int64
	BillingLinkState    string
	BillingRegisterID   *uuid.UUID
	AmountExVAT         *decimal.Decimal
	CurrencyCode        string
	TaxBasis            domain.TaxBasis
}

type RegisterPayableFact struct {
	BillingRegisterID uuid.UUID
	TenantID          uuid.UUID
	Version           int64
	TotalWithVAT      decimal.Decimal
	CurrencyCode      string
	UpdatedAt         time.Time
}

type settlementResponse struct {
	SettlementID        string  `json:"settlement_id"`
	TransportOrderID    string  `json:"transport_order_id"`
	TenantID            string  `json:"tenant_id"`
	BuyerCompanyID      string  `json:"buyer_company_id"`
	CarrierCompanyID    string  `json:"carrier_company_id"`
	ShipmentID          string  `json:"shipment_id"`
	Status              string  `json:"status"`
	OpenDisputeCount    int     `json:"open_dispute_count"`
	Version             int     `json:"version"`
	BillingLinkRevision int64   `json:"billing_link_revision"`
	BillingLinkState    string  `json:"billing_link_state"`
	CurrencyCode        string  `json:"currency_code"`
	BaseFreightAmount   string  `json:"base_freight_amount"`
	AccrualAmountExVAT              string  `json:"accrual_amount_ex_vat"`
	TotalWithoutVAT                 string  `json:"total_without_vat"`
	ProposedAccessorialTotalExVAT   string  `json:"proposed_accessorial_total_ex_vat"`
	ProposedAccessorialSourceStatus string  `json:"proposed_accessorial_source_status"`
	ApprovedAccessorials            []approvedAccessorialResponse `json:"approved_accessorials"`
	RateSnapshotID      *string `json:"rate_snapshot_id,omitempty"`
	UpdatedAt           string  `json:"updated_at"`
}

type approvedAccessorialResponse struct {
	AccessorialID string `json:"accessorial_id"`
	ChargeCode    string `json:"charge_code"`
	AmountExVAT   string `json:"amount_ex_vat"`
}

type billingLinkResponse struct {
	SettlementID        string  `json:"settlement_id"`
	TenantID            string  `json:"tenant_id"`
	TransportOrderID    string  `json:"transport_order_id"`
	BillingLinkRevision int64   `json:"billing_link_revision"`
	BillingLinkState    string  `json:"billing_link_state"`
	BillingRegisterID   *string `json:"billing_register_id,omitempty"`
	AmountExVAT         *string `json:"amount_ex_vat"`
	CurrencyCode        string  `json:"currency_code"`
	TaxBasis            string  `json:"tax_basis"`
}

type registerPayableResponse struct {
	RegisterID   string `json:"register_id"`
	TenantID     string `json:"tenant_id"`
	Status       string `json:"status"`
	Version      int64  `json:"version"`
	TotalWithVAT string `json:"total_with_vat"`
	CurrencyCode string `json:"currency_code"`
	TaxBasis     string `json:"tax_basis"`
	UpdatedAt    string `json:"updated_at"`
}

func (c *Client) GetSettlementByTransportOrder(ctx context.Context, tenantID, transportOrderID uuid.UUID) (*SettlementFact, error) {
	url := fmt.Sprintf("%s/internal/v1/freight-settlements/by-transport-order/%s", c.baseURL, transportOrderID)
	body, status, err := c.doGet(ctx, tenantID, url, "get_settlement_by_transport_order")
	if err != nil {
		return nil, err
	}
	if status == http.StatusNotFound {
		return nil, apperrors.NotFound("settlement not found")
	}
	if status != http.StatusOK {
		return nil, apperrors.BadGateway("billing register returned unexpected status", fmt.Errorf("status %d", status))
	}
	var payload settlementResponse
	if err := json.Unmarshal(body, &payload); err != nil {
		return nil, apperrors.BadGateway("invalid settlement response", err)
	}
	return mapSettlement(payload, tenantID, transportOrderID)
}

func (c *Client) GetBillingLink(ctx context.Context, tenantID, settlementID uuid.UUID) (*BillingLinkFact, error) {
	url := fmt.Sprintf("%s/internal/v1/freight-settlements/%s/billing-link", c.baseURL, settlementID)
	body, status, err := c.doGet(ctx, tenantID, url, "get_settlement_billing_link")
	if err != nil {
		return nil, err
	}
	if status == http.StatusNotFound {
		return nil, apperrors.NotFound("billing link not found")
	}
	if status != http.StatusOK {
		return nil, apperrors.BadGateway("billing register returned unexpected status", fmt.Errorf("status %d", status))
	}
	var payload billingLinkResponse
	if err := json.Unmarshal(body, &payload); err != nil {
		return nil, apperrors.BadGateway("invalid billing link response", err)
	}
	return mapBillingLink(payload, tenantID, settlementID)
}

func (c *Client) GetRegisterPayable(ctx context.Context, tenantID, billingRegisterID uuid.UUID) (*RegisterPayableFact, error) {
	url := fmt.Sprintf("%s/internal/v1/billing-registers/%s/payable", c.baseURL, billingRegisterID)
	body, status, err := c.doGet(ctx, tenantID, url, "get_register_payable")
	if err != nil {
		return nil, err
	}
	if status == http.StatusNotFound {
		return nil, apperrors.NotFound("billing register not found")
	}
	if status != http.StatusOK {
		return nil, apperrors.BadGateway("billing register returned unexpected status", fmt.Errorf("status %d", status))
	}
	var payload registerPayableResponse
	if err := json.Unmarshal(body, &payload); err != nil {
		return nil, apperrors.BadGateway("invalid register payable response", err)
	}
	return mapRegisterPayable(payload, tenantID, billingRegisterID)
}

func (c *Client) doGet(ctx context.Context, tenantID uuid.UUID, url, operation string) ([]byte, int, error) {
	if c == nil || c.baseURL == "" {
		return nil, 0, apperrors.Unavailable("billing register service url is not configured", errors.New("missing base url"))
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, 0, apperrors.Internal("create billing register request", err)
	}
	req.Header.Set("X-Tenant-ID", tenantID.String())
	if c.token != "" {
		req.Header.Set(internalauth.HeaderName, c.token)
	}
	resp, err := c.client.Do(req)
	if err != nil {
		c.observe(operation, "error", string(apperrors.CodeUnavailable))
		return nil, 0, apperrors.Unavailable("billing register service unavailable", err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		c.observe(operation, "error", string(apperrors.CodeUnavailable))
		return nil, 0, apperrors.Unavailable("billing register service unavailable", err)
	}
	if resp.StatusCode >= 500 {
		c.observe(operation, "error", string(apperrors.CodeUnavailable))
		return nil, resp.StatusCode, apperrors.Unavailable("billing register service unavailable", fmt.Errorf("status %d", resp.StatusCode))
	}
	if resp.StatusCode == http.StatusOK {
		c.observe(operation, "success", "")
	} else if resp.StatusCode == http.StatusNotFound {
		c.observe(operation, "not_found", string(apperrors.CodeNotFound))
	} else {
		c.observe(operation, "error", string(apperrors.CodeBadGateway))
	}
	return body, resp.StatusCode, nil
}

func mapSettlement(payload settlementResponse, requestedTenantID, requestedTransportOrderID uuid.UUID) (*SettlementFact, error) {
	transportOrderID, err := parseUUID("transport_order_id", payload.TransportOrderID)
	if err != nil {
		return nil, err
	}
	if transportOrderID != requestedTransportOrderID {
		return nil, apperrors.BadGateway("transport order mismatch", fmt.Errorf("expected %s", requestedTransportOrderID))
	}
	tenantID, err := parseUUID("tenant_id", payload.TenantID)
	if err != nil {
		return nil, err
	}
	if tenantID != requestedTenantID {
		return nil, apperrors.BadGateway("tenant mismatch", fmt.Errorf("expected %s", requestedTenantID))
	}
	settlementID, err := parseUUID("settlement_id", payload.SettlementID)
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
	rateSnapshotID, err := parseOptionalUUID("rate_snapshot_id", payload.RateSnapshotID)
	if err != nil {
		return nil, err
	}
	if err := domain.ValidateCurrencyCode(payload.CurrencyCode); err != nil {
		return nil, apperrors.BadGateway("invalid currency", err)
	}
	var shipmentID *uuid.UUID
	if strings.TrimSpace(payload.ShipmentID) != "" {
		id, err := parseUUID("shipment_id", payload.ShipmentID)
		if err != nil {
			return nil, err
		}
		shipmentID = &id
	}
	updatedAt, err := time.Parse(time.RFC3339, strings.TrimSpace(payload.UpdatedAt))
	if err != nil {
		return nil, apperrors.BadGateway("invalid updated_at", err)
	}
	fact := &SettlementFact{
		SettlementID:        settlementID,
		TransportOrderID:    transportOrderID,
		TenantID:            tenantID,
		BuyerCompanyID:      buyerID,
		CarrierCompanyID:    carrierID,
		ShipmentID:          shipmentID,
		Status:              strings.TrimSpace(payload.Status),
		OpenDisputeCount:    payload.OpenDisputeCount,
		Version:             int64(payload.Version),
		BillingLinkRevision: payload.BillingLinkRevision,
		BillingLinkState:    strings.TrimSpace(payload.BillingLinkState),
		CurrencyCode:                    strings.ToUpper(strings.TrimSpace(payload.CurrencyCode)),
		ProposedAccessorialSourceStatus: strings.ToUpper(strings.TrimSpace(payload.ProposedAccessorialSourceStatus)),
		RateSnapshotID:                  rateSnapshotID,
		UpdatedAt:                       updatedAt.UTC(),
	}
	if strings.TrimSpace(payload.AccrualAmountExVAT) != "" {
		amount, err := domain.ParseMoneyAmount(payload.AccrualAmountExVAT)
		if err != nil {
			return nil, apperrors.BadGateway("invalid accrual amount", err)
		}
		fact.AccrualAmountExVAT = &amount
	}
	if strings.TrimSpace(payload.TotalWithoutVAT) != "" {
		amount, err := domain.ParseMoneyAmount(payload.TotalWithoutVAT)
		if err != nil {
			return nil, apperrors.BadGateway("invalid total_without_vat", err)
		}
		fact.TotalWithoutVAT = &amount
	}
	if strings.TrimSpace(payload.ProposedAccessorialTotalExVAT) != "" && fact.ProposedAccessorialSourceStatus == domain.ProposedSourceKnown {
		amount, err := domain.ParseMoneyAmount(payload.ProposedAccessorialTotalExVAT)
		if err != nil {
			return nil, apperrors.BadGateway("invalid proposed_accessorial_total_ex_vat", err)
		}
		fact.ProposedAccessorialTotalExVAT = &amount
	}
	if strings.TrimSpace(payload.BaseFreightAmount) != "" {
		amount, err := domain.ParseMoneyAmount(payload.BaseFreightAmount)
		if err != nil {
			return nil, apperrors.BadGateway("invalid base_freight_amount", err)
		}
		fact.BaseFreightAmount = &amount
	}
	for _, item := range payload.ApprovedAccessorials {
		accessorialID, err := parseUUID("accessorial_id", item.AccessorialID)
		if err != nil {
			return nil, err
		}
		amount, err := domain.ParseMoneyAmount(item.AmountExVAT)
		if err != nil {
			return nil, apperrors.BadGateway("invalid approved accessorial amount", err)
		}
		fact.ApprovedAccessorials = append(fact.ApprovedAccessorials, ApprovedAccessorialFact{
			AccessorialID: accessorialID,
			ChargeCode:    item.ChargeCode,
			Amount:        amount,
		})
	}
	return fact, nil
}

func mapBillingLink(payload billingLinkResponse, requestedTenantID, requestedSettlementID uuid.UUID) (*BillingLinkFact, error) {
	settlementID, err := parseUUID("settlement_id", payload.SettlementID)
	if err != nil {
		return nil, err
	}
	if settlementID != requestedSettlementID {
		return nil, apperrors.BadGateway("settlement mismatch", fmt.Errorf("expected %s", requestedSettlementID))
	}
	tenantID, err := parseUUID("tenant_id", payload.TenantID)
	if err != nil {
		return nil, err
	}
	if tenantID != requestedTenantID {
		return nil, apperrors.BadGateway("tenant mismatch", fmt.Errorf("expected %s", requestedTenantID))
	}
	transportOrderID, err := parseUUID("transport_order_id", payload.TransportOrderID)
	if err != nil {
		return nil, err
	}
	fact := &BillingLinkFact{
		SettlementID:        settlementID,
		TransportOrderID:    transportOrderID,
		TenantID:            tenantID,
		BillingLinkRevision: payload.BillingLinkRevision,
		BillingLinkState:    strings.TrimSpace(payload.BillingLinkState),
		CurrencyCode:        strings.ToUpper(strings.TrimSpace(payload.CurrencyCode)),
		TaxBasis:            domain.TaxBasisExVAT,
	}
	if payload.BillingRegisterID != nil && strings.TrimSpace(*payload.BillingRegisterID) != "" {
		id, err := parseUUID("billing_register_id", *payload.BillingRegisterID)
		if err != nil {
			return nil, err
		}
		fact.BillingRegisterID = &id
	}
	if payload.AmountExVAT != nil && strings.TrimSpace(*payload.AmountExVAT) != "" {
		amount, err := domain.ParseMoneyAmount(*payload.AmountExVAT)
		if err != nil {
			return nil, apperrors.BadGateway("invalid billed amount", err)
		}
		fact.AmountExVAT = &amount
	}
	return fact, nil
}

func mapRegisterPayable(payload registerPayableResponse, requestedTenantID, requestedRegisterID uuid.UUID) (*RegisterPayableFact, error) {
	registerID, err := parseUUID("register_id", payload.RegisterID)
	if err != nil {
		return nil, err
	}
	if registerID != requestedRegisterID {
		return nil, apperrors.BadGateway("register mismatch", fmt.Errorf("expected %s", requestedRegisterID))
	}
	tenantID, err := parseUUID("tenant_id", payload.TenantID)
	if err != nil {
		return nil, err
	}
	if tenantID != requestedTenantID {
		return nil, apperrors.BadGateway("tenant mismatch", fmt.Errorf("expected %s", requestedTenantID))
	}
	updatedAt, err := time.Parse(time.RFC3339, strings.TrimSpace(payload.UpdatedAt))
	if err != nil {
		return nil, apperrors.BadGateway("invalid updated_at", err)
	}
	totalWithVAT, err := domain.ParseMoneyAmount(payload.TotalWithVAT)
	if err != nil {
		return nil, apperrors.BadGateway("invalid total_with_vat", err)
	}
	return &RegisterPayableFact{
		BillingRegisterID: registerID,
		TenantID:          tenantID,
		Version:           payload.Version,
		TotalWithVAT:      totalWithVAT,
		CurrencyCode:      strings.ToUpper(strings.TrimSpace(payload.CurrencyCode)),
		UpdatedAt:         updatedAt.UTC(),
	}, nil
}

func parseOptionalUUID(field string, raw *string) (*uuid.UUID, error) {
	if raw == nil || strings.TrimSpace(*raw) == "" {
		return nil, nil
	}
	id, err := parseUUID(field, *raw)
	if err != nil {
		return nil, err
	}
	return &id, nil
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
