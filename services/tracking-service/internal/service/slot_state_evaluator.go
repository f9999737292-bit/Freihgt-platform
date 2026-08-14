package service

import (
	"context"
	"sync"
	"time"

	"github.com/google/uuid"

	"github.com/freight-platform/tracking-service/internal/domain"
	"github.com/freight-platform/tracking-service/internal/repository"
)

type SlotStateEvaluator struct {
	repo *repository.SlotRepository
	mu   sync.Mutex
	last map[string]string
}

func NewSlotStateEvaluator(repo *repository.SlotRepository) *SlotStateEvaluator {
	return &SlotStateEvaluator{
		repo: repo,
		last: map[string]string{},
	}
}

func (e *SlotStateEvaluator) BuildStateFromRevision(
	tenantID, shipmentID uuid.UUID,
	slotType string,
	windowStart, windowEnd, sourceObservedAt, receivedAt time.Time,
	timezone *string,
	slotStatus, sourceType, providerCode string,
	facilityID, locationID *uuid.UUID,
	providerSlotID *string,
	qualityStatus string,
	bookedAt, confirmedAt *time.Time,
	now time.Time,
) domain.ShipmentSlotState {
	windowStatus := domain.SlotWindowUnavailable
	var slotStatusPtr *string
	var windowStartPtr *time.Time
	var windowEndPtr *time.Time
	if domain.IsActiveSlotStatus(slotStatus) || slotStatus == domain.SlotStatusCompleted || slotStatus == domain.SlotStatusMissed {
		windowStatus = domain.SlotWindowAvailable
		slotStatusCopy := slotStatus
		slotStatusPtr = &slotStatusCopy
		start := windowStart.UTC()
		end := windowEnd.UTC()
		windowStartPtr = &start
		windowEndPtr = &end
	}
	srcType := sourceType
	provider := providerCode
	return domain.ShipmentSlotState{
		TenantID:         tenantID,
		ShipmentID:       shipmentID,
		SlotType:         slotType,
		WindowStatus:     windowStatus,
		SlotStatus:       slotStatusPtr,
		WindowStart:      windowStartPtr,
		WindowEnd:        windowEndPtr,
		Timezone:         timezone,
		FacilityID:       facilityID,
		LocationID:       locationID,
		SourceType:       &srcType,
		ProviderCode:     &provider,
		ProviderSlotID:   providerSlotID,
		SourceObservedAt: &sourceObservedAt,
		ReceivedAt:       &receivedAt,
		QualityStatus:    qualityStatus,
		BookedAt:         bookedAt,
		ConfirmedAt:      confirmedAt,
		UpdatedAt:        now,
	}
}

func (e *SlotStateEvaluator) ShouldReplaceCurrent(current *domain.ShipmentSlotState, candidate domain.ShipmentSlotState) bool {
	if current == nil {
		return candidate.WindowStatus == domain.SlotWindowAvailable
	}
	return domain.ShouldReplaceSlotState(*current, candidate)
}

func (e *SlotStateEvaluator) RecordTransitionIfNeeded(
	ctx context.Context,
	tenantID, shipmentID uuid.UUID,
	slotType, transitionType, toStatus string,
	fromStatus *string,
	metadata map[string]any,
) {
	key := tenantID.String() + "|" + shipmentID.String() + "|" + slotType + "|" + transitionType + "|" + toStatus
	e.mu.Lock()
	if e.last[key] == toStatus {
		e.mu.Unlock()
		return
	}
	e.last[key] = toStatus
	e.mu.Unlock()

	_ = e.repo.InsertSlotTransition(ctx, domain.SlotStateTransition{
		ID:             uuid.New(),
		TenantID:       tenantID,
		ShipmentID:     shipmentID,
		SlotType:       slotType,
		TransitionType: transitionType,
		FromStatus:     fromStatus,
		ToStatus:       toStatus,
		Metadata:       metadata,
		OccurredAt:     time.Now().UTC(),
	})
}

func EvaluateSlotQuality(sourceType string, slotStatus string) (string, []string) {
	reasons := make([]string, 0)
	if slotStatus == domain.SlotStatusProposed {
		reasons = append(reasons, "unconfirmed_booking")
		return domain.SlotQualityDegraded, reasons
	}
	if sourceType == domain.SlotSourceManualOperator {
		reasons = append(reasons, "manual_source")
	}
	if len(reasons) == 0 {
		return domain.SlotQualityGood, reasons
	}
	return domain.SlotQualityGood, reasons
}
