//go:build integration

package chainapi_test

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/freight-platform/network-optimizer-service/internal/chainapi"
	"github.com/freight-platform/network-optimizer-service/internal/domain"
	"github.com/freight-platform/network-optimizer-service/internal/repository"
	"github.com/freight-platform/network-optimizer-service/internal/service"
)

func TestI2C_ChainAPI_ProductionPool(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 4*time.Minute)
	defer cancel()
	pool := startPostgres(t, ctx)
	applyThrough000097(t, ctx, pool)
	store := repository.NewPostgres(pool)
	now := time.Date(2026, 10, 16, 11, 0, 0, 0, time.UTC)
	adapter := chainapi.MarketplacePool{Store: store}

	t.Run("CHAIN_API_OWN_OWN", func(t *testing.T) {
		viewer, company := uuid.New(), uuid.New()
		a := insertChainLoad(t, ctx, store, chainDiscoveryLoad(viewer, now, true, false, domain.VisPrivate, nil))
		b := insertChainLoad(t, ctx, store, chainDiscoveryLoad(viewer, now.Add(time.Second), true, false, domain.VisPrivate, nil))
		loads := productionLoads(t, adapter, service.Actor{TenantID: viewer, CompanyID: &company})
		if !containsVisible(loads, a) || !containsVisible(loads, b) {
			t.Fatalf("own+own %+v", loads)
		}
	})

	t.Run("CHAIN_API_OWN_FOREIGN", func(t *testing.T) {
		viewer, company, shipper := uuid.New(), uuid.New(), uuid.New()
		own := insertChainLoad(t, ctx, store, chainDiscoveryLoad(viewer, now, true, false, domain.VisPrivate, nil))
		foreign := insertChainLoad(t, ctx, store, chainDiscoveryLoad(shipper, now, false, true, domain.VisMarketplace, nil))
		denied := insertChainLoad(t, ctx, store, chainDiscoveryLoad(viewer, now, false, true, domain.VisMarketplace, nil))
		hidden := insertChainLoad(t, ctx, store, chainDiscoveryLoad(shipper, now, false, true, domain.VisPrivate, nil))
		loads := productionLoads(t, adapter, service.Actor{TenantID: viewer, CompanyID: &company})
		if !containsVisible(loads, own) || !containsVisible(loads, foreign) || containsVisible(loads, denied) || containsVisible(loads, hidden) {
			t.Fatalf("own+foreign %+v", loads)
		}
	})
}

func chainDiscoveryLoad(owner uuid.UUID, created time.Time, consolidation, cross bool, scope string, invited []uuid.UUID) domain.LoadOpportunity {
	lat, lon := 1.0, 2.0
	weight := 10.0
	start := created
	end := created.Add(time.Hour)
	return domain.LoadOpportunity{
		ID: uuid.New(), OwnerTenantID: owner, SourceType: domain.SourceTransportOrder, SourceID: uuid.New(),
		Pickup: domain.Place{Latitude: &lat, Longitude: &lon}, Delivery: domain.Place{Latitude: &lat, Longitude: &lon},
		PickupWindow: domain.TimeWindow{Start: &start, End: &end}, DeliveryWindow: domain.TimeWindow{Start: &start, End: &end},
		WeightKg: &weight, VisibilityScope: scope, InvitedCarrierCompanyIDs: invited,
		ConsolidationAllowed: consolidation, CrossShipperConsolidationAllowed: cross,
		Status: domain.LoadPublished, Version: 1, CreatedAt: created, UpdatedAt: created,
	}
}

func insertChainLoad(t *testing.T, ctx context.Context, store *repository.Postgres, load domain.LoadOpportunity) uuid.UUID {
	t.Helper()
	if err := store.Within(ctx, func(tx repository.Tx) error { return tx.InsertLoad(ctx, load) }); err != nil {
		t.Fatal(err)
	}
	return load.ID
}

func productionLoads(t *testing.T, pool chainapi.MarketplacePool, actor service.Actor) []uuid.UUID {
	t.Helper()
	rows, err := pool.Loads(context.Background(), actor)
	if err != nil {
		t.Fatal(err)
	}
	out := make([]uuid.UUID, len(rows))
	for i, row := range rows {
		out[i] = row.ID
	}
	return out
}

func containsVisible(rows []uuid.UUID, id uuid.UUID) bool {
	for _, row := range rows {
		if row == id {
			return true
		}
	}
	return false
}
