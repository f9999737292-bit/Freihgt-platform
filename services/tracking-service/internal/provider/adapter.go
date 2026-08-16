package provider

import (
	"context"
	"encoding/json"
	"time"
)

// ProviderPayload is opaque to the domain; each adapter interprets its shape.
type ProviderPayload []byte

type NormalizedLocationInput struct {
	ProviderDeviceID string
	ProviderEventID  *string
	Latitude         float64
	Longitude        float64
	RecordedAt       time.Time
	SpeedKph         *float64
	HeadingDegrees   *float64
	AccuracyMeters   *float64
	AltitudeMeters   *float64
	SourceType       string
	VehicleID        *string
	DriverID         *string
}

type TelemetryProviderAdapter interface {
	ProviderCode() string
	Normalize(ctx context.Context, payload ProviderPayload) ([]NormalizedLocationInput, error)
}

type Registry struct {
	adapters map[string]TelemetryProviderAdapter
}

func NewRegistry(adapters ...TelemetryProviderAdapter) *Registry {
	m := make(map[string]TelemetryProviderAdapter, len(adapters))
	for _, a := range adapters {
		m[a.ProviderCode()] = a
	}
	return &Registry{adapters: m}
}

func (r *Registry) Get(code string) (TelemetryProviderAdapter, bool) {
	a, ok := r.adapters[code]
	return a, ok
}

type genericPayload struct {
	Events []genericEvent `json:"events"`
}

type genericEvent struct {
	ProviderDeviceID string   `json:"providerDeviceId"`
	ProviderEventID  *string  `json:"providerEventId"`
	Latitude         float64  `json:"latitude"`
	Longitude        float64  `json:"longitude"`
	RecordedAt       string   `json:"recordedAt"`
	SpeedKph         *float64 `json:"speedKph"`
	HeadingDegrees   *float64 `json:"headingDegrees"`
	AccuracyMeters   *float64 `json:"accuracyMeters"`
	AltitudeMeters   *float64 `json:"altitudeMeters"`
	SourceType       string   `json:"sourceType"`
	VehicleID        *string  `json:"vehicleId"`
	DriverID         *string  `json:"driverId"`
}

type GenericAdapter struct{}

func (GenericAdapter) ProviderCode() string { return "generic" }

func (GenericAdapter) Normalize(_ context.Context, payload ProviderPayload) ([]NormalizedLocationInput, error) {
	var body genericPayload
	if err := json.Unmarshal(payload, &body); err != nil {
		return nil, err
	}
	out := make([]NormalizedLocationInput, 0, len(body.Events))
	for _, item := range body.Events {
		recordedAt, err := time.Parse(time.RFC3339, item.RecordedAt)
		if err != nil {
			return nil, err
		}
		sourceType := item.SourceType
		if sourceType == "" {
			sourceType = "vehicle_telematics"
		}
		out = append(out, NormalizedLocationInput{
			ProviderDeviceID: item.ProviderDeviceID,
			ProviderEventID:  item.ProviderEventID,
			Latitude:         item.Latitude,
			Longitude:        item.Longitude,
			RecordedAt:       recordedAt.UTC(),
			SpeedKph:         item.SpeedKph,
			HeadingDegrees:   item.HeadingDegrees,
			AccuracyMeters:   item.AccuracyMeters,
			AltitudeMeters:   item.AltitudeMeters,
			SourceType:       sourceType,
			VehicleID:        item.VehicleID,
			DriverID:         item.DriverID,
		})
	}
	return out, nil
}

// DriverMobileAdapter is an extension point for future driver-app ingestion (v0.7.0 hook only).
type DriverMobileAdapter struct{}

func (DriverMobileAdapter) ProviderCode() string { return "driver_mobile" }

func (DriverMobileAdapter) Normalize(ctx context.Context, payload ProviderPayload) ([]NormalizedLocationInput, error) {
	return GenericAdapter{}.Normalize(ctx, payload)
}
