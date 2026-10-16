package chainapi

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/freight-platform/network-optimizer-service/internal/chain"
	"github.com/freight-platform/network-optimizer-service/internal/currenttrip"
	"github.com/freight-platform/network-optimizer-service/internal/domain"
	"github.com/freight-platform/network-optimizer-service/internal/repository"
	"github.com/freight-platform/network-optimizer-service/internal/routeplan"
	"github.com/freight-platform/network-optimizer-service/internal/routing"
	"github.com/freight-platform/network-optimizer-service/internal/service"
)

func TestI2C_I271_003_dangerous_cargo_tristate(t *testing.T) {
	yes, no := true, false
	adr := true
	safe := confirmedCargo(&no)
	danger := confirmedCargo(&yes)
	unknown := confirmedCargo(nil)

	t.Run("I271_003_NO_EVIDENCE_IS_UNKNOWN", func(t *testing.T) {
		expectCargo(t, dangerousCargoState(currenttrip.ResolutionUnproven, nil), nil)
		expectCargo(t, dangerousCargoState("", nil), nil)
		expectCargo(t, dangerousCargoState(currenttrip.EvidenceConfirmedOnboard, nil), nil)
	})
	t.Run("I271_003_EXPLICIT_UNLOADED_IS_FALSE", func(t *testing.T) {
		expectCargo(t, dangerousCargoState(currenttrip.EvidenceUnloaded, nil), &no)
	})
	t.Run("I271_003_CONFIRMED_SAFE_IS_FALSE", func(t *testing.T) {
		expectCargo(t, dangerousCargoState(currenttrip.EvidenceConfirmedOnboard, []currenttrip.OnboardCargoUnit{safe, safe}), &no)
	})
	t.Run("I271_003_CONFIRMED_DANGEROUS_IS_TRUE", func(t *testing.T) {
		expectCargo(t, dangerousCargoState(currenttrip.EvidenceConfirmedOnboard, []currenttrip.OnboardCargoUnit{danger}), &yes)
	})
	t.Run("I271_003_DANGEROUS_PLUS_UNKNOWN_IS_TRUE", func(t *testing.T) {
		expectCargo(t, dangerousCargoState(currenttrip.EvidenceConfirmedOnboard, []currenttrip.OnboardCargoUnit{unknown, danger}), &yes)
	})
	t.Run("I271_003_SAFE_PLUS_UNKNOWN_IS_UNKNOWN", func(t *testing.T) {
		expectCargo(t, dangerousCargoState(currenttrip.EvidenceConfirmedOnboard, []currenttrip.OnboardCargoUnit{safe, unknown}), nil)
	})
	t.Run("I271_003_ADR_CAPABILITY_NOT_CARGO_STATE", func(t *testing.T) {
		withADR := routingProfileFromCanonical(&currenttrip.VehicleCapability{ADRCapability: &adr}, currenttrip.ResolutionUnproven, nil)
		without := routingProfileFromCanonical(&currenttrip.VehicleCapability{ADRCapability: boolPtr(false)}, currenttrip.EvidenceUnloaded, nil)
		if withADR.DangerousCargo != nil || without.DangerousCargo == nil || *without.DangerousCargo {
			t.Fatalf("adr=%v unloaded=%v", withADR.DangerousCargo, without.DangerousCargo)
		}
	})
	t.Run("I271_003_ROUTING_PROPAGATION", func(t *testing.T) {
		for _, want := range []*bool{&yes, &no, nil} {
			got := propagateCargo(t, routing.VehicleProfile{DangerousCargo: want})
			expectCargo(t, got, want)
		}
	})
}

func confirmedCargo(dangerous *bool) currenttrip.OnboardCargoUnit {
	return currenttrip.OnboardCargoUnit{
		EvidenceState: currenttrip.EvidenceConfirmedOnboard,
		Profile:       currenttrip.CargoProfile{DangerousGoods: dangerous},
	}
}

func expectCargo(t *testing.T, got, want *bool) {
	t.Helper()
	if want == nil {
		if got != nil {
			t.Fatalf("dangerous cargo=%v, want unknown", *got)
		}
		return
	}
	if got == nil || *got != *want {
		t.Fatalf("dangerous cargo=%v want=%v", got, *want)
	}
}

func boolPtr(v bool) *bool { return &v }

func propagateCargo(t *testing.T, profile routing.VehicleProfile) *bool {
	t.Helper()
	now := time.Date(2026, 10, 16, 12, 0, 0, 0, time.UTC)
	store := repository.NewMemory()
	viewer := uuid.New()
	company := uuid.New()
	actor := service.Actor{TenantID: viewer, CompanyID: &company}
	left := publishedLoad(uuid.New(), now, false, true, domain.VisMarketplace, nil)
	right := publishedLoad(uuid.New(), now, false, true, domain.VisMarketplace, nil)
	putLoad(t, store, left)
	putLoad(t, store, right)
	loads := mustLoads(t, MarketplacePool{Store: store}, actor)
	recorder := &captureRoute{}
	_, err := (chain.Engine{}).Search(context.Background(), chain.Request{
		TenantID: viewer,
		Seed: chain.Seed{
			Source:   chain.SeedCurrentTripResidual,
			Start:    routeplan.Point{Kind: routeplan.PointAnchor, Latitude: 1, Longitude: 2, Source: routeplan.SourceTracking},
			Capacity: chain.Capacity{Payload: chain.Dimension{Known: true, Value: 1000}, Volume: chain.Dimension{Known: true, Value: 1000}},
		},
		Loads: loads, VehicleProfile: profile, Route: recorder, Now: func() time.Time { return now },
	})
	if err != nil || recorder.calls == 0 {
		t.Fatalf("calls=%d err=%v", recorder.calls, err)
	}
	return recorder.req.VehicleProfile.DangerousCargo
}
