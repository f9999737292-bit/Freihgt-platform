package tracking

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	sharedmiddleware "github.com/freight-platform/shared-go/middleware"
)

type ETATargetSummary struct {
	Status                    string     `json:"status"`
	EstimatedArrivalAt        *time.Time `json:"estimatedArrivalAt,omitempty"`
	SourceType                *string    `json:"sourceType,omitempty"`
	Provider                  *string    `json:"provider,omitempty"`
	SourceObservedAt          *time.Time `json:"sourceObservedAt,omitempty"`
	ReceivedAt                *time.Time `json:"receivedAt,omitempty"`
	AgeSeconds                *int64     `json:"ageSeconds,omitempty"`
	FreshnessStatus           string     `json:"freshnessStatus"`
	QualityStatus             string     `json:"qualityStatus"`
	QualityReasons            []string   `json:"qualityReasons,omitempty"`
	ProviderConfidence        *float64   `json:"providerConfidence,omitempty"`
	DeliveryLagSeconds        *int64     `json:"deliveryLagSeconds,omitempty"`
	PlannedArrivalAt          *time.Time `json:"plannedArrivalAt,omitempty"`
	ProjectedDeviationSeconds *int64     `json:"projectedDeviationSeconds,omitempty"`
	ArrivalProjection         string     `json:"arrivalProjection"`
}

type ShipmentETASummary struct {
	ShipmentID string            `json:"shipmentId"`
	Delivery   *ETATargetSummary `json:"delivery,omitempty"`
	Pickup     *ETATargetSummary `json:"pickup,omitempty"`
}

type ETALookupPlanned struct {
	PlannedPickupAt   *string `json:"plannedPickupAt,omitempty"`
	PlannedDeliveryAt *string `json:"plannedDeliveryAt,omitempty"`
	ActualDeliveryAt  *string `json:"actualDeliveryAt,omitempty"`
	ActualPickupAt    *string `json:"actualPickupAt,omitempty"`
	ShipmentStatus    string  `json:"shipmentStatus"`
}

func (c *Client) GetETA(ctx context.Context, tenantID, requestID, shipmentID string, query string) (*ShipmentETASummary, error) {
	endpoint := fmt.Sprintf("%s/v1/shipments/%s/eta", c.baseURL, shipmentID)
	if query != "" {
		endpoint += "?" + query
	}
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
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 4096))
		return nil, fmt.Errorf("eta service status %d", resp.StatusCode)
	}
	var payload shipmentETADTO
	if err := json.NewDecoder(io.LimitReader(resp.Body, 1<<20)).Decode(&payload); err != nil {
		return nil, err
	}
	return payload.toSummary(), nil
}

func (c *Client) LookupDeliveryETA(ctx context.Context, tenantID, requestID string, shipmentIDs []string, planned map[string]ETALookupPlanned) (map[string]ETATargetSummary, error) {
	if len(shipmentIDs) == 0 {
		return map[string]ETATargetSummary{}, nil
	}
	payload, _ := json.Marshal(map[string]any{
		"shipmentIds": shipmentIDs,
		"planned":     planned,
	})
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/internal/v1/tracking/eta/lookup", bytes.NewReader(payload))
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
		return nil, fmt.Errorf("eta lookup status %d", resp.StatusCode)
	}
	var body struct {
		Items map[string]etaTargetDTO `json:"items"`
	}
	if err := json.NewDecoder(io.LimitReader(resp.Body, 2<<20)).Decode(&body); err != nil {
		return nil, err
	}
	out := make(map[string]ETATargetSummary, len(body.Items))
	for id, item := range body.Items {
		out[id] = item.toSummary()
	}
	return out, nil
}

type shipmentETADTO struct {
	ShipmentID string         `json:"shipmentId"`
	Delivery   *etaTargetDTO  `json:"delivery"`
	Pickup     *etaTargetDTO  `json:"pickup"`
}

type etaTargetDTO struct {
	Status                    string   `json:"status"`
	EstimatedArrivalAt        *string  `json:"estimatedArrivalAt"`
	SourceType                *string  `json:"sourceType"`
	Provider                  *string  `json:"provider"`
	SourceObservedAt          *string  `json:"sourceObservedAt"`
	ReceivedAt                *string  `json:"receivedAt"`
	AgeSeconds                *int64   `json:"ageSeconds"`
	FreshnessStatus           string   `json:"freshnessStatus"`
	QualityStatus             string   `json:"qualityStatus"`
	QualityReasons            []string `json:"qualityReasons"`
	ProviderConfidence        *float64 `json:"providerConfidence"`
	DeliveryLagSeconds        *int64   `json:"deliveryLagSeconds"`
	PlannedArrivalAt          *string  `json:"plannedArrivalAt"`
	ProjectedDeviationSeconds *int64   `json:"projectedDeviationSeconds"`
	ArrivalProjection         string   `json:"arrivalProjection"`
}

func (d shipmentETADTO) toSummary() *ShipmentETASummary {
	s := &ShipmentETASummary{ShipmentID: d.ShipmentID}
	if d.Delivery != nil {
		item := d.Delivery.toSummary()
		s.Delivery = &item
	}
	if d.Pickup != nil {
		item := d.Pickup.toSummary()
		s.Pickup = &item
	}
	return s
}

func (d etaTargetDTO) toSummary() ETATargetSummary {
	s := ETATargetSummary{
		Status:            d.Status,
		FreshnessStatus:   d.FreshnessStatus,
		QualityStatus:     d.QualityStatus,
		QualityReasons:    d.QualityReasons,
		ProviderConfidence: d.ProviderConfidence,
		ArrivalProjection: d.ArrivalProjection,
		AgeSeconds:        d.AgeSeconds,
		DeliveryLagSeconds: d.DeliveryLagSeconds,
		ProjectedDeviationSeconds: d.ProjectedDeviationSeconds,
		SourceType:        d.SourceType,
		Provider:          d.Provider,
	}
	s.EstimatedArrivalAt = parseTimePtr(d.EstimatedArrivalAt)
	s.SourceObservedAt = parseTimePtr(d.SourceObservedAt)
	s.ReceivedAt = parseTimePtr(d.ReceivedAt)
	s.PlannedArrivalAt = parseTimePtr(d.PlannedArrivalAt)
	return s
}

func parseTimePtr(raw *string) *time.Time {
	if raw == nil || *raw == "" {
		return nil
	}
	t, err := time.Parse(time.RFC3339, *raw)
	if err != nil {
		return nil
	}
	utc := t.UTC()
	return &utc
}

func FormatTimePtr(t *time.Time) *string {
	if t == nil {
		return nil
	}
	s := t.UTC().Format(time.RFC3339)
	return &s
}
