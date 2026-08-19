package domain

import (
	"testing"

	"github.com/google/uuid"
)

func TestDeriveSettlementActorKindFromCompanyType(t *testing.T) {
	t.Parallel()
	kind, err := DeriveSettlementActorKind("CARRIER", nil)
	if err != nil || kind != SettlementActorCarrier {
		t.Fatalf("got %q err=%v", kind, err)
	}
	kind, err = DeriveSettlementActorKind("SHIPPER", nil)
	if err != nil || kind != SettlementActorBuyer {
		t.Fatalf("got %q err=%v", kind, err)
	}
}

func TestResolveTrustedSettlementActorRejectsActorKindMismatch(t *testing.T) {
	t.Parallel()
	companyID := uuid.New()
	tenantID := uuid.New()
	userID := uuid.New()
	memberships := []UserCompanyMembership{{
		CompanyID: companyID, CompanyType: "CARRIER", RoleCodes: []string{"CARRIER_ADMIN"},
	}}
	_, err := ResolveTrustedSettlementActor(tenantID, userID, companyID, SettlementActorBuyer, memberships, false)
	if err == nil {
		t.Fatal("expected actor kind mismatch to be denied")
	}
}

func TestResolveTrustedSettlementActorAcceptsValidMembership(t *testing.T) {
	t.Parallel()
	companyID := uuid.New()
	tenantID := uuid.New()
	userID := uuid.New()
	memberships := []UserCompanyMembership{{
		CompanyID: companyID, CompanyType: "SHIPPER", RoleCodes: []string{"SHIPPER_ADMIN"},
	}}
	actor, err := ResolveTrustedSettlementActor(tenantID, userID, companyID, SettlementActorBuyer, memberships, false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if actor.ActorCompanyID != companyID || actor.ActorKind != SettlementActorBuyer {
		t.Fatalf("unexpected actor: %+v", actor)
	}
}
