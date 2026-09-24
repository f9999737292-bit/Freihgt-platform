package sourceclient

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

	"github.com/freight-platform/network-optimizer-service/internal/predict"
	"github.com/freight-platform/shared-go/internalauth"
	"github.com/freight-platform/shared-go/lowcode"
)

type HTTP struct {
	client       *http.Client
	shipmentURL  string
	trackingURL  string
	serviceToken string
}

func New(shipmentURL, trackingURL, serviceToken string) *HTTP {
	return &HTTP{
		client: &http.Client{
			Timeout: 5 * time.Second,
			CheckRedirect: func(*http.Request, []*http.Request) error {
				return http.ErrUseLastResponse
			},
		},
		shipmentURL:  strings.TrimRight(shipmentURL, "/"),
		trackingURL:  strings.TrimRight(trackingURL, "/"),
		serviceToken: strings.TrimSpace(serviceToken),
	}
}

func (h *HTTP) Shipment(ctx context.Context, tenant, id uuid.UUID) (predict.ShipmentFact, error) {
	if h.shipmentURL == "" {
		return predict.ShipmentFact{}, predict.ErrUnavailable
	}
	body, err := h.get(ctx, tenant, fmt.Sprintf("%s/internal/v1/shipments/%s/prediction-input", h.shipmentURL, id))
	if err != nil {
		return predict.ShipmentFact{}, err
	}
	var payload struct {
		ID                          uuid.UUID  `json:"id"`
		TenantID                    uuid.UUID  `json:"tenant_id"`
		Status                      string     `json:"status"`
		Version                     int        `json:"version"`
		VehicleID                   *uuid.UUID `json:"vehicle_id"`
		DestinationLocationID       uuid.UUID  `json:"destination_location_id"`
		DestinationLatitude         *float64   `json:"destination_latitude"`
		DestinationLongitude        *float64   `json:"destination_longitude"`
		PlannedDeliveryAt           *time.Time `json:"planned_delivery_at"`
		OtherActiveVehicleShipments int        `json:"other_active_vehicle_shipments"`
	}
	if err := json.Unmarshal(body, &payload); err != nil || payload.ID != id || payload.TenantID != tenant {
		return predict.ShipmentFact{}, predict.ErrUnavailable
	}
	return predict.ShipmentFact{
		ID: payload.ID, TenantID: payload.TenantID, Status: payload.Status, Version: payload.Version,
		VehicleID: payload.VehicleID, DestinationLocationID: payload.DestinationLocationID,
		DestinationLatitude: payload.DestinationLatitude, DestinationLongitude: payload.DestinationLongitude,
		PlannedDeliveryAt: payload.PlannedDeliveryAt, OtherActiveAssignments: payload.OtherActiveVehicleShipments,
	}, nil
}

func (h *HTTP) Vehicle(ctx context.Context, tenant, id uuid.UUID) (predict.VehicleFact, error) {
	if h.shipmentURL == "" {
		return predict.VehicleFact{}, predict.ErrUnavailable
	}
	body, err := h.get(ctx, tenant, fmt.Sprintf("%s/internal/v1/vehicles/%s/capability", h.shipmentURL, id))
	if err != nil {
		return predict.VehicleFact{}, err
	}
	var payload struct {
		ID                            uuid.UUID `json:"id"`
		TenantID                      uuid.UUID `json:"tenant_id"`
		Version                       int       `json:"version"`
		EquipmentType                 *string   `json:"equipment_type"`
		CombinationType               *string   `json:"combination_type"`
		BodyType                      *string   `json:"body_type"`
		LoadingAccess                 []string  `json:"loading_access"`
		UnloadingAccess               []string  `json:"unloading_access"`
		CapacityWeight                *float64  `json:"capacity_weight"`
		CapacityVolume                *float64  `json:"capacity_volume"`
		TemperatureControlMode        *string   `json:"temperature_control_mode"`
		TemperatureCapabilityMinC     *float64  `json:"temperature_capability_min_c"`
		TemperatureCapabilityMaxC     *float64  `json:"temperature_capability_max_c"`
		TemperatureZoneCount          *int      `json:"temperature_zone_count"`
		IndependentTemperatureControl *bool     `json:"independent_temperature_control"`
		ContainerSize                 *string   `json:"container_size"`
	}
	if err := json.Unmarshal(body, &payload); err != nil || payload.ID != id || payload.TenantID != tenant {
		return predict.VehicleFact{}, predict.ErrUnavailable
	}
	return predict.VehicleFact{
		ID: payload.ID, Version: payload.Version, LegacyEquipmentType: payload.EquipmentType,
		CombinationType: payload.CombinationType, BodyType: payload.BodyType,
		LoadingAccess: payload.LoadingAccess, UnloadingAccess: payload.UnloadingAccess,
		CapacityWeightKg: payload.CapacityWeight, CapacityVolumeM3: payload.CapacityVolume,
		TemperatureControlMode:    payload.TemperatureControlMode,
		TemperatureCapabilityMinC: payload.TemperatureCapabilityMinC, TemperatureCapabilityMaxC: payload.TemperatureCapabilityMaxC,
		TemperatureZoneCount: payload.TemperatureZoneCount, IndependentTemperatureControl: payload.IndependentTemperatureControl,
		ContainerSize: payload.ContainerSize,
	}, nil
}

func (h *HTTP) ETA(ctx context.Context, tenant, shipment uuid.UUID) (predict.ETAFact, error) {
	if h.trackingURL == "" {
		return predict.ETAFact{}, predict.ErrUnavailable
	}
	raw, err := json.Marshal(map[string]any{"shipmentIds": []string{shipment.String()}})
	if err != nil {
		return predict.ETAFact{}, predict.ErrUnavailable
	}
	body, err := h.send(ctx, tenant, http.MethodPost, h.trackingURL+"/internal/v1/tracking/eta/lookup", raw)
	if err != nil {
		return predict.ETAFact{}, err
	}
	var payload struct {
		Items map[string]struct {
			EstimatedArrivalAt *time.Time `json:"estimatedArrivalAt"`
			SourceObservedAt   *time.Time `json:"sourceObservedAt"`
		} `json:"items"`
	}
	if err := json.Unmarshal(body, &payload); err != nil {
		return predict.ETAFact{}, predict.ErrUnavailable
	}
	item, ok := payload.Items[shipment.String()]
	if !ok || item.EstimatedArrivalAt == nil || item.SourceObservedAt == nil {
		return predict.ETAFact{}, nil
	}
	return predict.ETAFact{Present: true, Arrival: item.EstimatedArrivalAt.UTC(), ObservedAt: item.SourceObservedAt.UTC()}, nil
}

func (h *HTTP) get(ctx context.Context, tenant uuid.UUID, endpoint string) ([]byte, error) {
	return h.send(ctx, tenant, http.MethodGet, endpoint, nil)
}

func (h *HTTP) send(ctx context.Context, tenant uuid.UUID, method, endpoint string, body []byte) ([]byte, error) {
	if h.serviceToken == "" {
		return nil, predict.ErrUnavailable
	}
	var reader io.Reader
	if body != nil {
		reader = bytes.NewReader(body)
	}
	req, err := http.NewRequestWithContext(ctx, method, endpoint, reader)
	if err != nil {
		return nil, predict.ErrUnavailable
	}
	req.Header.Set(lowcode.HeaderTenantID, tenant.String())
	req.Header.Set(internalauth.HeaderName, h.serviceToken)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := h.client.Do(req)
	if err != nil {
		return nil, predict.ErrUnavailable
	}
	defer resp.Body.Close()
	payload, err := io.ReadAll(io.LimitReader(resp.Body, 1<<16))
	if err != nil {
		return nil, predict.ErrUnavailable
	}
	switch resp.StatusCode {
	case http.StatusOK:
		return payload, nil
	case http.StatusNotFound, http.StatusForbidden:
		return nil, predict.ErrNotFound
	default:
		return nil, predict.ErrUnavailable
	}
}
