//go:build integration

package repository

import (
	"bytes"
	"context"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/freight-platform/network-optimizer-service/internal/domain"
)

func TestI2C_PG_CHAIN_POOL(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 4*time.Minute)
	defer cancel()
	pool := startPostgres(t, ctx)
	applyThrough000097(t, ctx, pool)
	store := NewPostgres(pool)
	now := time.Date(2026, 10, 16, 9, 0, 0, 0, time.UTC)

	t.Run("PG_CHAIN_POOL_OWN_OWN", func(t *testing.T) {
		viewer, company := uuid.New(), uuid.New()
		a := insertDiscoveryLoad(t, ctx, store, discoveryLoad(viewer, now, true, false, domain.VisPrivate, domain.LoadPublished, nil))
		b := insertDiscoveryLoad(t, ctx, store, discoveryLoad(viewer, now.Add(time.Second), true, false, domain.VisPrivate, domain.LoadPublished, nil))
		rows := chainPool(t, ctx, store, viewer, &company, 100)
		if !hasLoad(rows, a.ID) || !hasLoad(rows, b.ID) {
			t.Fatalf("own+own missing from %+v", ids(rows))
		}
		market := marketplacePool(t, ctx, store, viewer, &company, 100)
		if hasLoad(market, a.ID) || hasLoad(market, b.ID) {
			t.Fatal("own loads entered the shared marketplace list")
		}
	})

	t.Run("PG_CHAIN_POOL_OWN_FOREIGN", func(t *testing.T) {
		viewer, company, shipper := uuid.New(), uuid.New(), uuid.New()
		own := insertDiscoveryLoad(t, ctx, store, discoveryLoad(viewer, now, true, false, domain.VisPrivate, domain.LoadPublished, nil))
		foreign := insertDiscoveryLoad(t, ctx, store, discoveryLoad(shipper, now, false, true, domain.VisMarketplace, domain.LoadPublished, nil))
		rows := chainPool(t, ctx, store, viewer, &company, 100)
		if !hasLoad(rows, own.ID) || !hasLoad(rows, foreign.ID) {
			t.Fatalf("own+foreign missing from %+v", ids(rows))
		}
	})

	t.Run("PG_CHAIN_POOL_OWN_CONSOLIDATION_DENIED", func(t *testing.T) {
		viewer, company := uuid.New(), uuid.New()
		denied := insertDiscoveryLoad(t, ctx, store, discoveryLoad(viewer, now, false, true, domain.VisMarketplace, domain.LoadPublished, nil))
		if hasLoad(chainPool(t, ctx, store, viewer, &company, 100), denied.ID) {
			t.Fatal("own load with consolidation_allowed=false entered the chain pool")
		}
	})

	t.Run("PG_CHAIN_POOL_FOREIGN_VISIBILITY", func(t *testing.T) {
		viewer, company, other, shipper := uuid.New(), uuid.New(), uuid.New(), uuid.New()
		visible := insertDiscoveryLoad(t, ctx, store, discoveryLoad(shipper, now, false, true, domain.VisMarketplace, domain.LoadPublished, nil))
		anonymized := insertDiscoveryLoad(t, ctx, store, discoveryLoad(shipper, now, false, true, domain.VisAnonymized, domain.LoadPublished, nil))
		noCross := insertDiscoveryLoad(t, ctx, store, discoveryLoad(shipper, now, true, false, domain.VisMarketplace, domain.LoadPublished, nil))
		private := insertDiscoveryLoad(t, ctx, store, discoveryLoad(shipper, now, false, true, domain.VisPrivate, domain.LoadPublished, nil))
		draft := insertDiscoveryLoad(t, ctx, store, discoveryLoad(shipper, now, false, true, domain.VisMarketplace, domain.LoadDraft, nil))
		invited := insertDiscoveryLoad(t, ctx, store, discoveryLoad(shipper, now, false, true, domain.VisInvited, domain.LoadPublished, []uuid.UUID{company}))
		uninvited := insertDiscoveryLoad(t, ctx, store, discoveryLoad(shipper, now, false, true, domain.VisInvited, domain.LoadPublished, []uuid.UUID{other}))
		rows := chainPool(t, ctx, store, viewer, &company, 100)
		for _, id := range []uuid.UUID{visible.ID, anonymized.ID, invited.ID} {
			if !hasLoad(rows, id) {
				t.Fatalf("eligible foreign %s missing", id)
			}
		}
		for _, id := range []uuid.UUID{noCross.ID, private.ID, draft.ID, uninvited.ID} {
			if hasLoad(rows, id) {
				t.Fatalf("ineligible foreign %s leaked", id)
			}
		}
	})

	t.Run("PG_CHAIN_POOL_COMBINED_LIMIT", func(t *testing.T) {
		if _, err := pool.Exec(ctx, `DELETE FROM network_optimizer.load_opportunities`); err != nil {
			t.Fatal(err)
		}
		viewer, company, shipper := uuid.New(), uuid.New(), uuid.New()
		stamp := now
		older := insertDiscoveryLoad(t, ctx, store, discoveryLoad(viewer, stamp, true, false, domain.VisPrivate, domain.LoadPublished, nil))
		middle := insertDiscoveryLoad(t, ctx, store, discoveryLoad(shipper, stamp.Add(time.Second), false, true, domain.VisMarketplace, domain.LoadPublished, nil))
		newest := insertDiscoveryLoad(t, ctx, store, discoveryLoad(viewer, stamp.Add(2*time.Second), true, false, domain.VisPrivate, domain.LoadPublished, nil))
		rows := chainPool(t, ctx, store, viewer, &company, 2)
		if len(rows) != 2 || rows[0].ID != newest.ID || rows[1].ID != middle.ID || hasLoad(rows, older.ID) {
			t.Fatalf("combined limit %+v", ids(rows))
		}
		tiedA := discoveryLoad(viewer, stamp.Add(3*time.Second), true, false, domain.VisPrivate, domain.LoadPublished, nil)
		tiedB := discoveryLoad(viewer, stamp.Add(3*time.Second), true, false, domain.VisPrivate, domain.LoadPublished, nil)
		insertDiscoveryLoad(t, ctx, store, tiedA)
		insertDiscoveryLoad(t, ctx, store, tiedB)
		tied := chainPool(t, ctx, store, viewer, &company, 2)
		if len(tied) != 2 || !tied[0].CreatedAt.Equal(tied[1].CreatedAt) || bytes.Compare(tied[0].ID[:], tied[1].ID[:]) > 0 {
			t.Fatalf("tie order %+v", ids(tied))
		}
	})
}

func discoveryLoad(owner uuid.UUID, created time.Time, consolidation, cross bool, scope, status string, invited []uuid.UUID) domain.LoadOpportunity {
	lat, lon := 55.0, 37.0
	weight := 10.0
	start := created
	end := created.Add(time.Hour)
	return domain.LoadOpportunity{
		ID: uuid.New(), OwnerTenantID: owner, SourceType: domain.SourceTransportOrder, SourceID: uuid.New(),
		Pickup:          domain.Place{Latitude: &lat, Longitude: &lon},
		Delivery:        domain.Place{Latitude: &lat, Longitude: &lon},
		PickupWindow:    domain.TimeWindow{Start: &start, End: &end},
		DeliveryWindow:  domain.TimeWindow{Start: &start, End: &end},
		WeightKg:        &weight,
		VisibilityScope: scope, InvitedCarrierCompanyIDs: invited,
		ConsolidationAllowed: consolidation, CrossShipperConsolidationAllowed: cross,
		Status: status, Version: 1, CreatedAt: created, UpdatedAt: created,
	}
}

func insertDiscoveryLoad(t *testing.T, ctx context.Context, store *Postgres, load domain.LoadOpportunity) domain.LoadOpportunity {
	t.Helper()
	if err := store.Within(ctx, func(tx Tx) error { return tx.InsertLoad(ctx, load) }); err != nil {
		t.Fatal(err)
	}
	return load
}

func chainPool(t *testing.T, ctx context.Context, store *Postgres, viewer uuid.UUID, company *uuid.UUID, limit int) []domain.LoadOpportunity {
	t.Helper()
	var rows []domain.LoadOpportunity
	err := store.Within(ctx, func(tx Tx) error {
		listed, err := tx.ListChainDiscoveryLoads(ctx, viewer, company, limit, 0)
		rows = listed
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
	return rows
}

func marketplacePool(t *testing.T, ctx context.Context, store *Postgres, viewer uuid.UUID, company *uuid.UUID, limit int) []domain.LoadOpportunity {
	t.Helper()
	var rows []domain.LoadOpportunity
	err := store.Within(ctx, func(tx Tx) error {
		listed, err := tx.ListMarketplaceLoads(ctx, viewer, company, limit, 0)
		rows = listed
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
	return rows
}

func hasLoad(rows []domain.LoadOpportunity, id uuid.UUID) bool {
	for _, row := range rows {
		if row.ID == id {
			return true
		}
	}
	return false
}

func ids(rows []domain.LoadOpportunity) []uuid.UUID {
	out := make([]uuid.UUID, len(rows))
	for i, row := range rows {
		out[i] = row.ID
	}
	return out
}
