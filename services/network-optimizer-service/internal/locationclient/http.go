package locationclient

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

	"github.com/freight-platform/network-optimizer-service/internal/domain"
	"github.com/freight-platform/shared-go/internalauth"
	"github.com/freight-platform/shared-go/lowcode"
)

var (
	ErrNotFound    = errors.New("location not found")
	ErrUnavailable = errors.New("location resolution unavailable")
)

type Directory interface {
	Projection(ctx context.Context, tenantID, locationID uuid.UUID) (domain.LocationSnapshot, error)
	SourceEndpoints(ctx context.Context, tenantID uuid.UUID, sourceType string, sourceID uuid.UUID) (origin, destination uuid.UUID, err error)
}

type HTTP struct {
	client            *http.Client
	transportOrderURL string
	shipmentURL       string
	serviceToken      string
}

func NewHTTP(transportOrderURL, shipmentURL, serviceToken string) *HTTP {
	return &HTTP{
		client: &http.Client{
			Timeout: 5 * time.Second,
			CheckRedirect: func(*http.Request, []*http.Request) error {
				return http.ErrUseLastResponse
			},
		},
		transportOrderURL: strings.TrimRight(transportOrderURL, "/"),
		shipmentURL:       strings.TrimRight(shipmentURL, "/"),
		serviceToken:      strings.TrimSpace(serviceToken),
	}
}

func (h *HTTP) Projection(ctx context.Context, tenantID, locationID uuid.UUID) (domain.LocationSnapshot, error) {
	if h.serviceToken == "" || h.transportOrderURL == "" {
		return domain.LocationSnapshot{}, ErrUnavailable
	}
	endpoint := fmt.Sprintf("%s/internal/v1/locations/%s", h.transportOrderURL, locationID)
	body, status, err := h.get(ctx, tenantID, endpoint)
	if err != nil {
		return domain.LocationSnapshot{}, err
	}
	if status == http.StatusNotFound || status == http.StatusForbidden {
		return domain.LocationSnapshot{}, ErrNotFound
	}
	if status != http.StatusOK {
		return domain.LocationSnapshot{}, ErrUnavailable
	}
	var doc struct {
		ID          uuid.UUID `json:"id"`
		CountryCode string    `json:"country_code"`
		Region      *string   `json:"region"`
		City        *string   `json:"city"`
		Lat         *float64  `json:"lat"`
		Lon         *float64  `json:"lon"`
		Timezone    string    `json:"timezone"`
		Status      string    `json:"status"`
		Version     int       `json:"version"`
	}
	if json.Unmarshal(body, &doc) != nil || doc.ID != locationID || doc.Status != "ACTIVE" {
		return domain.LocationSnapshot{}, ErrNotFound
	}
	snap := domain.LocationSnapshot{
		ID: doc.ID, CountryCode: doc.CountryCode, Latitude: doc.Lat, Longitude: doc.Lon,
		Timezone: doc.Timezone, Status: doc.Status, Version: doc.Version,
	}
	if doc.Region != nil {
		snap.Region = *doc.Region
	}
	if doc.City != nil {
		snap.City = *doc.City
	}
	return snap, nil
}

func (h *HTTP) SourceEndpoints(ctx context.Context, tenantID uuid.UUID, sourceType string, sourceID uuid.UUID) (uuid.UUID, uuid.UUID, error) {
	if h.serviceToken == "" {
		return uuid.Nil, uuid.Nil, ErrUnavailable
	}
	var endpoint string
	switch sourceType {
	case domain.SourceTransportOrder:
		if h.transportOrderURL == "" {
			return uuid.Nil, uuid.Nil, ErrUnavailable
		}
		endpoint = fmt.Sprintf("%s/internal/v1/transport-orders/%s/planning-locations", h.transportOrderURL, sourceID)
	case domain.SourceShipment:
		if h.shipmentURL == "" {
			return uuid.Nil, uuid.Nil, ErrUnavailable
		}
		endpoint = fmt.Sprintf("%s/internal/v1/shipments/%s/planning-locations", h.shipmentURL, sourceID)
	default:
		return uuid.Nil, uuid.Nil, ErrNotFound
	}
	body, status, err := h.get(ctx, tenantID, endpoint)
	if err != nil {
		return uuid.Nil, uuid.Nil, err
	}
	if status == http.StatusNotFound || status == http.StatusForbidden {
		return uuid.Nil, uuid.Nil, ErrNotFound
	}
	if status != http.StatusOK {
		return uuid.Nil, uuid.Nil, ErrUnavailable
	}
	var doc struct {
		TenantID              uuid.UUID `json:"tenant_id"`
		OriginLocationID      uuid.UUID `json:"origin_location_id"`
		DestinationLocationID uuid.UUID `json:"destination_location_id"`
	}
	if json.Unmarshal(body, &doc) != nil || doc.TenantID != tenantID || doc.OriginLocationID == uuid.Nil || doc.DestinationLocationID == uuid.Nil {
		return uuid.Nil, uuid.Nil, ErrNotFound
	}
	return doc.OriginLocationID, doc.DestinationLocationID, nil
}

func (h *HTTP) get(ctx context.Context, tenantID uuid.UUID, endpoint string) ([]byte, int, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, 0, ErrUnavailable
	}
	req.Header.Set(lowcode.HeaderTenantID, tenantID.String())
	req.Header.Set(internalauth.HeaderName, h.serviceToken)
	resp, err := h.client.Do(req)
	if err != nil {
		return nil, 0, ErrUnavailable
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<16))
	if err != nil {
		return nil, 0, ErrUnavailable
	}
	return body, resp.StatusCode, nil
}
