package chainapi

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/freight-platform/network-optimizer-service/internal/chain"
	"github.com/freight-platform/network-optimizer-service/internal/compat"
	"github.com/freight-platform/network-optimizer-service/internal/currenttrip"
	"github.com/freight-platform/network-optimizer-service/internal/domain"
	"github.com/freight-platform/network-optimizer-service/internal/reference"
	"github.com/freight-platform/network-optimizer-service/internal/repository"
	"github.com/freight-platform/network-optimizer-service/internal/routeplan"
	"github.com/freight-platform/network-optimizer-service/internal/routing"
	"github.com/freight-platform/network-optimizer-service/internal/service"
)

const (
	filterFingerprint = "STATUS=PUBLISHED;SAME_OWNER=CONSOLIDATION_ALLOWED;FOREIGN=CROSS_SHIPPER_CONSOLIDATION_ALLOWED;DISCOVERY_CAP=1000"
	poolLabel         = "PUBLISHED_MARKETPLACE"
)

// ResolvedSeed is server-owned chain input. Callers cannot supply versions or horizons.
type ResolvedSeed struct {
	InternalSource  string
	CapacityID      *uuid.UUID
	CapacityVersion *int
	ShipmentID      *uuid.UUID
	ShipmentVersion *int
	AvailableFrom   *time.Time
	AvailableUntil  *time.Time
	HorizonPolicy   string
	ChainSeed       chain.Seed
	Vehicle         routing.VehicleProfile
}

// SeedResolver loads canonical tenant-owned seeds. Missing and foreign seeds
// return ErrSeedIneligible with the same result so existence does not leak.
type SeedResolver interface {
	ResolveManual(ctx context.Context, actor service.Actor, capacityID uuid.UUID) (ResolvedSeed, error)
	ResolveTrip(ctx context.Context, actor service.Actor, shipmentID uuid.UUID) (ResolvedSeed, error)
}

// LoadPool is the server-applied publication-visible load set.
type LoadPool interface {
	Loads(ctx context.Context, actor service.Actor) ([]chain.VisibleLoad, error)
}

var (
	ErrSeedIneligible  = errors.New("seed ineligible")
	ErrSeedUnavailable = errors.New("seed unavailable")
	ErrPredictedSeed   = errors.New("predicted seed")
)

// TripBuilder is the existing current-trip context builder.
type TripBuilder interface {
	Build(ctx context.Context, tenantID, shipmentID uuid.UUID) (currenttrip.CurrentTripContext, error)
}

// ServiceTrip adapts service.Service.BuildCurrentTrip to TripBuilder.
type ServiceTrip struct {
	Service *service.Service
}

func (s ServiceTrip) Build(ctx context.Context, tenantID, shipmentID uuid.UUID) (currenttrip.CurrentTripContext, error) {
	if s.Service == nil {
		return currenttrip.CurrentTripContext{}, currenttrip.ErrUnavailable
	}
	return s.Service.BuildCurrentTrip(ctx, tenantID, shipmentID)
}

// StoreSeedResolver reads capacity and current-trip context from canonical sources.
type StoreSeedResolver struct {
	Store repository.Store
	Trips TripBuilder
}

func (r StoreSeedResolver) ResolveManual(ctx context.Context, actor service.Actor, capacityID uuid.UUID) (ResolvedSeed, error) {
	if r.Store == nil {
		return ResolvedSeed{}, ErrSeedUnavailable
	}
	var capacity domain.Capacity
	err := r.Store.Within(ctx, func(tx repository.Tx) error {
		row, err := tx.GetCapacity(ctx, capacityID)
		if err != nil {
			return err
		}
		capacity = row
		return nil
	})
	if errors.Is(err, repository.ErrNotFound) {
		return ResolvedSeed{}, ErrSeedIneligible
	}
	if err != nil {
		return ResolvedSeed{}, ErrSeedUnavailable
	}
	if capacity.OwnerTenantID != actor.TenantID || !companyMatches(actor.CompanyID, capacity.CarrierCompanyID) {
		return ResolvedSeed{}, ErrSeedIneligible
	}
	if capacity.Status == domain.CapacityPredicted || capacity.Source == domain.SourceCurrentShipmentPrediction {
		return ResolvedSeed{}, ErrPredictedSeed
	}
	if capacity.Source != domain.SourceManual || capacity.Status != domain.CapacityAvailable {
		return ResolvedSeed{}, ErrSeedIneligible
	}
	if capacity.Latitude == nil || capacity.Longitude == nil {
		return ResolvedSeed{}, ErrSeedIneligible
	}
	version := capacity.Version
	from := capacity.AvailableFrom.UTC()
	until := capacity.AvailableUntil.UTC()
	id := capacity.ID
	return ResolvedSeed{
		InternalSource:  repository.ChainSeedManualAvailableCapacity,
		CapacityID:      &id,
		CapacityVersion: &version,
		AvailableFrom:   &from,
		AvailableUntil:  &until,
		ChainSeed: chain.Seed{
			Source:         chain.SeedManualAvailable,
			AvailableFrom:  &from,
			AvailableUntil: &until,
			Start: routeplan.Point{
				Kind: routeplan.PointAnchor, Latitude: *capacity.Latitude, Longitude: *capacity.Longitude,
				Source: routeplan.SourceCanonical,
			},
			Capacity: chain.Capacity{
				Payload: dimPtr(capacity.PayloadRemainingKg),
				Volume:  dimPtr(capacity.VolumeRemainingM3),
			},
		},
		Vehicle: routing.VehicleProfile{},
	}, nil
}

func (r StoreSeedResolver) ResolveTrip(ctx context.Context, actor service.Actor, shipmentID uuid.UUID) (ResolvedSeed, error) {
	if r.Trips == nil {
		return ResolvedSeed{}, ErrSeedUnavailable
	}
	built, err := r.Trips.Build(ctx, actor.TenantID, shipmentID)
	if errors.Is(err, currenttrip.ErrNotFound) {
		return ResolvedSeed{}, ErrSeedIneligible
	}
	if err != nil {
		return ResolvedSeed{}, ErrSeedUnavailable
	}
	if built.ShipmentID == uuid.Nil || !companyMatches(actor.CompanyID, built.CarrierCompanyID) {
		return ResolvedSeed{}, ErrSeedIneligible
	}
	if built.CurrentPosition == nil || built.CurrentPosition.Latitude == nil || built.CurrentPosition.Longitude == nil {
		return ResolvedSeed{}, ErrSeedIneligible
	}
	version := built.ShipmentVersion
	id := built.ShipmentID
	confirmed := 0
	for _, unit := range built.OnboardCargoUnits {
		if unit.EvidenceState == currenttrip.EvidenceConfirmedOnboard {
			confirmed++
		}
	}
	return ResolvedSeed{
		InternalSource:  repository.ChainSeedCurrentTripResidual,
		ShipmentID:      &id,
		ShipmentVersion: &version,
		HorizonPolicy:   "SEARCH_CLOCK+48H",
		ChainSeed: chain.Seed{
			Source: chain.SeedCurrentTripResidual,
			Start: routeplan.Point{
				Kind: routeplan.PointAnchor, Latitude: *built.CurrentPosition.Latitude, Longitude: *built.CurrentPosition.Longitude,
				Source: routeplan.SourceTracking, ObservedAt: built.PositionObservedAt,
			},
			Capacity: chain.Capacity{
				Payload: residualDim(built.ResidualCapacity.Payload),
				Volume:  residualDim(built.ResidualCapacity.Volume),
				Pallets: residualDim(built.ResidualCapacity.PalletPositions),
				Linear:  residualDim(built.ResidualCapacity.LinearMeters),
			},
			OnboardSubjects: confirmed,
		},
	}, nil
}

func companyMatches(actorCompany, ownerCompany *uuid.UUID) bool {
	if actorCompany == nil || ownerCompany == nil {
		return false
	}
	return *actorCompany == *ownerCompany
}

// MarketplacePool lists the existing publication-visible consolidation pool.
type MarketplacePool struct {
	Store repository.Store
}

func (p MarketplacePool) Loads(ctx context.Context, actor service.Actor) ([]chain.VisibleLoad, error) {
	if p.Store == nil {
		return nil, ErrSeedUnavailable
	}
	var rows []domain.LoadOpportunity
	err := p.Store.Within(ctx, func(tx repository.Tx) error {
		var err error
		rows, err = tx.ListMarketplaceLoads(ctx, actor.TenantID, actor.CompanyID, domain.CandidateDiscoveryCap, 0)
		return err
	})
	if err != nil {
		return nil, err
	}
	out := make([]chain.VisibleLoad, 0, len(rows))
	for _, row := range rows {
		if row.Pickup.Latitude == nil || row.Pickup.Longitude == nil || row.Delivery.Latitude == nil || row.Delivery.Longitude == nil {
			continue
		}
		if row.PickupWindow.Start == nil || row.PickupWindow.End == nil || row.DeliveryWindow.Start == nil || row.DeliveryWindow.End == nil {
			continue
		}
		weight := 0.0
		if row.WeightKg != nil {
			weight = *row.WeightKg
		}
		out = append(out, chain.VisibleLoad{
			ID: row.ID, Version: row.Version, OwnerTenantID: row.OwnerTenantID,
			ConsolidationAllowed: row.ConsolidationAllowed, CrossShipperConsolidationAllowed: row.CrossShipperConsolidationAllowed,
			Pickup: routeplan.Point{
				Kind: routeplan.PointCanonical, LocationID: row.Pickup.LocationID,
				Latitude: *row.Pickup.Latitude, Longitude: *row.Pickup.Longitude, Source: routeplan.SourceCanonical,
			},
			Delivery: routeplan.Point{
				Kind: routeplan.PointCanonical, LocationID: row.Delivery.LocationID,
				Latitude: *row.Delivery.Latitude, Longitude: *row.Delivery.Longitude, Source: routeplan.SourceCanonical,
			},
			PickupStart: *row.PickupWindow.Start, PickupEnd: *row.PickupWindow.End,
			DeliveryStart: *row.DeliveryWindow.Start, DeliveryEnd: *row.DeliveryWindow.End,
			Stackable: row.Cargo.Stackable, WeightKg: weight, PublicationLabel: poolLabel,
		})
	}
	return out, nil
}

func catalogFingerprint(ctx context.Context, catalog reference.Catalog, tenant uuid.UUID) (string, error) {
	if catalog == nil {
		return "UNKNOWN", nil
	}
	eval, err := catalog.Evaluation(ctx, tenant)
	if err != nil {
		return "", err
	}
	raw, err := json.Marshal(struct {
		Cargo     int                 `json:"cargo"`
		Equipment int                 `json:"equipment"`
		Pallet    int                 `json:"pallet"`
		Packaging int                 `json:"packaging"`
		Rules     []compat.RuleSetRef `json:"rules"`
	}{
		Cargo: eval.CargoCatalogVersion, Equipment: eval.EquipmentCatalogVersion,
		Pallet: eval.PalletCatalogVersion, Packaging: eval.PackagingCatalogVersion,
		Rules: eval.RuleSets,
	})
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(raw)
	return hex.EncodeToString(sum[:]), nil
}

func publicationFingerprint(actor service.Actor) string {
	company := "none"
	if actor.CompanyID != nil {
		company = actor.CompanyID.String()
	}
	sum := sha256.Sum256([]byte(fmt.Sprintf("viewer=%s;company=%s;pool=%s", actor.TenantID, company, poolLabel)))
	return hex.EncodeToString(sum[:])
}

func dimPtr(v *float64) chain.Dimension {
	if v == nil {
		return chain.Dimension{}
	}
	return chain.Dimension{Known: true, Value: *v}
}

func residualDim(d currenttrip.SubtractiveDimension) chain.Dimension {
	if d.Status != currenttrip.DimensionKnown || d.Remaining == nil {
		return chain.Dimension{}
	}
	return chain.Dimension{Known: true, Value: *d.Remaining}
}
