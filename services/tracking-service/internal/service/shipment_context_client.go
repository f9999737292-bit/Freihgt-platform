package service

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"
)

type ShipmentContextClient struct {
	baseURL string
	token   string
	http    *http.Client
}

func NewShipmentContextClient(baseURL, token string) *ShipmentContextClient {
	return &ShipmentContextClient{
		baseURL: strings.TrimRight(baseURL, "/"),
		token:   token,
		http:    &http.Client{Timeout: 5 * time.Second},
	}
}

func (c *ShipmentContextClient) Get(ctx context.Context, operatingTenantID, executionID, stopID uuid.UUID) (StopContext, error) {
	if c == nil || c.baseURL == "" || strings.TrimSpace(c.token) == "" {
		return StopContext{}, fmt.Errorf("shipment tracking context client is not configured")
	}
	url := fmt.Sprintf("%s/internal/v1/transport-executions/%s/stops/%s/tracking-context", c.baseURL, executionID, stopID)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return StopContext{}, err
	}
	req.Header.Set("X-Internal-Service-Token", c.token)
	req.Header.Set("X-Tenant-ID", operatingTenantID.String())
	resp, err := c.http.Do(req)
	if err != nil {
		return StopContext{}, err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if resp.StatusCode == http.StatusNotFound {
		return StopContext{}, ContextNotFoundError{}
	}
	if resp.StatusCode != http.StatusOK {
		return StopContext{}, fmt.Errorf("tracking context status %d", resp.StatusCode)
	}
	var payload struct {
		ExecutionID           uuid.UUID  `json:"executionId"`
		RevisionID            uuid.UUID  `json:"revisionId"`
		ExecutionStopID       uuid.UUID  `json:"executionStopId"`
		Ordinal               int        `json:"ordinal"`
		Status                string     `json:"status"`
		PlannedArrival        *time.Time `json:"plannedArrival"`
		LocationID            *uuid.UUID `json:"locationId"`
		TargetLatitude        *float64   `json:"targetLatitude"`
		TargetLongitude       *float64   `json:"targetLongitude"`
		DriverID              *uuid.UUID `json:"driverId"`
		VehicleID             *uuid.UUID `json:"vehicleId"`
		PointKind             string     `json:"pointKind"`
		StopRole              string     `json:"stopRole"`
		LiveETAStopID         *uuid.UUID `json:"liveEtaStopId"`
		LiveETAOrdinal        *int       `json:"liveEtaOrdinal"`
		LiveETAPlannedArrival *time.Time `json:"liveEtaPlannedArrival"`
		LiveETALocationID     *uuid.UUID `json:"liveEtaLocationId"`
		LiveETALatitude       *float64   `json:"liveEtaLatitude"`
		LiveETALongitude      *float64   `json:"liveEtaLongitude"`
	}
	if err := json.Unmarshal(body, &payload); err != nil {
		return StopContext{}, err
	}
	return StopContext{
		ExecutionID: payload.ExecutionID, RevisionID: payload.RevisionID, ExecutionStopID: payload.ExecutionStopID,
		Ordinal: payload.Ordinal, Status: payload.Status, PlannedArrival: payload.PlannedArrival,
		LocationID: payload.LocationID, TargetLatitude: payload.TargetLatitude, TargetLongitude: payload.TargetLongitude,
		DriverID: payload.DriverID, VehicleID: payload.VehicleID, PointKind: payload.PointKind, StopRole: payload.StopRole,
		LiveETAStopID: payload.LiveETAStopID, LiveETAOrdinal: payload.LiveETAOrdinal,
		LiveETAPlannedArrival: payload.LiveETAPlannedArrival, LiveETALocationID: payload.LiveETALocationID,
		LiveETALatitude: payload.LiveETALatitude, LiveETALongitude: payload.LiveETALongitude,
	}, nil
}
