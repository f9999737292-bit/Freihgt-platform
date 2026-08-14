package provider

import (
	"context"
	"encoding/json"
	"time"
)

type NormalizedSlotInput struct {
	ProviderDeviceID   string
	ProviderSlotID     *string
	ProviderVersion    *string
	SlotType           string
	FacilityID         *string
	LocationID         *string
	WindowStart        time.Time
	WindowEnd          time.Time
	Timezone           *string
	SlotStatus         string
	SourceObservedAt   time.Time
	SourceType         string
	BookedAt           *time.Time
	ConfirmedAt        *time.Time
	CancelledAt        *time.Time
}

type SlotProviderAdapter interface {
	ProviderCode() string
	NormalizeSlot(ctx context.Context, payload ProviderPayload) ([]NormalizedSlotInput, error)
}

type genericSlotPayload struct {
	Observations []genericSlotObservation `json:"observations"`
}

type genericSlotObservation struct {
	ProviderDeviceID string  `json:"providerDeviceId"`
	ProviderSlotID   *string `json:"providerSlotId"`
	ProviderVersion  *string `json:"providerVersion"`
	SlotType         string  `json:"slotType"`
	FacilityID       *string `json:"facilityId"`
	LocationID       *string `json:"locationId"`
	WindowStart      string  `json:"windowStart"`
	WindowEnd        string  `json:"windowEnd"`
	Timezone         *string `json:"timezone"`
	SlotStatus       string  `json:"slotStatus"`
	SourceObservedAt string  `json:"sourceObservedAt"`
	SourceType       string  `json:"sourceType"`
	BookedAt         *string `json:"bookedAt"`
	ConfirmedAt      *string `json:"confirmedAt"`
	CancelledAt      *string `json:"cancelledAt"`
}

type GenericSlotAdapter struct{}

func (GenericSlotAdapter) ProviderCode() string { return "generic" }

func (GenericSlotAdapter) NormalizeSlot(_ context.Context, payload ProviderPayload) ([]NormalizedSlotInput, error) {
	var body genericSlotPayload
	if err := json.Unmarshal(payload, &body); err != nil {
		return nil, err
	}
	out := make([]NormalizedSlotInput, 0, len(body.Observations))
	for _, item := range body.Observations {
		windowStart, err := time.Parse(time.RFC3339, item.WindowStart)
		if err != nil {
			return nil, err
		}
		windowEnd, err := time.Parse(time.RFC3339, item.WindowEnd)
		if err != nil {
			return nil, err
		}
		if !windowStart.Before(windowEnd) {
			continue
		}
		observedAt, err := time.Parse(time.RFC3339, item.SourceObservedAt)
		if err != nil {
			return nil, err
		}
		sourceType := item.SourceType
		if sourceType == "" {
			sourceType = "warehouse_api"
		}
		slotType := item.SlotType
		if slotType == "" {
			slotType = "delivery"
		}
		slotStatus := item.SlotStatus
		if slotStatus == "" {
			slotStatus = "booked"
		}
		out = append(out, NormalizedSlotInput{
			ProviderDeviceID: item.ProviderDeviceID,
			ProviderSlotID:   item.ProviderSlotID,
			ProviderVersion:  item.ProviderVersion,
			SlotType:         slotType,
			FacilityID:       item.FacilityID,
			LocationID:       item.LocationID,
			WindowStart:      windowStart.UTC(),
			WindowEnd:        windowEnd.UTC(),
			Timezone:         item.Timezone,
			SlotStatus:       slotStatus,
			SourceObservedAt: observedAt.UTC(),
			SourceType:       sourceType,
			BookedAt:         parseOptionalTime(item.BookedAt),
			ConfirmedAt:      parseOptionalTime(item.ConfirmedAt),
			CancelledAt:      parseOptionalTime(item.CancelledAt),
		})
	}
	return out, nil
}

func parseOptionalTime(raw *string) *time.Time {
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

type SlotRegistry struct {
	adapters map[string]SlotProviderAdapter
}

func NewSlotRegistry(adapters ...SlotProviderAdapter) *SlotRegistry {
	m := make(map[string]SlotProviderAdapter, len(adapters))
	for _, a := range adapters {
		m[a.ProviderCode()] = a
	}
	return &SlotRegistry{adapters: m}
}

func (r *SlotRegistry) Get(code string) (SlotProviderAdapter, bool) {
	a, ok := r.adapters[code]
	return a, ok
}
