package tracking

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	sharedmiddleware "github.com/freight-platform/shared-go/middleware"
)

type Client struct {
	httpClient *http.Client
	baseURL    string
	token      string
}

func NewClient(httpClient *http.Client, baseURL, internalToken string) *Client {
	if httpClient == nil {
		httpClient = &http.Client{Timeout: 5 * time.Second}
	}
	return &Client{
		httpClient: httpClient,
		baseURL:    strings.TrimRight(baseURL, "/"),
		token:      internalToken,
	}
}

type Summary struct {
	ShipmentID         string              `json:"shipmentId"`
	TrackingStatus     string              `json:"trackingStatus"`
	Provider           *string             `json:"provider,omitempty"`
	LastKnownPosition  *LastKnownPosition  `json:"lastKnownPosition,omitempty"`
	Freshness          FreshnessSummary    `json:"freshness"`
	Quality            QualitySummary      `json:"quality"`
	LastRecordedAt     *time.Time          `json:"lastRecordedAt,omitempty"`
	LastReceivedAt     *time.Time          `json:"lastReceivedAt,omitempty"`
	SpeedKph           *float64            `json:"speedKph,omitempty"`
	HeadingDegrees     *float64            `json:"headingDegrees,omitempty"`
	DeliveryDelaySeconds *int64            `json:"deliveryDelaySeconds,omitempty"`
}

type LastKnownPosition struct {
	Latitude   float64   `json:"latitude"`
	Longitude  float64   `json:"longitude"`
	RecordedAt time.Time `json:"recordedAt"`
	AgeSeconds int64     `json:"ageSeconds"`
}

type FreshnessSummary struct {
	Status     string `json:"status"`
	AgeSeconds *int64 `json:"ageSeconds,omitempty"`
}

type QualitySummary struct {
	Status string  `json:"status"`
	Reason *string `json:"reason,omitempty"`
}

func (c *Client) GetCurrent(ctx context.Context, tenantID, requestID, shipmentID string) (*Summary, error) {
	endpoint := fmt.Sprintf("%s/v1/shipments/%s/tracking", c.baseURL, shipmentID)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("X-Tenant-ID", tenantID)
	if requestID != "" {
		req.Header.Set(sharedmiddleware.RequestIDHeader, requestID)
	}
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusNotFound {
		return nil, nil
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 4096))
		return nil, fmt.Errorf("tracking service status %d", resp.StatusCode)
	}
	var payload summaryDTO
	if err := json.NewDecoder(io.LimitReader(resp.Body, 1<<20)).Decode(&payload); err != nil {
		return nil, err
	}
	return payload.toSummary(), nil
}

func (c *Client) ProxyJSON(ctx context.Context, tenantID, requestID, method, path string, query string) (json.RawMessage, int, error) {
	endpoint := c.baseURL + path
	if query != "" {
		endpoint += "?" + query
	}
	req, err := http.NewRequestWithContext(ctx, method, endpoint, nil)
	if err != nil {
		return nil, 0, err
	}
	req.Header.Set("X-Tenant-ID", tenantID)
	if requestID != "" {
		req.Header.Set(sharedmiddleware.RequestIDHeader, requestID)
	}
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, 0, err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return nil, resp.StatusCode, err
	}
	return json.RawMessage(body), resp.StatusCode, nil
}

func (c *Client) LookupStates(ctx context.Context, tenantID, requestID string, shipmentIDs []string) (map[string]Summary, error) {
	if len(shipmentIDs) == 0 {
		return map[string]Summary{}, nil
	}
	payload, _ := json.Marshal(map[string]any{"shipmentIds": shipmentIDs})
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/internal/v1/tracking/states/lookup", bytes.NewReader(payload))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Tenant-ID", tenantID)
	req.Header.Set("X-Internal-Service-Token", c.token)
	if requestID != "" {
		req.Header.Set(sharedmiddleware.RequestIDHeader, requestID)
	}
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 4096))
		return nil, fmt.Errorf("tracking lookup status %d", resp.StatusCode)
	}
	var body struct {
		Items map[string]summaryDTO `json:"items"`
	}
	if err := json.NewDecoder(io.LimitReader(resp.Body, 2<<20)).Decode(&body); err != nil {
		return nil, err
	}
	out := make(map[string]Summary, len(body.Items))
	for id, item := range body.Items {
		if summary := item.toSummary(); summary != nil {
			out[id] = *summary
		}
	}
	return out, nil
}

func (c *Client) IngestDriverLocation(ctx context.Context, tenantID, requestID, shipmentID, driverID, vehicleID string, body []byte) (json.RawMessage, int, error) {
	endpoint := fmt.Sprintf("%s/internal/v1/tracking/driver/shipments/%s/locations", c.baseURL, shipmentID)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return nil, 0, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Tenant-ID", tenantID)
	req.Header.Set("X-Driver-ID", driverID)
	if vehicleID != "" {
		req.Header.Set("X-Vehicle-ID", vehicleID)
	}
	req.Header.Set("X-Internal-Service-Token", c.token)
	if requestID != "" {
		req.Header.Set(sharedmiddleware.RequestIDHeader, requestID)
	}
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, 0, err
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return nil, resp.StatusCode, err
	}
	return json.RawMessage(raw), resp.StatusCode, nil
}

type summaryDTO struct {
	ShipmentID         string             `json:"shipmentId"`
	TrackingStatus     string             `json:"trackingStatus"`
	Provider           *string            `json:"provider"`
	LastKnownPosition  *LastKnownPosition `json:"lastKnownPosition"`
	Freshness          FreshnessSummary   `json:"freshness"`
	Quality            QualitySummary     `json:"quality"`
	LastRecordedAt     *string            `json:"lastRecordedAt"`
	LastReceivedAt     *string            `json:"lastReceivedAt"`
	SpeedKph           *float64           `json:"speedKph"`
	HeadingDegrees     *float64           `json:"headingDegrees"`
	DeliveryDelaySeconds *int64           `json:"deliveryDelaySeconds"`
}

func (d summaryDTO) toSummary() *Summary {
	s := &Summary{
		ShipmentID:     d.ShipmentID,
		TrackingStatus: d.TrackingStatus,
		Provider:       d.Provider,
		LastKnownPosition: d.LastKnownPosition,
		Freshness:      d.Freshness,
		Quality:        d.Quality,
		SpeedKph:       d.SpeedKph,
		HeadingDegrees: d.HeadingDegrees,
		DeliveryDelaySeconds: d.DeliveryDelaySeconds,
	}
	if d.LastRecordedAt != nil {
		if parsed, err := time.Parse(time.RFC3339, *d.LastRecordedAt); err == nil {
			s.LastRecordedAt = &parsed
		}
	}
	if d.LastReceivedAt != nil {
		if parsed, err := time.Parse(time.RFC3339, *d.LastReceivedAt); err == nil {
			s.LastReceivedAt = &parsed
		}
	}
	return s
}
