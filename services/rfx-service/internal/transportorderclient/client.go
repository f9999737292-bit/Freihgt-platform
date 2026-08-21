package transportorderclient

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"

	apperrors "github.com/freight-platform/rfx-service/internal/platform/errors"
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
		timeout = 15 * time.Second
	}
	return &Client{cfg: cfg, client: &http.Client{Timeout: timeout}}
}

type CreateFromAwardScopeRequest struct {
	RfxEventID            uuid.UUID
	RfxLotID              *uuid.UUID
	OrderNumber           string
	ShipperCompanyID      uuid.UUID
	ConsigneeCompanyID    uuid.UUID
	OriginLocationID      uuid.UUID
	DestinationLocationID uuid.UUID
	CargoID               uuid.UUID
	TransportMode         string
	EquipmentType         string
	CarrierCompanyID      uuid.UUID
	SourceSystem          *string
	ExternalReference     *string
	ActorUserID           uuid.UUID
	ActorCompanyID        uuid.UUID
	IdempotencyKey        string
	TenantID              uuid.UUID
}

type CreateFromAwardScopeResponse struct {
	TransportOrderID uuid.UUID
	RateSnapshotID   uuid.UUID
	OrderNumber      string
	OrderStatus      string
}

type createRequestBody struct {
	RfxEventID            string  `json:"rfx_event_id"`
	RfxLotID              *string `json:"rfx_lot_id,omitempty"`
	OrderNumber           string  `json:"order_number"`
	ShipperCompanyID      string  `json:"shipper_company_id"`
	ConsigneeCompanyID    string  `json:"consignee_company_id"`
	OriginLocationID      string  `json:"origin_location_id"`
	DestinationLocationID string  `json:"destination_location_id"`
	CargoID               string  `json:"cargo_id"`
	TransportMode         string  `json:"transport_mode"`
	EquipmentType         string  `json:"equipment_type"`
	CarrierCompanyID      string  `json:"carrier_company_id"`
	SourceSystem          *string `json:"source_system,omitempty"`
	ExternalReference     *string `json:"external_reference,omitempty"`
	ActorUserID           string  `json:"actor_user_id"`
	ActorCompanyID        string  `json:"actor_company_id"`
}

type createResponseBody struct {
	ID            string `json:"id"`
	OrderNumber   string `json:"order_number"`
	Status        string `json:"status"`
	RateSnapshotID string `json:"rate_snapshot_id"`
}

func (c *Client) CreateFromAwardScope(ctx context.Context, in CreateFromAwardScopeRequest) (CreateFromAwardScopeResponse, error) {
	base := strings.TrimRight(strings.TrimSpace(c.cfg.BaseURL), "/")
	if base == "" {
		return CreateFromAwardScopeResponse{}, apperrors.Internal("transport order service url is not configured", nil)
	}
	body := createRequestBody{
		RfxEventID:            in.RfxEventID.String(),
		OrderNumber:           in.OrderNumber,
		ShipperCompanyID:      in.ShipperCompanyID.String(),
		ConsigneeCompanyID:    in.ConsigneeCompanyID.String(),
		OriginLocationID:      in.OriginLocationID.String(),
		DestinationLocationID: in.DestinationLocationID.String(),
		CargoID:               in.CargoID.String(),
		TransportMode:         in.TransportMode,
		EquipmentType:         in.EquipmentType,
		CarrierCompanyID:      in.CarrierCompanyID.String(),
		SourceSystem:          in.SourceSystem,
		ExternalReference:     in.ExternalReference,
		ActorUserID:           in.ActorUserID.String(),
		ActorCompanyID:        in.ActorCompanyID.String(),
	}
	if in.RfxLotID != nil && *in.RfxLotID != uuid.Nil {
		s := in.RfxLotID.String()
		body.RfxLotID = &s
	}
	raw, err := json.Marshal(body)
	if err != nil {
		return CreateFromAwardScopeResponse{}, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, base+"/internal/v1/transport-orders/from-award-scope", bytes.NewReader(raw))
	if err != nil {
		return CreateFromAwardScopeResponse{}, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Tenant-ID", in.TenantID.String())
	req.Header.Set("Idempotency-Key", in.IdempotencyKey)
	if token := strings.TrimSpace(c.cfg.InternalServiceToken); token != "" {
		req.Header.Set("X-Internal-Service-Token", token)
	}
	resp, err := c.client.Do(req)
	if err != nil {
		return CreateFromAwardScopeResponse{}, apperrors.Validation("transport order creation unavailable", map[string]any{"code": "TRANSPORT_ORDER_UNAVAILABLE"})
	}
	defer resp.Body.Close()
	payload, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if resp.StatusCode >= 400 {
		return CreateFromAwardScopeResponse{}, mapClientError(payload)
	}
	var parsed createResponseBody
	if err := json.Unmarshal(payload, &parsed); err != nil {
		return CreateFromAwardScopeResponse{}, apperrors.Internal("invalid transport order response", err)
	}
	orderID, err := uuid.Parse(parsed.ID)
	if err != nil {
		return CreateFromAwardScopeResponse{}, apperrors.Internal("invalid transport order id in response", err)
	}
	snapshotID, err := uuid.Parse(parsed.RateSnapshotID)
	if err != nil {
		return CreateFromAwardScopeResponse{}, apperrors.Internal("invalid rate snapshot id in response", err)
	}
	return CreateFromAwardScopeResponse{
		TransportOrderID: orderID,
		RateSnapshotID:   snapshotID,
		OrderNumber:      parsed.OrderNumber,
		OrderStatus:      parsed.Status,
	}, nil
}

func mapClientError(body []byte) error {
	var envelope struct {
		Error struct {
			Message string         `json:"message"`
			Details map[string]any `json:"details"`
		} `json:"error"`
	}
	_ = json.Unmarshal(body, &envelope)
	if envelope.Error.Message == "" {
		return apperrors.Validation("transport order creation failed", map[string]any{"code": "TRANSPORT_ORDER_CREATE_FAILED"})
	}
	return apperrors.Validation(envelope.Error.Message, envelope.Error.Details)
}

func AwardConversionIdempotencyKey(tenantID, eventID uuid.UUID, lotID uuid.UUID) string {
	lotKey := "event"
	if lotID != uuid.Nil {
		lotKey = lotID.String()
	}
	return fmt.Sprintf("award-conv:%s:%s:%s", tenantID.String(), eventID.String(), lotKey)
}
