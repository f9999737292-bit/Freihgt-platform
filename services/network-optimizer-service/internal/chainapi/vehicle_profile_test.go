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

func TestI271_001_vehicle_profile(t *testing.T) {
	ctx := context.Background()
	tenant := uuid.New()
	company := uuid.New()
	actor := service.Actor{TenantID: tenant, UserID: uuid.New(), CompanyID: &company}
	weight := 20000.0
	internal := 4000
	adr := true

	t.Run("I271_001_TRIP_CANONICAL_VEHICLE", func(t *testing.T) {
		yes := true
		capability := &currenttrip.VehicleCapability{
			ID: uuid.New(), TenantID: tenant, CapacityWeight: &weight, InternalHeightMM: &internal, ADRCapability: &adr,
		}
		built := tripContext(company, capability, []currenttrip.OnboardCargoUnit{{
			EvidenceState: currenttrip.EvidenceConfirmedOnboard,
			Profile:       currenttrip.CargoProfile{DangerousGoods: &yes},
		}})
		resolved, err := (StoreSeedResolver{Trips: fixedTrip{ctx: built}}).ResolveTrip(ctx, actor, built.ShipmentID)
		if err != nil {
			t.Fatal(err)
		}
		if resolved.Vehicle.DangerousCargo == nil || !*resolved.Vehicle.DangerousCargo {
			t.Fatalf("dangerous cargo %+v", resolved.Vehicle.DangerousCargo)
		}
		assertUnmappedRoutingDimensions(t, resolved.Vehicle)
	})

	t.Run("I271_001_TRIP_NO_CLIENT_PROFILE", func(t *testing.T) {
		_, err := parsePublicSeed([]byte(`{"seed_type":"CURRENT_TRIP","shipment_id":"11111111-1111-1111-1111-111111111111","vehicle_profile":{"gross_weight_kg":1}}`))
		status, ok := err.(*StatusError)
		if !ok || status.Status != 400 {
			t.Fatalf("status=%v err=%v", status, err)
		}
	})

	t.Run("I271_001_TRIP_UNKNOWN_FIELD_STAYS_NIL", func(t *testing.T) {
		capability := &currenttrip.VehicleCapability{ID: uuid.New(), TenantID: tenant, CapacityWeight: &weight, InternalLengthMM: &internal, InternalWidthMM: &internal, InternalHeightMM: &internal, ADRCapability: &adr}
		built := tripContext(company, capability, []currenttrip.OnboardCargoUnit{{
			EvidenceState: currenttrip.EvidenceConfirmedOnboard,
			Profile:       currenttrip.CargoProfile{},
		}})
		resolved, err := (StoreSeedResolver{Trips: fixedTrip{ctx: built}}).ResolveTrip(ctx, actor, built.ShipmentID)
		if err != nil {
			t.Fatal(err)
		}
		if resolved.Vehicle.DangerousCargo != nil {
			t.Fatal("unknown onboard dangerous state was fabricated")
		}
		assertUnmappedRoutingDimensions(t, resolved.Vehicle)
	})

	t.Run("I271_001_MANUAL_VEHICLE_LOOKUP", func(t *testing.T) {
		store := repository.NewMemory()
		vehicleID := uuid.New()
		cap := sampleCapacity(tenant, &company)
		cap.VehicleID = &vehicleID
		putCapacity(t, store, cap)
		source := &fakeVehicles{capability: currenttrip.VehicleCapability{ID: vehicleID, TenantID: tenant, CapacityWeight: &weight, InternalHeightMM: &internal, ADRCapability: &adr}}
		resolved, err := (StoreSeedResolver{Store: store, Vehicles: source}).ResolveManual(ctx, actor, cap.ID)
		if err != nil || !source.called || source.tenant != tenant {
			t.Fatalf("resolved err=%v called=%v tenant=%s", err, source.called, source.tenant)
		}
		assertUnmappedRoutingDimensions(t, resolved.Vehicle)
		if resolved.Vehicle.DangerousCargo != nil {
			t.Fatal("manual capacity invented onboard dangerous cargo")
		}
	})

	t.Run("I271_001_MANUAL_FOREIGN_VEHICLE_FAILS_CLOSED", func(t *testing.T) {
		store := repository.NewMemory()
		vehicleID := uuid.New()
		cap := sampleCapacity(tenant, &company)
		cap.VehicleID = &vehicleID
		putCapacity(t, store, cap)
		missing := publicManualVehicle(t, store, actor, cap.ID, &fakeVehicles{err: currenttrip.ErrNotFound})
		foreign := publicManualVehicle(t, store, actor, cap.ID, &fakeVehicles{capability: currenttrip.VehicleCapability{ID: vehicleID, TenantID: uuid.New()}})
		if !sameStatus(missing, foreign) || missing.Status != 422 || missing.Message != "seed is not eligible" {
			t.Fatalf("missing=%+v foreign=%+v", missing, foreign)
		}
	})

	t.Run("I271_001_MANUAL_NO_VEHICLE", func(t *testing.T) {
		store := repository.NewMemory()
		cap := sampleCapacity(tenant, &company)
		putCapacity(t, store, cap)
		source := &fakeVehicles{}
		resolved, err := (StoreSeedResolver{Store: store, Vehicles: source}).ResolveManual(ctx, actor, cap.ID)
		if err != nil || source.called {
			t.Fatalf("err=%v called=%v", err, source.called)
		}
		assertUnmappedRoutingDimensions(t, resolved.Vehicle)
	})

	t.Run("I271_001_DANGEROUS_CARGO_NOT_FROM_ADR_CAPABILITY", func(t *testing.T) {
		profile := routingProfileFromCanonical(&currenttrip.VehicleCapability{ADRCapability: &adr, CapacityWeight: &weight}, nil, true)
		if profile.DangerousCargo == nil || *profile.DangerousCargo {
			t.Fatalf("ADR capability became dangerous cargo: %+v", profile.DangerousCargo)
		}
	})

	t.Run("I271_001_CAPACITY_WEIGHT_NOT_GROSS_WEIGHT_WITHOUT_PROOF", func(t *testing.T) {
		profile := routingProfileFromCanonical(&currenttrip.VehicleCapability{CapacityWeight: &weight, InternalLengthMM: &internal, InternalWidthMM: &internal, InternalHeightMM: &internal}, nil, false)
		assertUnmappedRoutingDimensions(t, profile)
	})

	t.Run("I271_001_ROUTING_REQUEST_RECEIVES_PROFILE", func(t *testing.T) {
		yes := true
		capability := &currenttrip.VehicleCapability{ID: uuid.New(), TenantID: tenant, CapacityWeight: &weight, ADRCapability: &adr}
		built := tripContext(company, capability, []currenttrip.OnboardCargoUnit{{
			EvidenceState: currenttrip.EvidenceConfirmedOnboard,
			Profile:       currenttrip.CargoProfile{DangerousGoods: &yes},
		}})
		resolved, err := (StoreSeedResolver{Trips: fixedTrip{ctx: built}}).ResolveTrip(ctx, actor, built.ShipmentID)
		if err != nil {
			t.Fatal(err)
		}
		now := time.Date(2026, 10, 16, 12, 0, 0, 0, time.UTC)
		store := repository.NewMemory()
		left := publishedLoad(uuid.New(), now, false, true, domain.VisMarketplace, nil)
		right := publishedLoad(uuid.New(), now, false, true, domain.VisMarketplace, nil)
		putLoad(t, store, left)
		putLoad(t, store, right)
		loads := mustLoads(t, MarketplacePool{Store: store}, actor)
		recorder := &captureRoute{}
		_, err = (chain.Engine{}).Search(ctx, chain.Request{
			TenantID: tenant,
			Seed: chain.Seed{
				Source:   chain.SeedCurrentTripResidual,
				Start:    routeplan.Point{Kind: routeplan.PointAnchor, Latitude: 1, Longitude: 2, Source: routeplan.SourceTracking},
				Capacity: chain.Capacity{Payload: chain.Dimension{Known: true, Value: 1000}, Volume: chain.Dimension{Known: true, Value: 1000}},
			},
			Loads: loads, VehicleProfile: resolved.Vehicle, Route: recorder, Now: func() time.Time { return now },
		})
		if err != nil {
			t.Fatal(err)
		}
		if recorder.calls == 0 || recorder.req.VehicleProfile.DangerousCargo == nil || !*recorder.req.VehicleProfile.DangerousCargo {
			t.Fatalf("calls=%d profile=%+v", recorder.calls, recorder.req.VehicleProfile)
		}
		if recorder.req.VehicleProfile.GrossWeightKg != nil {
			t.Fatal("routing request received fabricated gross weight")
		}
	})
}

func assertUnmappedRoutingDimensions(t *testing.T, profile routing.VehicleProfile) {
	t.Helper()
	if profile.GrossWeightKg != nil || profile.HeightM != nil || profile.WidthM != nil || profile.LengthM != nil || profile.AxleLoadKg != nil {
		t.Fatalf("unproven routing field populated: %+v", profile)
	}
}

func publicManualVehicle(t *testing.T, store *repository.Memory, actor service.Actor, capacityID uuid.UUID, vehicles *fakeVehicles) *StatusError {
	t.Helper()
	_, err := (&API{Seeds: StoreSeedResolver{Store: store, Vehicles: vehicles}}).resolve(context.Background(), actor, publicSeed{SeedType: PublicSeedManualCapacity, CapacityID: &capacityID})
	status, ok := err.(*StatusError)
	if !ok {
		t.Fatalf("err=%v", err)
	}
	return status
}

func tripContext(company uuid.UUID, capability *currenttrip.VehicleCapability, units []currenttrip.OnboardCargoUnit) currenttrip.CurrentTripContext {
	lat, lon := 1.0, 2.0
	return currenttrip.CurrentTripContext{
		ShipmentID: uuid.New(), ShipmentVersion: 3, CarrierCompanyID: &company,
		VehicleCapability: capability, OnboardCargoUnits: units,
		CurrentPosition: &currenttrip.TrackingPosition{Latitude: &lat, Longitude: &lon},
	}
}

type fakeVehicles struct {
	capability currenttrip.VehicleCapability
	err        error
	called     bool
	tenant     uuid.UUID
}

func (f *fakeVehicles) VehicleCapability(_ context.Context, tenant, _ uuid.UUID) (currenttrip.VehicleCapability, error) {
	f.called = true
	f.tenant = tenant
	return f.capability, f.err
}

type captureRoute struct {
	req   routing.RouteRequest
	calls int
}

func (c *captureRoute) Route(_ context.Context, req routing.RouteRequest) (routing.RouteResult, error) {
	c.calls++
	c.req = req
	return routing.RouteResult{Provider: "test", DistanceM: 1000, DurationSeconds: 60, CalculatedAt: time.Unix(1, 0).UTC(), ExpiresAt: time.Unix(1, 0).UTC().Add(time.Hour)}, nil
}

func (c *captureRoute) Matrix(context.Context, routing.MatrixRequest) (routing.MatrixResult, error) {
	return routing.MatrixResult{}, routing.ErrInvalidResponse
}
