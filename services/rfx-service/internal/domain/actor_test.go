package domain

import (
	"errors"
	"testing"

	"github.com/google/uuid"

	apperrors "github.com/freight-platform/rfx-service/internal/platform/errors"
)

func TestClassifyActorKindBuyerRoles(t *testing.T) {
	t.Parallel()
	kind := ClassifyActorKind(nil, []string{"PROCUREMENT_MANAGER"})
	if kind != ActorKindBuyer {
		t.Fatalf("expected buyer, got %v", kind)
	}
}

func TestClassifyActorKindCarrierRoles(t *testing.T) {
	t.Parallel()
	kind := ClassifyActorKind(nil, []string{"CARRIER_DISPATCHER"})
	if kind != ActorKindCarrier {
		t.Fatalf("expected carrier, got %v", kind)
	}
}

func TestResolveCarrierCompanyIDSingleMembership(t *testing.T) {
	t.Parallel()
	companyID := uuid.New()
	resolved, err := ResolveCarrierCompanyID(uuid.Nil, []uuid.UUID{companyID})
	if err != nil || resolved != companyID {
		t.Fatalf("resolved=%s err=%v", resolved, err)
	}
}

func TestResolveCarrierCompanyIDMismatchForbidden(t *testing.T) {
	t.Parallel()
	_, err := ResolveCarrierCompanyID(uuid.New(), []uuid.UUID{uuid.New()})
	if err == nil {
		t.Fatal("expected forbidden")
	}
	var appErr *apperrors.AppError
	if !errors.As(err, &appErr) || appErr.Code != apperrors.CodeForbidden {
		t.Fatalf("expected forbidden, got %v", err)
	}
}

func TestCanViewBidCarrierIsolation(t *testing.T) {
	t.Parallel()
	carrierA := uuid.New()
	carrierB := uuid.New()
	if CanViewBid(ActorKindCarrier, carrierA, carrierB) {
		t.Fatal("carrier must not view competitor bid")
	}
	if !CanViewBid(ActorKindBuyer, uuid.Nil, carrierB) {
		t.Fatal("buyer must view all bids")
	}
}
