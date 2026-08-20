package rfxclient

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/freight-platform/contract-rate-service/internal/domain"
	apperrors "github.com/freight-platform/contract-rate-service/internal/platform/errors"
)

type Config struct {
	BaseURL              string
	InternalServiceToken string
	Timeout              time.Duration
}

type Client struct {
	cfg    Config
	client *http.Client
}

func New(cfg Config) *Client {
	timeout := cfg.Timeout
	if timeout <= 0 {
		timeout = 5 * time.Second
	}
	return &Client{cfg: cfg, client: &http.Client{Timeout: timeout}}
}

type pricingContextDTO struct {
	TenantID                 uuid.UUID `json:"tenant_id"`
	SourceType               string    `json:"source_type"`
	SourceID                 uuid.UUID `json:"source_id"`
	BuyerCompanyID           uuid.UUID `json:"buyer_company_id"`
	CarrierCompanyID         uuid.UUID `json:"carrier_company_id"`
	OriginLocationID         uuid.UUID `json:"origin_location_id"`
	DestinationLocationID    uuid.UUID `json:"destination_location_id"`
	EquipmentType            string    `json:"equipment_type"`
	TransportMode            string    `json:"transport_mode"`
	CurrencyCode             string    `json:"currency_code"`
	TotalAmount              string    `json:"total_amount"`
	BaseAmount               *string   `json:"base_amount"`
	Components               []domain.ResolvedComponent `json:"components"`
	ComponentBreakdownStatus string    `json:"component_breakdown_status"`
	SourceStatus             string    `json:"source_status"`
	RfxEventID               *uuid.UUID `json:"rfx_event_id,omitempty"`
	RfxLotID                 *uuid.UUID `json:"rfx_lot_id,omitempty"`
	AwardLinkID              *uuid.UUID `json:"award_link_id,omitempty"`
	BidID                    *uuid.UUID `json:"bid_id,omitempty"`
}

func (c *Client) GetAwardLinkPricingContext(ctx context.Context, tenantID, linkID uuid.UUID) (domain.RFxPricingContext, error) {
	path := fmt.Sprintf("/internal/v1/pricing/award-context/%s", linkID)
	return c.fetch(ctx, tenantID, path)
}

func (c *Client) GetAwardScopePricingContext(ctx context.Context, tenantID, eventID uuid.UUID, lotID *uuid.UUID) (domain.RFxPricingContext, error) {
	path := fmt.Sprintf("/internal/v1/pricing/award-scope/%s", eventID)
	if lotID != nil && *lotID != uuid.Nil {
		path += "?lot_id=" + lotID.String()
	}
	return c.fetch(ctx, tenantID, path)
}

func (c *Client) GetAcceptedBidPricingContext(ctx context.Context, tenantID, bidID uuid.UUID) (domain.RFxPricingContext, error) {
	path := fmt.Sprintf("/internal/v1/pricing/bid-context/%s", bidID)
	return c.fetch(ctx, tenantID, path)
}

func (c *Client) fetch(ctx context.Context, tenantID uuid.UUID, path string) (domain.RFxPricingContext, error) {
	base := strings.TrimRight(strings.TrimSpace(c.cfg.BaseURL), "/")
	if base == "" {
		return domain.RFxPricingContext{}, apperrors.Validation("rfx service url is not configured", map[string]any{"code": domain.ReasonPricingSourceNotAvail})
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, base+path, nil)
	if err != nil {
		return domain.RFxPricingContext{}, err
	}
	req.Header.Set("X-Tenant-ID", tenantID.String())
	if token := strings.TrimSpace(c.cfg.InternalServiceToken); token != "" {
		req.Header.Set("X-Internal-Service-Token", token)
	}
	resp, err := c.client.Do(req)
	if err != nil {
		return domain.RFxPricingContext{}, apperrors.Validation("rfx pricing source unavailable", map[string]any{"code": domain.ReasonPricingSourceNotAvail})
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if resp.StatusCode == http.StatusNotFound {
		return domain.RFxPricingContext{}, apperrors.NotFound("pricing source not found")
	}
	if resp.StatusCode == http.StatusForbidden || resp.StatusCode == http.StatusUnauthorized {
		return domain.RFxPricingContext{}, apperrors.Forbidden("pricing source forbidden", map[string]any{"code": domain.ReasonSourceForbidden})
	}
	if resp.StatusCode >= 500 {
		return domain.RFxPricingContext{}, apperrors.Validation("rfx pricing source unavailable", map[string]any{"code": domain.ReasonPricingSourceNotAvail})
	}
	if resp.StatusCode >= 400 {
		return domain.RFxPricingContext{}, mapClientError(body, resp.StatusCode)
	}
	var dto pricingContextDTO
	if err := json.Unmarshal(body, &dto); err != nil {
		return domain.RFxPricingContext{}, apperrors.Validation("invalid rfx pricing response", map[string]any{"code": domain.ReasonPricingSourceNotAvail})
	}
	return mapDTO(dto), nil
}

func mapDTO(dto pricingContextDTO) domain.RFxPricingContext {
	return domain.RFxPricingContext{
		TenantID:                 dto.TenantID,
		SourceType:               dto.SourceType,
		SourceID:                 dto.SourceID,
		BuyerCompanyID:           dto.BuyerCompanyID,
		CarrierCompanyID:         dto.CarrierCompanyID,
		OriginLocationID:         dto.OriginLocationID,
		DestinationLocationID:    dto.DestinationLocationID,
		EquipmentType:            dto.EquipmentType,
		TransportMode:            dto.TransportMode,
		CurrencyCode:             dto.CurrencyCode,
		TotalAmount:              dto.TotalAmount,
		BaseAmount:               dto.BaseAmount,
		ComponentBreakdownStatus: dto.ComponentBreakdownStatus,
		Components:               dto.Components,
		RfxEventID:               dto.RfxEventID,
		RfxLotID:                 dto.RfxLotID,
		AwardLinkID:              dto.AwardLinkID,
		BidID:                    dto.BidID,
		SourceStatus:             dto.SourceStatus,
	}
}

func mapClientError(body []byte, status int) error {
	var payload struct {
		Error struct {
			Code    string         `json:"code"`
			Message string         `json:"message"`
			Details map[string]any `json:"details"`
		} `json:"error"`
	}
	_ = json.Unmarshal(body, &payload)
	details := payload.Error.Details
	if details == nil {
		details = map[string]any{}
	}
	if code, ok := details["code"].(string); ok && code == "INVALID_PRICING_SOURCE" {
		return apperrors.Validation(payload.Error.Message, map[string]any{"code": domain.ReasonInvalidPricingSource})
	}
	if status == http.StatusNotFound {
		return apperrors.NotFound("pricing source not found")
	}
	return apperrors.Validation("pricing source request failed", map[string]any{"code": domain.ReasonPricingSourceNotAvail})
}
