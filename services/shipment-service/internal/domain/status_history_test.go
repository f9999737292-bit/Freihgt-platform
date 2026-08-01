package domain

import (
	"testing"
	"time"

	"github.com/google/uuid"

	apperrors "github.com/freight-platform/shipment-service/internal/platform/errors"
)

func TestValidateStatusTransitionContextUserRequiresActorID(t *testing.T) {
	t.Parallel()
	err := ValidateStatusTransitionContext(StatusTransitionContext{
		ActorType:  ActorTypeUser,
		Source:     StatusHistorySourceShipmentService,
		OccurredAt: time.Now().UTC(),
	})
	if err == nil {
		t.Fatal("expected validation error")
	}
}

func TestValidateStatusTransitionContextUserRejectsZeroActorID(t *testing.T) {
	t.Parallel()
	zero := uuid.Nil
	err := ValidateStatusTransitionContext(StatusTransitionContext{
		ActorType:  ActorTypeUser,
		ActorID:    &zero,
		Source:     StatusHistorySourceShipmentService,
		OccurredAt: time.Now().UTC(),
	})
	if err == nil {
		t.Fatal("expected validation error for zero actor id")
	}
}

func TestValidateStatusTransitionContextSystemRejectsActorID(t *testing.T) {
	t.Parallel()
	actorID := uuid.New()
	err := ValidateStatusTransitionContext(StatusTransitionContext{
		ActorType:  ActorTypeSystem,
		ActorID:    &actorID,
		Source:     StatusHistorySourceShipmentService,
		OccurredAt: time.Now().UTC(),
	})
	if err == nil {
		t.Fatal("expected validation error")
	}
}

func TestValidateStatusTransitionContextSystemWithoutActorIDAllowed(t *testing.T) {
	t.Parallel()
	ctx := NewSystemTransitionContext(StatusHistorySourceShipmentService, nil, time.Now().UTC())
	if err := ValidateStatusTransitionContext(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestValidateStatusTransitionContextRejectsUnknownActorType(t *testing.T) {
	t.Parallel()
	err := ValidateStatusTransitionContext(StatusTransitionContext{
		ActorType:  ActorType("BOT"),
		Source:     StatusHistorySourceShipmentService,
		OccurredAt: time.Now().UTC(),
	})
	if err == nil {
		t.Fatal("expected validation error")
	}
}

func TestValidateStatusTransitionContextRejectsUnknownSource(t *testing.T) {
	t.Parallel()
	userID := uuid.New()
	err := ValidateStatusTransitionContext(StatusTransitionContext{
		ActorType:  ActorTypeUser,
		ActorID:    &userID,
		Source:     StatusHistorySource("CLIENT"),
		OccurredAt: time.Now().UTC(),
	})
	if err == nil {
		t.Fatal("expected validation error")
	}
}

func TestNewUserTransitionContextSetsActorFields(t *testing.T) {
	t.Parallel()
	userID := uuid.New()
	correlation := "req-1"
	ctx := NewUserTransitionContext(userID, &correlation, time.Now().UTC())
	if ctx.ActorType != ActorTypeUser || ctx.ActorID == nil || *ctx.ActorID != userID {
		t.Fatalf("unexpected user context: %#v", ctx)
	}
	if ctx.CorrelationID == nil || *ctx.CorrelationID != correlation {
		t.Fatalf("correlation=%v", ctx.CorrelationID)
	}
	if err := ValidateStatusTransitionContext(ctx); err != nil {
		t.Fatalf("unexpected validation error: %v", err)
	}
}

func TestNewSystemTransitionContextClearsActorID(t *testing.T) {
	t.Parallel()
	ctx := NewSystemTransitionContext(StatusHistorySourceShipmentService, nil, time.Now().UTC())
	if ctx.ActorType != ActorTypeSystem || ctx.ActorID != nil {
		t.Fatalf("unexpected system context: %#v", ctx)
	}
}

func TestValidateStatusTransitionContextRejectsZeroOccurredAt(t *testing.T) {
	t.Parallel()
	userID := uuid.New()
	err := ValidateStatusTransitionContext(StatusTransitionContext{
		ActorType: ActorTypeUser,
		ActorID:   &userID,
		Source:    StatusHistorySourceShipmentService,
	})
	var appErr *apperrors.AppError
	if err == nil {
		t.Fatal("expected validation error")
	}
	_ = appErr
}
