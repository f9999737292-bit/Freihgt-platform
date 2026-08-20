package contractrate

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"

	"github.com/freight-platform/transport-order-service/internal/domain"
	apperrors "github.com/freight-platform/transport-order-service/internal/platform/errors"
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
		timeout = 10 * time.Second
	}
	return &Client{cfg: cfg, client: &http.Client{Timeout: timeout}}
}

type resolveRequest struct {
	BuyerCompanyID        uuid.UUID  `json:"buyer_company_id"`
	CarrierCompanyID      uuid.UUID  `json:"carrier_company_id"`
	OriginLocationID      uuid.UUID  `json:"origin_location_id"`
	DestinationLocationID uuid.UUID  `json:"destination_location_id"`
	EquipmentType         string     `json:"equipment_type"`
	TransportMode         string     `json:"transport_mode"`
	PricingDate           string     `json:"pricing_date"`
	CurrencyCode          *string    `json:"currency_code,omitempty"`
	ManualSpotAmount      *string    `json:"manual_spot_amount,omitempty"`
	ManualSpotCurrency    *string    `json:"manual_spot_currency,omitempty"`
	PricingSource         *string    `json:"pricing_source,omitempty"`
	AwardLinkID           *uuid.UUID `json:"award_link_id,omitempty"`
	AwardScopeEventID     *uuid.UUID `json:"award_scope_event_id,omitempty"`
	AwardScopeLotID       *uuid.UUID `json:"award_scope_lot_id,omitempty"`
	BidID                 *uuid.UUID `json:"bid_id,omitempty"`
}

func (c *Client) Resolve(ctx context.Context, in domain.CreatePricedTransportOrderInput, pricingDate time.Time) (domain.ResolveRateResult, error) {
	base := strings.TrimRight(strings.TrimSpace(c.cfg.BaseURL), "/")
	if base == "" {
		return domain.ResolveRateResult{}, apperrors.Validation("contract rate service url is not configured", nil)
	}
	equipment := strings.ToUpper(strings.TrimSpace(domain.DerefString(in.EquipmentType)))
	body := resolveRequest{
		BuyerCompanyID:        in.ShipperCompanyID,
		CarrierCompanyID:      in.PricingContext.CarrierCompanyID,
		OriginLocationID:      in.OriginLocationID,
		DestinationLocationID: in.DestinationLocationID,
		EquipmentType:         equipment,
		TransportMode:         domain.NormalizeTransportMode(in.TransportMode),
		PricingDate:           pricingDate.Format("2006-01-02"),
		PricingSource:         in.PricingContext.PricingSource,
		AwardLinkID:           in.PricingContext.AwardLinkID,
		AwardScopeEventID:     in.PricingContext.AwardScopeEventID,
		AwardScopeLotID:       in.PricingContext.AwardScopeLotID,
		BidID:                 in.PricingContext.BidID,
		ManualSpotAmount:      in.PricingContext.ManualSpotAmount,
		ManualSpotCurrency:    in.PricingContext.ManualSpotCurrency,
	}

	raw, err := json.Marshal(body)
	if err != nil {
		return domain.ResolveRateResult{}, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, base+"/internal/v1/rates/resolve", bytes.NewReader(raw))
	if err != nil {
		return domain.ResolveRateResult{}, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Tenant-ID", in.Actor.TenantID.String())
	req.Header.Set("X-User-ID", in.Actor.UserID.String())
	req.Header.Set("X-Company-ID", in.Actor.CompanyID.String())
	req.Header.Set("X-Actor-Kind", in.Actor.ActorKind)
	if token := strings.TrimSpace(c.cfg.InternalServiceToken); token != "" {
		req.Header.Set("X-Internal-Service-Token", token)
	}
	resp, err := c.client.Do(req)
	if err != nil {
		return domain.ResolveRateResult{}, apperrors.Validation("rate resolution unavailable", map[string]any{"code": "RATE_RESOLUTION_UNAVAILABLE"})
	}
	defer resp.Body.Close()
	payload, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if resp.StatusCode >= 400 {
		return domain.ResolveRateResult{}, mapResolveError(payload, resp.StatusCode)
	}
	var result domain.ResolveRateResult
	if err := json.Unmarshal(payload, &result); err != nil {
		return domain.ResolveRateResult{}, apperrors.Internal("invalid rate resolution response", err)
	}
	if result.Status != "MATCHED" {
		code := "RATE_NOT_FOUND"
		return domain.ResolveRateResult{}, apperrors.Validation("rate could not be resolved for transport order", map[string]any{"code": code, "status": result.Status})
	}
	return result, nil
}

func mapResolveError(body []byte, status int) error {
	var envelope struct {
		Error struct {
			Message string         `json:"message"`
			Details map[string]any `json:"details"`
		} `json:"error"`
	}
	_ = json.Unmarshal(body, &envelope)
	details := envelope.Error.Details
	if details == nil {
		details = map[string]any{}
	}
	if code, ok := details["code"].(string); ok && code != "" {
		return apperrors.Validation(envelope.Error.Message, map[string]any{"code": code})
	}
	if status == http.StatusForbidden {
		return apperrors.Validation(envelope.Error.Message, map[string]any{"code": "MANUAL_SPOT_FORBIDDEN"})
	}
	return apperrors.Validation(envelope.Error.Message, details)
}

func ResolveCarrierID(in domain.CreatePricedTransportOrderInput, result domain.ResolveRateResult) (uuid.UUID, error) {
	if in.PricingContext.CarrierCompanyID != uuid.Nil {
		return in.PricingContext.CarrierCompanyID, nil
	}
	if result.CarrierCompanyID != nil && *result.CarrierCompanyID != uuid.Nil {
		return *result.CarrierCompanyID, nil
	}
	return uuid.Nil, apperrors.Validation("carrier_company_id is required", map[string]any{"field": "carrier_company_id"})
}

func BuildSnapshotFromResolve(
	in domain.CreatePricedTransportOrderInput,
	result domain.ResolveRateResult,
	requestHash string,
	carrierID uuid.UUID,
) (domain.RateSnapshot, error) {
	if result.TotalAmount == nil || strings.TrimSpace(*result.TotalAmount) == "" {
		return domain.RateSnapshot{}, apperrors.Validation("resolved rate missing total_amount", map[string]any{"code": "RATE_NOT_FOUND"})
	}
	total, err := decimal.NewFromString(strings.TrimSpace(*result.TotalAmount))
	if err != nil {
		return domain.RateSnapshot{}, apperrors.Validation("invalid resolved total_amount", map[string]any{"code": "RATE_NOT_FOUND"})
	}
	var baseAmount *decimal.Decimal
	if result.BaseAmount != nil && strings.TrimSpace(*result.BaseAmount) != "" {
		base, err := decimal.NewFromString(strings.TrimSpace(*result.BaseAmount))
		if err != nil {
			return domain.RateSnapshot{}, apperrors.Validation("invalid resolved base_amount", map[string]any{"code": "RATE_NOT_FOUND"})
		}
		baseAmount = &base
	}
	components := domain.SortedComponentJSON(result.Components)
	accessorials := result.AccessorialRules
	if len(accessorials) == 0 {
		accessorials = domain.EmptyJSONArray()
	}
	currency := ""
	if result.CurrencyCode != nil {
		currency = strings.TrimSpace(*result.CurrencyCode)
	}
	pricingDate, _ := time.Parse("2006-01-02", result.PricingDate)
	resolverVersion := result.ResolverVersion
	if strings.TrimSpace(resolverVersion) == "" {
		resolverVersion = domain.ResolverVersionSnapshotV1
	}
	return domain.RateSnapshot{
		TenantID:                 in.TenantID,
		BuyerCompanyID:           in.ShipperCompanyID,
		CarrierCompanyID:         carrierID,
		PricingSource:            domain.NormalizePricingSource(result.PricingSource),
		AwardLinkID:              result.AwardLinkID,
		RfxEventID:               result.RfxEventID,
		RfxLotID:                 result.RfxLotID,
		BidID:                    result.BidID,
		ManualSpotAuditID:        result.ManualSpotAuditID,
		ContractID:               result.ContractID,
		RateCardID:               result.RateCardID,
		RateVersionID:            result.RateVersionID,
		RateLineID:               result.RateLineID,
		ContractNumber:           result.ContractNumber,
		RateCardName:             result.RateCardName,
		RateVersionNumber:        result.VersionNumber,
		OriginLocationID:         in.OriginLocationID,
		DestinationLocationID:    in.DestinationLocationID,
		EquipmentType:            strings.ToUpper(strings.TrimSpace(domain.DerefString(in.EquipmentType))),
		TransportMode:            domain.NormalizeTransportMode(in.TransportMode),
		CurrencyCode:             currency,
		ComponentBreakdownStatus: result.ComponentBreakdownStatus,
		Components:               components,
		AccessorialRules:         accessorials,
		BaseAmount:               baseAmount,
		TotalAmount:              total,
		PricingDate:              pricingDate,
		ResolvedAt:               result.ResolvedAt,
		ResolvedByService:        domain.ResolvedServiceContractRate,
		ResolverVersion:          resolverVersion,
		ResolutionRequestHash:    requestHash,
	}, nil
}
