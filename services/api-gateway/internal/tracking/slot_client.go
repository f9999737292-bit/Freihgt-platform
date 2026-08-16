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

type SlotTargetSummary struct {
	WindowStatus           string     `json:"windowStatus"`
	SlotStatus             *string    `json:"slotStatus,omitempty"`
	WindowStart            *time.Time `json:"windowStart,omitempty"`
	WindowEnd              *time.Time `json:"windowEnd,omitempty"`
	Timezone               *string    `json:"timezone,omitempty"`
	SourceType             *string    `json:"sourceType,omitempty"`
	Provider               *string    `json:"provider,omitempty"`
	SourceObservedAt       *time.Time `json:"sourceObservedAt,omitempty"`
	QualityStatus          string     `json:"qualityStatus"`
	ArrivalProjection      string     `json:"arrivalProjection"`
	ProjectedLateBySeconds *int64     `json:"projectedLateBySeconds,omitempty"`
	EarlyBySeconds         *int64     `json:"earlyBySeconds,omitempty"`
	MarginSeconds          *int64     `json:"marginSeconds,omitempty"`
	ETARelation            string     `json:"etaRelation"`
}

type ShipmentSlotSummary struct {
	ShipmentID string             `json:"shipmentId"`
	Pickup     *SlotTargetSummary `json:"pickup,omitempty"`
	Delivery   *SlotTargetSummary `json:"delivery,omitempty"`
}

type SlotLookupContext struct {
	ShipmentStatus   string         `json:"shipmentStatus"`
	ActualPickupAt   *string        `json:"actualPickupAt,omitempty"`
	ActualDeliveryAt *string        `json:"actualDeliveryAt,omitempty"`
	PickupETA        *ETASnapshotIn `json:"pickupEta,omitempty"`
	DeliveryETA      *ETASnapshotIn `json:"deliveryEta,omitempty"`
}

type ETASnapshotIn struct {
	HasUsableETA       bool    `json:"hasUsableEta"`
	Status             string  `json:"status"`
	FreshnessStatus    string  `json:"freshnessStatus"`
	QualityStatus      string  `json:"qualityStatus"`
	EstimatedArrivalAt *string `json:"estimatedArrivalAt,omitempty"`
}

func (c *Client) GetSlots(ctx context.Context, tenantID, requestID, shipmentID string, query string) (*ShipmentSlotSummary, error) {
	endpoint := fmt.Sprintf("%s/v1/shipments/%s/slots", c.baseURL, shipmentID)
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
		return nil, fmt.Errorf("slot service status %d", resp.StatusCode)
	}
	var payload shipmentSlotDTO
	if err := json.NewDecoder(io.LimitReader(resp.Body, 1<<20)).Decode(&payload); err != nil {
		return nil, err
	}
	return payload.toSummary(), nil
}

func (c *Client) LookupSlots(ctx context.Context, tenantID, requestID string, shipmentIDs []string, context map[string]SlotLookupContext) (map[string]ShipmentSlotSummary, error) {
	if len(shipmentIDs) == 0 {
		return map[string]ShipmentSlotSummary{}, nil
	}
	payload, _ := json.Marshal(map[string]any{
		"shipmentIds": shipmentIDs,
		"context":     context,
	})
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/internal/v1/tracking/slots/lookup", bytes.NewReader(payload))
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
		return nil, fmt.Errorf("slot lookup status %d", resp.StatusCode)
	}
	var body struct {
		Items map[string]shipmentSlotDTO `json:"items"`
	}
	if err := json.NewDecoder(io.LimitReader(resp.Body, 2<<20)).Decode(&body); err != nil {
		return nil, err
	}
	out := make(map[string]ShipmentSlotSummary, len(body.Items))
	for id, item := range body.Items {
		if summary := item.toSummary(); summary != nil {
			out[id] = *summary
		}
	}
	return out, nil
}

type shipmentSlotDTO struct {
	ShipmentID string         `json:"shipmentId"`
	Pickup     *slotTargetDTO `json:"pickup"`
	Delivery   *slotTargetDTO `json:"delivery"`
}

type slotTargetDTO struct {
	WindowStatus           string   `json:"windowStatus"`
	SlotStatus             *string  `json:"slotStatus"`
	WindowStart            *string  `json:"windowStart"`
	WindowEnd              *string  `json:"windowEnd"`
	Timezone               *string  `json:"timezone"`
	SourceType             *string  `json:"sourceType"`
	Provider               *string  `json:"provider"`
	SourceObservedAt       *string  `json:"sourceObservedAt"`
	QualityStatus          string   `json:"qualityStatus"`
	ArrivalProjection      string   `json:"arrivalProjection"`
	ProjectedLateBySeconds *int64   `json:"projectedLateBySeconds"`
	EarlyBySeconds         *int64   `json:"earlyBySeconds"`
	MarginSeconds          *int64   `json:"marginSeconds"`
	ETARelation            string   `json:"etaRelation"`
}

func (d shipmentSlotDTO) toSummary() *ShipmentSlotSummary {
	s := &ShipmentSlotSummary{ShipmentID: d.ShipmentID}
	if d.Pickup != nil {
		item := d.Pickup.toSummary()
		s.Pickup = &item
	}
	if d.Delivery != nil {
		item := d.Delivery.toSummary()
		s.Delivery = &item
	}
	return s
}

func (d slotTargetDTO) toSummary() SlotTargetSummary {
	s := SlotTargetSummary{
		WindowStatus:           d.WindowStatus,
		SlotStatus:             d.SlotStatus,
		Timezone:               d.Timezone,
		QualityStatus:          d.QualityStatus,
		ArrivalProjection:      d.ArrivalProjection,
		ProjectedLateBySeconds: d.ProjectedLateBySeconds,
		EarlyBySeconds:         d.EarlyBySeconds,
		MarginSeconds:          d.MarginSeconds,
		ETARelation:            d.ETARelation,
		SourceType:             d.SourceType,
		Provider:               d.Provider,
	}
	s.WindowStart = parseTimePtr(d.WindowStart)
	s.WindowEnd = parseTimePtr(d.WindowEnd)
	s.SourceObservedAt = parseTimePtr(d.SourceObservedAt)
	return s
}
