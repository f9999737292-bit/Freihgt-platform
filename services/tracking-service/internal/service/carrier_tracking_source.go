package service

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/freight-platform/tracking-service/internal/domain"
	apperrors "github.com/freight-platform/tracking-service/internal/platform/errors"
)

// CarrierTrackingContext is the canonical shipment timing proof used by carrier-safe tracking.
type CarrierTrackingContext struct {
	ShipmentID        uuid.UUID
	TenantID          uuid.UUID
	CarrierCompanyID  uuid.UUID
	Status            string
	PlannedPickupAt   *time.Time
	PlannedDeliveryAt *time.Time
	ActualPickupAt    *time.Time
	ActualDeliveryAt  *time.Time
}

type CarrierParticipantProver interface {
	Prove(ctx context.Context, tenantID, shipmentID, carrierCompanyID uuid.UUID) (CarrierTrackingContext, error)
}

// CarrierTrackingContextClient calls shipment-service once per request. It does not read tracking tables.
type CarrierTrackingContextClient struct {
	baseURL string
	token   string
	http    *http.Client
}

func NewCarrierTrackingContextClient(baseURL, token string) *CarrierTrackingContextClient {
	return &CarrierTrackingContextClient{
		baseURL: strings.TrimRight(strings.TrimSpace(baseURL), "/"),
		token:   strings.TrimSpace(token),
		http:    &http.Client{Timeout: 5 * time.Second},
	}
}

func (c *CarrierTrackingContextClient) Prove(ctx context.Context, tenantID, shipmentID, carrierCompanyID uuid.UUID) (CarrierTrackingContext, error) {
	var zero CarrierTrackingContext
	if c == nil || c.baseURL == "" || c.token == "" {
		return zero, apperrors.Unavailable("shipment participant check is not configured")
	}
	endpoint := fmt.Sprintf("%s/internal/v1/shipments/%s/carrier-tracking-context?carrier_company_id=%s", c.baseURL, shipmentID, url.QueryEscape(carrierCompanyID.String()))
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return zero, apperrors.Unavailable("shipment participant check failed")
	}
	req.Header.Set("X-Internal-Service-Token", c.token)
	req.Header.Set("X-Tenant-ID", tenantID.String())
	resp, err := c.http.Do(req)
	if err != nil {
		return zero, apperrors.Unavailable("shipment participant check failed")
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return zero, apperrors.Unavailable("shipment participant check failed")
	}
	switch resp.StatusCode {
	case http.StatusOK:
	case http.StatusNotFound:
		return zero, apperrors.NotFound("shipment not found")
	case http.StatusBadRequest:
		return zero, apperrors.Validation("carrier company is invalid", map[string]any{"field": "carrier_company_id"})
	default:
		return zero, apperrors.Unavailable("shipment participant check failed")
	}
	var payload struct {
		ShipmentID        uuid.UUID  `json:"shipmentId"`
		TenantID          uuid.UUID  `json:"tenantId"`
		CarrierCompanyID  uuid.UUID  `json:"carrierCompanyId"`
		Status            string     `json:"status"`
		PlannedPickupAt   *time.Time `json:"plannedPickupAt"`
		PlannedDeliveryAt *time.Time `json:"plannedDeliveryAt"`
		ActualPickupAt    *time.Time `json:"actualPickupAt"`
		ActualDeliveryAt  *time.Time `json:"actualDeliveryAt"`
	}
	if err := json.Unmarshal(body, &payload); err != nil {
		return zero, apperrors.Unavailable("shipment participant check failed")
	}
	if payload.ShipmentID != shipmentID || payload.TenantID != tenantID || payload.CarrierCompanyID != carrierCompanyID {
		return zero, apperrors.NotFound("shipment not found")
	}
	return CarrierTrackingContext{
		ShipmentID: payload.ShipmentID, TenantID: payload.TenantID, CarrierCompanyID: payload.CarrierCompanyID,
		Status: payload.Status, PlannedPickupAt: payload.PlannedPickupAt, PlannedDeliveryAt: payload.PlannedDeliveryAt,
		ActualPickupAt: payload.ActualPickupAt, ActualDeliveryAt: payload.ActualDeliveryAt,
	}, nil
}

// CarrierTrackingSource proves carrier participation before any tracking, ETA, or slot read.
type CarrierTrackingSource struct {
	participants CarrierParticipantProver
	tracking     shipperTrackingReader
	eta          shipperETAReader
	slots        shipperSlotReader
}

func NewCarrierTrackingSource(participants CarrierParticipantProver, tracking shipperTrackingReader, eta shipperETAReader, slots shipperSlotReader) *CarrierTrackingSource {
	return &CarrierTrackingSource{participants: participants, tracking: tracking, eta: eta, slots: slots}
}

func (s *CarrierTrackingSource) prove(ctx context.Context, tenantID, shipmentID, carrierCompanyID uuid.UUID) (CarrierTrackingContext, error) {
	if s == nil || s.participants == nil {
		return CarrierTrackingContext{}, apperrors.Unavailable("shipment participant check is not configured")
	}
	return s.participants.Prove(ctx, tenantID, shipmentID, carrierCompanyID)
}

func (s *CarrierTrackingSource) TrackingSummary(ctx context.Context, tenantID, shipmentID, carrierCompanyID uuid.UUID) (domain.TrackingSummary, error) {
	if _, err := s.prove(ctx, tenantID, shipmentID, carrierCompanyID); err != nil {
		return domain.TrackingSummary{}, err
	}
	return s.tracking.GetTrackingSummary(ctx, tenantID, shipmentID)
}

func (s *CarrierTrackingSource) LocationHistory(ctx context.Context, tenantID, shipmentID, carrierCompanyID uuid.UUID, from, to *time.Time, limit, offset int) ([]domain.LocationEvent, int, error) {
	if _, err := s.prove(ctx, tenantID, shipmentID, carrierCompanyID); err != nil {
		return nil, 0, err
	}
	return s.tracking.ListLocationHistory(ctx, tenantID, shipmentID, from, to, limit, offset)
}

func (s *CarrierTrackingSource) ETA(ctx context.Context, tenantID, shipmentID, carrierCompanyID uuid.UUID) (domain.ShipmentETASummary, error) {
	facts, err := s.prove(ctx, tenantID, shipmentID, carrierCompanyID)
	if err != nil {
		return domain.ShipmentETASummary{}, err
	}
	return s.eta.GetShipmentETA(ctx, tenantID, shipmentID, plannedFromCarrierContext(facts))
}

func (s *CarrierTrackingSource) ETAHistory(ctx context.Context, tenantID, shipmentID, carrierCompanyID uuid.UUID, targetType string, from, to *time.Time, limit, offset int) ([]domain.ETAObservation, int, error) {
	if _, err := s.prove(ctx, tenantID, shipmentID, carrierCompanyID); err != nil {
		return nil, 0, err
	}
	return s.eta.ListETAHistory(ctx, tenantID, shipmentID, targetType, from, to, limit, offset)
}

func (s *CarrierTrackingSource) Slots(ctx context.Context, tenantID, shipmentID, carrierCompanyID uuid.UUID) (domain.ShipmentSlotSummary, error) {
	facts, err := s.prove(ctx, tenantID, shipmentID, carrierCompanyID)
	if err != nil {
		return domain.ShipmentSlotSummary{}, err
	}
	etaSummary, err := s.eta.GetShipmentETA(ctx, tenantID, shipmentID, plannedFromCarrierContext(facts))
	if err != nil {
		return domain.ShipmentSlotSummary{}, err
	}
	return s.slots.GetShipmentSlots(ctx, tenantID, shipmentID, slotContextFromCarrier(facts, etaSummary))
}

func (s *CarrierTrackingSource) SlotHistory(ctx context.Context, tenantID, shipmentID, carrierCompanyID uuid.UUID, slotType string, from, to *time.Time, limit, offset int) ([]domain.SlotRevision, int, error) {
	if _, err := s.prove(ctx, tenantID, shipmentID, carrierCompanyID); err != nil {
		return nil, 0, err
	}
	return s.slots.ListSlotHistory(ctx, tenantID, shipmentID, slotType, from, to, limit, offset)
}

func plannedFromCarrierContext(facts CarrierTrackingContext) PlannedTimes {
	return PlannedTimes{
		PlannedPickupAt:   facts.PlannedPickupAt,
		PlannedDeliveryAt: facts.PlannedDeliveryAt,
		ActualPickupAt:    facts.ActualPickupAt,
		ActualDeliveryAt:  facts.ActualDeliveryAt,
		ShipmentStatus:    facts.Status,
	}
}

func slotContextFromCarrier(facts CarrierTrackingContext, eta domain.ShipmentETASummary) SlotMilestoneContext {
	out := SlotMilestoneContext{
		ShipmentStatus:   facts.Status,
		ActualPickupAt:   facts.ActualPickupAt,
		ActualDeliveryAt: facts.ActualDeliveryAt,
	}
	if eta.Pickup != nil {
		out.PickupETA = etaSnapshotFromTarget(*eta.Pickup)
	}
	if eta.Delivery != nil {
		out.DeliveryETA = etaSnapshotFromTarget(*eta.Delivery)
	}
	return out
}
