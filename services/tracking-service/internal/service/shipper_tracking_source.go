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

// ShipperTrackingContext is the canonical shipment timing proof used by customer-safe tracking.
type ShipperTrackingContext struct {
	ShipmentID        uuid.UUID
	TenantID          uuid.UUID
	ShipperCompanyID  uuid.UUID
	Status            string
	PlannedPickupAt   *time.Time
	PlannedDeliveryAt *time.Time
	ActualPickupAt    *time.Time
	ActualDeliveryAt  *time.Time
}

type ShipperParticipantProver interface {
	Prove(ctx context.Context, tenantID, shipmentID, shipperCompanyID uuid.UUID) (ShipperTrackingContext, error)
}

type shipperTrackingReader interface {
	GetTrackingSummary(ctx context.Context, tenantID, shipmentID uuid.UUID) (domain.TrackingSummary, error)
	ListLocationHistory(ctx context.Context, tenantID, shipmentID uuid.UUID, from, to *time.Time, limit, offset int) ([]domain.LocationEvent, int, error)
}

type shipperETAReader interface {
	GetShipmentETA(ctx context.Context, tenantID, shipmentID uuid.UUID, planned PlannedTimes) (domain.ShipmentETASummary, error)
	ListETAHistory(ctx context.Context, tenantID, shipmentID uuid.UUID, targetType string, from, to *time.Time, limit, offset int) ([]domain.ETAObservation, int, error)
}

type shipperSlotReader interface {
	GetShipmentSlots(ctx context.Context, tenantID, shipmentID uuid.UUID, milestone SlotMilestoneContext) (domain.ShipmentSlotSummary, error)
	ListSlotHistory(ctx context.Context, tenantID, shipmentID uuid.UUID, slotType string, from, to *time.Time, limit, offset int) ([]domain.SlotRevision, int, error)
}

// ShipperTrackingContextClient calls shipment-service once per request. It does not read tracking tables.
type ShipperTrackingContextClient struct {
	baseURL string
	token   string
	http    *http.Client
}

func NewShipperTrackingContextClient(baseURL, token string) *ShipperTrackingContextClient {
	return &ShipperTrackingContextClient{
		baseURL: strings.TrimRight(strings.TrimSpace(baseURL), "/"),
		token:   strings.TrimSpace(token),
		http:    &http.Client{Timeout: 5 * time.Second},
	}
}

func (c *ShipperTrackingContextClient) Prove(ctx context.Context, tenantID, shipmentID, shipperCompanyID uuid.UUID) (ShipperTrackingContext, error) {
	var zero ShipperTrackingContext
	if c == nil || c.baseURL == "" || c.token == "" {
		return zero, apperrors.Unavailable("shipment participant check is not configured")
	}
	endpoint := fmt.Sprintf("%s/internal/v1/shipments/%s/shipper-tracking-context?shipper_company_id=%s", c.baseURL, shipmentID, url.QueryEscape(shipperCompanyID.String()))
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
		return zero, apperrors.Validation("shipper company is invalid", map[string]any{"field": "shipper_company_id"})
	default:
		return zero, apperrors.Unavailable("shipment participant check failed")
	}
	var payload struct {
		ShipmentID        uuid.UUID  `json:"shipmentId"`
		TenantID          uuid.UUID  `json:"tenantId"`
		ShipperCompanyID  uuid.UUID  `json:"shipperCompanyId"`
		Status            string     `json:"status"`
		PlannedPickupAt   *time.Time `json:"plannedPickupAt"`
		PlannedDeliveryAt *time.Time `json:"plannedDeliveryAt"`
		ActualPickupAt    *time.Time `json:"actualPickupAt"`
		ActualDeliveryAt  *time.Time `json:"actualDeliveryAt"`
	}
	if err := json.Unmarshal(body, &payload); err != nil {
		return zero, apperrors.Unavailable("shipment participant check failed")
	}
	if payload.ShipmentID != shipmentID || payload.TenantID != tenantID || payload.ShipperCompanyID != shipperCompanyID {
		return zero, apperrors.NotFound("shipment not found")
	}
	return ShipperTrackingContext{
		ShipmentID: payload.ShipmentID, TenantID: payload.TenantID, ShipperCompanyID: payload.ShipperCompanyID,
		Status: payload.Status, PlannedPickupAt: payload.PlannedPickupAt, PlannedDeliveryAt: payload.PlannedDeliveryAt,
		ActualPickupAt: payload.ActualPickupAt, ActualDeliveryAt: payload.ActualDeliveryAt,
	}, nil
}

// ShipperTrackingSource proves shipper participation before any tracking, ETA, or slot read.
type ShipperTrackingSource struct {
	participants ShipperParticipantProver
	tracking     shipperTrackingReader
	eta          shipperETAReader
	slots        shipperSlotReader
}

func NewShipperTrackingSource(participants ShipperParticipantProver, tracking shipperTrackingReader, eta shipperETAReader, slots shipperSlotReader) *ShipperTrackingSource {
	return &ShipperTrackingSource{participants: participants, tracking: tracking, eta: eta, slots: slots}
}

func (s *ShipperTrackingSource) prove(ctx context.Context, tenantID, shipmentID, shipperCompanyID uuid.UUID) (ShipperTrackingContext, error) {
	if s == nil || s.participants == nil {
		return ShipperTrackingContext{}, apperrors.Unavailable("shipment participant check is not configured")
	}
	return s.participants.Prove(ctx, tenantID, shipmentID, shipperCompanyID)
}

func (s *ShipperTrackingSource) TrackingSummary(ctx context.Context, tenantID, shipmentID, shipperCompanyID uuid.UUID) (domain.TrackingSummary, error) {
	if _, err := s.prove(ctx, tenantID, shipmentID, shipperCompanyID); err != nil {
		return domain.TrackingSummary{}, err
	}
	return s.tracking.GetTrackingSummary(ctx, tenantID, shipmentID)
}

func (s *ShipperTrackingSource) LocationHistory(ctx context.Context, tenantID, shipmentID, shipperCompanyID uuid.UUID, from, to *time.Time, limit, offset int) ([]domain.LocationEvent, int, error) {
	if _, err := s.prove(ctx, tenantID, shipmentID, shipperCompanyID); err != nil {
		return nil, 0, err
	}
	return s.tracking.ListLocationHistory(ctx, tenantID, shipmentID, from, to, limit, offset)
}

func (s *ShipperTrackingSource) ETA(ctx context.Context, tenantID, shipmentID, shipperCompanyID uuid.UUID) (domain.ShipmentETASummary, error) {
	facts, err := s.prove(ctx, tenantID, shipmentID, shipperCompanyID)
	if err != nil {
		return domain.ShipmentETASummary{}, err
	}
	return s.eta.GetShipmentETA(ctx, tenantID, shipmentID, plannedFromShipperContext(facts))
}

func (s *ShipperTrackingSource) ETAHistory(ctx context.Context, tenantID, shipmentID, shipperCompanyID uuid.UUID, targetType string, from, to *time.Time, limit, offset int) ([]domain.ETAObservation, int, error) {
	if _, err := s.prove(ctx, tenantID, shipmentID, shipperCompanyID); err != nil {
		return nil, 0, err
	}
	return s.eta.ListETAHistory(ctx, tenantID, shipmentID, targetType, from, to, limit, offset)
}

func (s *ShipperTrackingSource) Slots(ctx context.Context, tenantID, shipmentID, shipperCompanyID uuid.UUID) (domain.ShipmentSlotSummary, error) {
	facts, err := s.prove(ctx, tenantID, shipmentID, shipperCompanyID)
	if err != nil {
		return domain.ShipmentSlotSummary{}, err
	}
	etaSummary, err := s.eta.GetShipmentETA(ctx, tenantID, shipmentID, plannedFromShipperContext(facts))
	if err != nil {
		return domain.ShipmentSlotSummary{}, err
	}
	return s.slots.GetShipmentSlots(ctx, tenantID, shipmentID, slotContextFromServer(facts, etaSummary))
}

func (s *ShipperTrackingSource) SlotHistory(ctx context.Context, tenantID, shipmentID, shipperCompanyID uuid.UUID, slotType string, from, to *time.Time, limit, offset int) ([]domain.SlotRevision, int, error) {
	if _, err := s.prove(ctx, tenantID, shipmentID, shipperCompanyID); err != nil {
		return nil, 0, err
	}
	return s.slots.ListSlotHistory(ctx, tenantID, shipmentID, slotType, from, to, limit, offset)
}

func plannedFromShipperContext(facts ShipperTrackingContext) PlannedTimes {
	return PlannedTimes{
		PlannedPickupAt:   facts.PlannedPickupAt,
		PlannedDeliveryAt: facts.PlannedDeliveryAt,
		ActualPickupAt:    facts.ActualPickupAt,
		ActualDeliveryAt:  facts.ActualDeliveryAt,
		ShipmentStatus:    facts.Status,
	}
}

func slotContextFromServer(facts ShipperTrackingContext, eta domain.ShipmentETASummary) SlotMilestoneContext {
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

func etaSnapshotFromTarget(target domain.ETATargetSummary) domain.ETASnapshot {
	return domain.ETASnapshot{
		HasUsableETA:       target.Status == domain.ETAStatusAvailable || target.Status == domain.ETAStatusStale,
		Status:             target.Status,
		FreshnessStatus:    target.FreshnessStatus,
		QualityStatus:      target.QualityStatus,
		EstimatedArrivalAt: target.EstimatedArrivalAt,
	}
}
