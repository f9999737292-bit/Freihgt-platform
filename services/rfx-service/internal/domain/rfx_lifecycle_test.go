package domain

import (
	"errors"
	"testing"

	"github.com/google/uuid"

	apperrors "github.com/freight-platform/rfx-service/internal/platform/errors"
)

func TestLifecycleProfileForType(t *testing.T) {
	t.Parallel()
	if LifecycleProfileForType("SPOT_RFQ") != LifecycleProfileSpot {
		t.Fatal("expected spot profile")
	}
	if LifecycleProfileForType("RFQ") != LifecycleProfileLongForm {
		t.Fatal("expected long-form profile")
	}
}

func TestResolveRfxTransitionTargetSpot(t *testing.T) {
	t.Parallel()
	target, err := ResolveRfxTransitionTarget(LifecycleProfileSpot, RfxStatusDraft, RfxCommandPublish)
	if err != nil || target != RfxStatusPublished {
		t.Fatalf("unexpected target=%s err=%v", target, err)
	}
}

func TestResolveRfxTransitionTargetInvalid(t *testing.T) {
	t.Parallel()
	_, err := ResolveRfxTransitionTarget(LifecycleProfileSpot, RfxStatusDraft, RfxCommandAward)
	if err == nil {
		t.Fatal("expected conflict")
	}
	var appErr *apperrors.AppError
	if !errors.As(err, &appErr) || appErr.Code != apperrors.CodeConflict {
		t.Fatalf("expected conflict, got %v", err)
	}
}

func TestValidatePublishRfxEventWithLotsLongForm(t *testing.T) {
	t.Parallel()
	event := &RfxEvent{Status: RfxStatusDraft, Title: "T", OwnerCompanyID: uuid.New(), RfxType: "RFQ"}
	if err := ValidatePublishRfxEventWithLots(event, 0); err == nil {
		t.Fatal("expected validation for missing lots")
	}
	if err := ValidatePublishRfxEventWithLots(event, 1); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestValidateCancelRfxEventExtended(t *testing.T) {
	t.Parallel()
	if err := ValidateCancelRfxEventExtended(RfxStatusResponsesOpen); err != nil {
		t.Fatalf("expected cancel allowed, got %v", err)
	}
	if err := ValidateCancelRfxEventExtended(RfxStatusAwarded); err == nil {
		t.Fatal("expected conflict for awarded status")
	}
}
