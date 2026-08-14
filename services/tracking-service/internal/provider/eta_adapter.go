package provider

import (
	"context"
	"encoding/json"
	"time"
)

type NormalizedETAInput struct {
	ProviderDeviceID   string
	ProviderEventID    *string
	TargetType         string
	TargetReference    *string
	EstimatedArrivalAt time.Time
	SourceObservedAt   time.Time
	SourceType         string
	ProviderConfidence *float64
}

type ETAProviderAdapter interface {
	ProviderCode() string
	NormalizeETA(ctx context.Context, payload ProviderPayload) ([]NormalizedETAInput, error)
}

type genericETAPayload struct {
	Observations []genericETAObservation `json:"observations"`
}

type genericETAObservation struct {
	ProviderDeviceID   string   `json:"providerDeviceId"`
	ProviderEventID    *string  `json:"providerEventId"`
	TargetType         string   `json:"targetType"`
	TargetReference    *string  `json:"targetReference"`
	EstimatedArrivalAt string   `json:"estimatedArrivalAt"`
	SourceObservedAt   string   `json:"sourceObservedAt"`
	SourceType         string   `json:"sourceType"`
	ProviderConfidence *float64 `json:"providerConfidence"`
}

type GenericETAAdapter struct{}

func (GenericETAAdapter) ProviderCode() string { return "generic" }

func (GenericETAAdapter) NormalizeETA(_ context.Context, payload ProviderPayload) ([]NormalizedETAInput, error) {
	var body genericETAPayload
	if err := json.Unmarshal(payload, &body); err != nil {
		return nil, err
	}
	out := make([]NormalizedETAInput, 0, len(body.Observations))
	for _, item := range body.Observations {
		estimatedAt, err := time.Parse(time.RFC3339, item.EstimatedArrivalAt)
		if err != nil {
			return nil, err
		}
		observedAt, err := time.Parse(time.RFC3339, item.SourceObservedAt)
		if err != nil {
			return nil, err
		}
		sourceType := item.SourceType
		if sourceType == "" {
			sourceType = "provider_eta"
		}
		targetType := item.TargetType
		if targetType == "" {
			targetType = "delivery"
		}
		out = append(out, NormalizedETAInput{
			ProviderDeviceID:   item.ProviderDeviceID,
			ProviderEventID:    item.ProviderEventID,
			TargetType:         targetType,
			TargetReference:    item.TargetReference,
			EstimatedArrivalAt: estimatedAt.UTC(),
			SourceObservedAt:   observedAt.UTC(),
			SourceType:         sourceType,
			ProviderConfidence: item.ProviderConfidence,
		})
	}
	return out, nil
}

type ETARegistry struct {
	adapters map[string]ETAProviderAdapter
}

func NewETARegistry(adapters ...ETAProviderAdapter) *ETARegistry {
	m := make(map[string]ETAProviderAdapter, len(adapters))
	for _, a := range adapters {
		m[a.ProviderCode()] = a
	}
	return &ETARegistry{adapters: m}
}

func (r *ETARegistry) Get(code string) (ETAProviderAdapter, bool) {
	a, ok := r.adapters[code]
	return a, ok
}
