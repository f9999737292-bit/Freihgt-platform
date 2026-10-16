package chainapi

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/freight-platform/network-optimizer-service/internal/chain"
	"github.com/freight-platform/network-optimizer-service/internal/chainorch"
	"github.com/freight-platform/network-optimizer-service/internal/currenttrip"
	"github.com/freight-platform/network-optimizer-service/internal/domain"
	"github.com/freight-platform/network-optimizer-service/internal/reference"
	"github.com/freight-platform/network-optimizer-service/internal/repository"
	"github.com/freight-platform/network-optimizer-service/internal/routeplan"
	"github.com/freight-platform/network-optimizer-service/internal/routing"
	"github.com/freight-platform/network-optimizer-service/internal/service"
)

func TestI2C_PRIV_public_projection_omits_durable_revenue(t *testing.T) {
	amount := int64(424242)
	currency := "EUR"
	raw, err := json.Marshal(projectCandidates([]repository.ChainCandidate{{
		CandidateID: uuid.New(), Ordinal: 1,
		FirstLoadID: uuid.New(), FirstLoadVersion: 1,
		SecondLoadID: uuid.New(), SecondLoadVersion: 1,
		FeasibilityStatus: "FEASIBLE",
		RevenueAmount:     &amount, RevenueCurrency: &currency,
		OmittedComponents: []string{}, ReasonCodes: []string{},
	}}))
	if err != nil {
		t.Fatal(err)
	}
	body := string(raw)
	for _, leaked := range []string{"revenue_amount", "revenue_currency", "424242", "contract", "shipper"} {
		if contains(body, leaked) {
			t.Fatalf("public projection leaked %s in %s", leaked, body)
		}
	}
}

func TestI2C_SEED_company_fail_closed(t *testing.T) {
	ctx := context.Background()
	store := repository.NewMemory()
	tenant := uuid.New()
	otherTenant := uuid.New()
	company := uuid.New()
	otherCompany := uuid.New()
	actor := service.Actor{TenantID: tenant, UserID: uuid.New(), CompanyID: &company}
	resolver := StoreSeedResolver{Store: store}

	owned := sampleCapacity(tenant, &company)
	putCapacity(t, store, owned)
	foreignCompany := sampleCapacity(tenant, &otherCompany)
	putCapacity(t, store, foreignCompany)
	nilCompany := sampleCapacity(tenant, nil)
	putCapacity(t, store, nilCompany)
	foreignTenant := sampleCapacity(otherTenant, &company)
	putCapacity(t, store, foreignTenant)

	t.Run("I2C-SEED-01 exact company allowed", func(t *testing.T) {
		resolved, err := resolver.ResolveManual(ctx, actor, owned.ID)
		if err != nil {
			t.Fatal(err)
		}
		if resolved.CapacityVersion == nil || *resolved.CapacityVersion != owned.Version {
			t.Fatalf("version %+v", resolved.CapacityVersion)
		}
	})

	unknown := publicSeedError(t, resolver, actor, uuid.New())
	wrong := publicSeedError(t, resolver, actor, foreignCompany.ID)
	missingCompany := publicSeedError(t, resolver, actor, nilCompany.ID)
	foreign := publicSeedError(t, resolver, actor, foreignTenant.ID)
	if unknown.Status != 422 || unknown.Message != "seed is not eligible" {
		t.Fatalf("unknown %+v", unknown)
	}
	t.Run("I2C-SEED-02 different company", func(t *testing.T) {
		if !sameStatus(wrong, unknown) {
			t.Fatalf("wrong %+v unknown %+v", wrong, unknown)
		}
	})
	t.Run("I2C-SEED-03 nil carrier company", func(t *testing.T) {
		if !sameStatus(missingCompany, unknown) {
			t.Fatalf("nil %+v unknown %+v", missingCompany, unknown)
		}
	})
	t.Run("I2C-SEED-04 foreign tenant", func(t *testing.T) {
		if !sameStatus(foreign, unknown) {
			t.Fatalf("foreign %+v unknown %+v", foreign, unknown)
		}
	})

	t.Run("I2C-SEED-05 current trip company fail closed", func(t *testing.T) {
		shipment := uuid.New()
		lat, lon := 1.0, 2.0
		base := currenttrip.CurrentTripContext{
			ShipmentID: shipment, ShipmentVersion: 7,
			CurrentPosition: &currenttrip.TrackingPosition{Latitude: &lat, Longitude: &lon},
		}
		allowed := base
		allowed.CarrierCompanyID = &company
		got, err := (StoreSeedResolver{Trips: fixedTrip{ctx: allowed}}).ResolveTrip(ctx, actor, shipment)
		if err != nil || got.ShipmentVersion == nil || *got.ShipmentVersion != 7 {
			t.Fatalf("allowed %+v err=%v", got, err)
		}
		different := base
		different.CarrierCompanyID = &otherCompany
		nilOwner := base
		notFound := publicTripError(t, fixedTrip{err: currenttrip.ErrNotFound}, actor, shipment)
		diffErr := publicTripError(t, fixedTrip{ctx: different}, actor, shipment)
		nilErr := publicTripError(t, fixedTrip{ctx: nilOwner}, actor, shipment)
		if !sameStatus(diffErr, notFound) || !sameStatus(nilErr, notFound) || notFound.Status != 422 {
			t.Fatalf("diff=%+v nil=%+v missing=%+v", diffErr, nilErr, notFound)
		}
	})
}

func TestI2C_POOL_cross_shipper_policy(t *testing.T) {
	carrier := uuid.New()
	company := uuid.New()
	other := uuid.New()
	shipper := uuid.New()
	actor := service.Actor{TenantID: carrier, UserID: uuid.New(), CompanyID: &company}
	now := time.Date(2026, 10, 16, 12, 0, 0, 0, time.UTC)

	t.Run("I2C-POOL-01 cross-shipper-only load reaches engine", func(t *testing.T) {
		store := repository.NewMemory()
		left := publishedLoad(shipper, now, false, true, domain.VisMarketplace, nil)
		right := publishedLoad(shipper, now, false, true, domain.VisMarketplace, nil)
		putLoad(t, store, left)
		putLoad(t, store, right)
		loads := mustLoads(t, MarketplacePool{Store: store}, actor)
		if len(loads) != 2 || loads[0].ConsolidationAllowed || !loads[0].CrossShipperConsolidationAllowed {
			t.Fatalf("pool %+v", loads)
		}
		result := searchLoads(t, carrier, now, loads)
		if result.ReasonCounts[chain.ReasonConsolidation] != 0 || len(result.FirstLoadIDs) == 0 {
			t.Fatalf("not eligible counts=%v ids=%v", result.ReasonCounts, result.FirstLoadIDs)
		}
	})

	t.Run("I2C-POOL-02 cross-shipper false rejected", func(t *testing.T) {
		store := repository.NewMemory()
		load := publishedLoad(shipper, now, false, false, domain.VisMarketplace, nil)
		putLoad(t, store, load)
		loads := mustLoads(t, MarketplacePool{Store: store}, actor)
		if len(loads) != 1 {
			t.Fatalf("pool dropped the load before the engine: %+v", loads)
		}
		result := searchLoads(t, carrier, now, loads)
		if result.ReasonCounts[chain.ReasonConsolidation] != 1 || len(result.FirstLoadIDs) != 0 {
			t.Fatalf("counts=%v ids=%v", result.ReasonCounts, result.FirstLoadIDs)
		}
	})

	t.Run("I2C-POOL-03 invited carrier respects selected company", func(t *testing.T) {
		store := repository.NewMemory()
		load := publishedLoad(shipper, now, false, true, domain.VisInvited, []uuid.UUID{company})
		putLoad(t, store, load)
		selected := mustLoads(t, MarketplacePool{Store: store}, actor)
		foreign := mustLoads(t, MarketplacePool{Store: store}, service.Actor{TenantID: carrier, CompanyID: &other})
		anonymous := mustLoads(t, MarketplacePool{Store: store}, service.Actor{TenantID: carrier})
		if len(selected) != 1 || len(foreign) != 0 || len(anonymous) != 0 {
			t.Fatalf("selected=%d foreign=%d anonymous=%d", len(selected), len(foreign), len(anonymous))
		}
	})

	t.Run("I2C-POOL-04 anonymized load stays eligible without private commercial data", func(t *testing.T) {
		store := repository.NewMemory()
		load := publishedLoad(shipper, now, false, true, domain.VisAnonymized, nil)
		load.Commercial = domain.Commercial{Mode: "FIXED", Currency: "SECRET_RATE_9911"}
		amount := 9911.0
		load.Commercial.Amount = &amount
		load.Pickup.Label = "SECRET_DOCK"
		putLoad(t, store, load)
		loads := mustLoads(t, MarketplacePool{Store: store}, actor)
		if len(loads) != 1 {
			t.Fatal("anonymized marketplace load was dropped")
		}
		encoded, err := json.Marshal(loads[0])
		if err != nil {
			t.Fatal(err)
		}
		for _, leaked := range []string{"SECRET_RATE_9911", "SECRET_DOCK", "9911"} {
			if contains(string(encoded), leaked) {
				t.Fatalf("pool upgraded privacy: %s", encoded)
			}
		}
	})
}

func TestI2C_FP_02_unknown_routing_version_is_deterministic(t *testing.T) {
	actor := service.Actor{TenantID: uuid.New(), CompanyID: ptr(uuid.New())}
	resolved := ResolvedSeed{InternalSource: repository.ChainSeedManualAvailableCapacity}
	withVersion := &API{Catalog: reference.NewMemoryCatalog(), RoutingProviderVersion: "UNKNOWN"}
	withoutVersion := &API{Catalog: withVersion.Catalog, RoutingProviderVersion: ""}
	left, err := withVersion.Fingerprint(context.Background(), actor, resolved)
	if err != nil {
		t.Fatal(err)
	}
	right, err := withoutVersion.Fingerprint(context.Background(), actor, resolved)
	if err != nil {
		t.Fatal(err)
	}
	if left.RoutingProviderVersion != "UNKNOWN" || right.RoutingProviderVersion != "UNKNOWN" {
		t.Fatalf("left=%s right=%s", left.RoutingProviderVersion, right.RoutingProviderVersion)
	}
	if left.FilterFingerprint != filterFingerprint || !contains(filterFingerprint, "FOREIGN=CROSS_SHIPPER_CONSOLIDATION_ALLOWED") {
		t.Fatal(left.FilterFingerprint)
	}
	if chainorch.ComputeRequestFingerprint(left) != chainorch.ComputeRequestFingerprint(right) {
		t.Fatal("UNKNOWN routing provenance is not deterministic")
	}
}

func sameStatus(left, right *StatusError) bool {
	return left.Status == right.Status && left.Code == right.Code && left.Message == right.Message
}

func publicSeedError(t *testing.T, resolver StoreSeedResolver, actor service.Actor, id uuid.UUID) *StatusError {
	t.Helper()
	_, err := (&API{Seeds: resolver}).resolve(context.Background(), actor, publicSeed{SeedType: PublicSeedManualCapacity, CapacityID: &id})
	status, ok := err.(*StatusError)
	if !ok {
		t.Fatalf("err=%v", err)
	}
	return status
}

func publicTripError(t *testing.T, trips TripBuilder, actor service.Actor, id uuid.UUID) *StatusError {
	t.Helper()
	_, err := (&API{Seeds: StoreSeedResolver{Trips: trips}}).resolve(context.Background(), actor, publicSeed{SeedType: PublicSeedCurrentTrip, ShipmentID: &id})
	status, ok := err.(*StatusError)
	if !ok {
		t.Fatalf("err=%v", err)
	}
	return status
}

type fixedTrip struct {
	ctx currenttrip.CurrentTripContext
	err error
}

func (f fixedTrip) Build(context.Context, uuid.UUID, uuid.UUID) (currenttrip.CurrentTripContext, error) {
	return f.ctx, f.err
}

func sampleCapacity(tenant uuid.UUID, company *uuid.UUID) domain.Capacity {
	lat, lon := 55.0, 37.0
	now := time.Date(2026, 10, 16, 8, 0, 0, 0, time.UTC)
	return domain.Capacity{
		ID: uuid.New(), OwnerTenantID: tenant, CarrierCompanyID: company,
		Latitude: &lat, Longitude: &lon, AvailableFrom: now, AvailableUntil: now.Add(24 * time.Hour),
		Source: domain.SourceManual, Status: domain.CapacityAvailable, Version: 4,
		VisibilityScope: domain.CapVisPrivate,
	}
}

func publishedLoad(owner uuid.UUID, now time.Time, consolidation, cross bool, scope string, invited []uuid.UUID) domain.LoadOpportunity {
	lat, lon := 1.0, 2.0
	dlat, dlon := 3.0, 4.0
	weight := 10.0
	start := now.Add(time.Hour)
	end := now.Add(2 * time.Hour)
	deliveryStart := now.Add(3 * time.Hour)
	deliveryEnd := now.Add(4 * time.Hour)
	pickupID, deliveryID := uuid.New(), uuid.New()
	return domain.LoadOpportunity{
		ID: uuid.New(), OwnerTenantID: owner, SourceType: domain.SourceManual, SourceID: uuid.New(),
		Pickup:         domain.Place{LocationID: &pickupID, Latitude: &lat, Longitude: &lon},
		Delivery:       domain.Place{LocationID: &deliveryID, Latitude: &dlat, Longitude: &dlon},
		PickupWindow:   domain.TimeWindow{Start: &start, End: &end},
		DeliveryWindow: domain.TimeWindow{Start: &deliveryStart, End: &deliveryEnd},
		WeightKg:       &weight, VisibilityScope: scope, InvitedCarrierCompanyIDs: invited,
		ConsolidationAllowed: consolidation, CrossShipperConsolidationAllowed: cross,
		Status: domain.LoadPublished, Version: 1, CreatedAt: now, UpdatedAt: now,
	}
}

func putCapacity(t *testing.T, store *repository.Memory, cap domain.Capacity) {
	t.Helper()
	if err := store.Within(context.Background(), func(tx repository.Tx) error {
		return tx.InsertCapacity(context.Background(), cap)
	}); err != nil {
		t.Fatal(err)
	}
}

func putLoad(t *testing.T, store *repository.Memory, load domain.LoadOpportunity) {
	t.Helper()
	if err := store.Within(context.Background(), func(tx repository.Tx) error {
		return tx.InsertLoad(context.Background(), load)
	}); err != nil {
		t.Fatal(err)
	}
}

func mustLoads(t *testing.T, pool MarketplacePool, actor service.Actor) []chain.VisibleLoad {
	t.Helper()
	loads, err := pool.Loads(context.Background(), actor)
	if err != nil {
		t.Fatal(err)
	}
	return loads
}

func searchLoads(t *testing.T, carrier uuid.UUID, now time.Time, loads []chain.VisibleLoad) chain.Result {
	t.Helper()
	from := now
	until := now.Add(48 * time.Hour)
	result, err := (chain.Engine{}).Search(context.Background(), chain.Request{
		TenantID: carrier,
		Seed: chain.Seed{
			Source: chain.SeedManualAvailable, AvailableFrom: &from, AvailableUntil: &until,
			Start:    routeplan.Point{Kind: routeplan.PointAnchor, Latitude: 0, Longitude: 0, Source: routeplan.SourceCanonical},
			Capacity: chain.Capacity{Payload: chain.Dimension{Known: true, Value: 1000}, Volume: chain.Dimension{Known: true, Value: 1000}, Pallets: chain.Dimension{Known: true, Value: 1000}, Linear: chain.Dimension{Known: true, Value: 1000}, Height: chain.Dimension{Known: true, Value: 1000}},
		},
		Loads: loads, Route: fixedRoute{}, Now: func() time.Time { return now },
	})
	if err != nil {
		t.Fatal(err)
	}
	return result
}

type fixedRoute struct{}

func (fixedRoute) Route(context.Context, routing.RouteRequest) (routing.RouteResult, error) {
	return routing.RouteResult{Provider: "test", DistanceM: 1000, DurationSeconds: 60, CalculatedAt: time.Unix(1, 0).UTC(), ExpiresAt: time.Unix(1, 0).UTC().Add(time.Hour)}, nil
}

func (fixedRoute) Matrix(context.Context, routing.MatrixRequest) (routing.MatrixResult, error) {
	return routing.MatrixResult{}, routing.ErrInvalidResponse
}

func ptr(id uuid.UUID) *uuid.UUID { return &id }

func contains(body, needle string) bool {
	return len(needle) > 0 && len(body) >= len(needle) && (body == needle || len(body) > 0 && (indexOf(body, needle) >= 0))
}

func indexOf(body, needle string) int {
	for i := 0; i+len(needle) <= len(body); i++ {
		if body[i:i+len(needle)] == needle {
			return i
		}
	}
	return -1
}
