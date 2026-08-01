package repository

import (
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/freight-platform/shipment-service/internal/domain"
)

func TestShouldRecordStatusHistoryInitialTransition(t *testing.T) {
	t.Parallel()
	if !shouldRecordStatusHistory(nil, domain.ShipmentStatusCarrierAssigned) {
		t.Fatal("initial transition from nil must be recorded")
	}
}

func TestShouldRecordStatusHistorySkipsSameStatus(t *testing.T) {
	t.Parallel()
	from := domain.ShipmentStatusCarrierAssigned
	if shouldRecordStatusHistory(&from, domain.ShipmentStatusCarrierAssigned) {
		t.Fatal("same-status assign must not record history")
	}
}

func TestShouldRecordStatusHistoryRecordsRealChange(t *testing.T) {
	t.Parallel()
	from := domain.ShipmentStatusCarrierAssigned
	if !shouldRecordStatusHistory(&from, domain.ShipmentStatusAcceptedByCarrier) {
		t.Fatal("status change must be recorded")
	}
}

func TestStatusHistoryWriteFromTransitionUsesActorAndCorrelation(t *testing.T) {
	t.Parallel()
	tenantID := uuid.New()
	shipmentID := uuid.New()
	actorID := uuid.New()
	correlation := "req-123"
	from := domain.ShipmentStatusCarrierAssigned
	transition := domain.NewUserTransitionContext(actorID, &correlation, time.Now().UTC())
	write := statusHistoryWriteFromTransition(tenantID, shipmentID, 2, &from, domain.ShipmentStatusInTransit, transition)
	if write.tenantID != tenantID || write.shipmentID != shipmentID {
		t.Fatal("write must preserve tenant and shipment")
	}
	if write.shipmentVersion != 2 {
		t.Fatalf("version=%d want 2", write.shipmentVersion)
	}
	if write.fromStatus == nil || *write.fromStatus != from {
		t.Fatal("from status must be preserved")
	}
	if write.toStatus != domain.ShipmentStatusInTransit {
		t.Fatalf("toStatus=%s", write.toStatus)
	}
	if write.actorType != string(domain.ActorTypeUser) {
		t.Fatalf("actorType=%s", write.actorType)
	}
	if write.actorID == nil || *write.actorID != actorID {
		t.Fatal("actor ID must be preserved")
	}
	if write.correlationID == nil || *write.correlationID != correlation {
		t.Fatal("correlation ID must be preserved")
	}
}
