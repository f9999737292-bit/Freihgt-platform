package repository

import (
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/freight-platform/shipment-service/internal/domain"
)

func TestStatusHistoryWriteFromTransitionSystemActorHasNullActorID(t *testing.T) {
	t.Parallel()
	transition := domain.NewSystemTransitionContext(domain.StatusHistorySourceShipmentService, nil, time.Now().UTC())
	write := statusHistoryWriteFromTransition(uuid.New(), uuid.New(), 1, nil, domain.ShipmentStatusCarrierAssigned, transition)
	if write.actorType != string(domain.ActorTypeSystem) {
		t.Fatalf("actorType=%s", write.actorType)
	}
	if write.actorID != nil {
		t.Fatal("SYSTEM write must not set actor_id")
	}
}

func TestStatusHistoryWriteFromTransitionUserActorPreservesUUID(t *testing.T) {
	t.Parallel()
	actorID := uuid.New()
	transition := domain.NewUserTransitionContext(actorID, nil, time.Now().UTC())
	write := statusHistoryWriteFromTransition(uuid.New(), uuid.New(), 1, nil, domain.ShipmentStatusCarrierAssigned, transition)
	if write.actorType != string(domain.ActorTypeUser) {
		t.Fatalf("actorType=%s", write.actorType)
	}
	if write.actorID == nil || *write.actorID != actorID {
		t.Fatal("USER write must preserve actor_id")
	}
}
