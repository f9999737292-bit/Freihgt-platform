package repository

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/freight-platform/network-optimizer-service/internal/domain"
)

func TestMarketplaceDiscoveryCapOrdering(t *testing.T) {
	store := NewMemory()
	viewer := uuid.New()
	owner := uuid.New()
	base := time.Date(2026, 10, 3, 8, 0, 0, 0, time.UTC)
	const extra = 5
	total := domain.CandidateDiscoveryCap + extra
	var newest uuid.UUID
	var oldestKept uuid.UUID
	err := store.Within(context.Background(), func(tx Tx) error {
		if err := tx.InsertLoad(context.Background(), marketplaceLoad(viewer, base.Add(-time.Hour))); err != nil {
			return err
		}
		hidden := marketplaceLoad(owner, base.Add(-time.Minute))
		hidden.VisibilityScope = domain.VisPrivate
		if err := tx.InsertLoad(context.Background(), hidden); err != nil {
			return err
		}
		for i := 0; i < total; i++ {
			load := marketplaceLoad(owner, base.Add(time.Duration(i)*time.Millisecond))
			if i == total-1 {
				newest = load.ID
			}
			if i == extra {
				oldestKept = load.ID
			}
			if err := tx.InsertLoad(context.Background(), load); err != nil {
				return err
			}
		}
		count, err := tx.CountMarketplaceLoads(context.Background(), viewer, nil)
		if err != nil {
			return err
		}
		if count != total {
			t.Fatalf("visible count %d", count)
		}
		rows, err := tx.ListMarketplaceLoads(context.Background(), viewer, nil, domain.CandidateDiscoveryCap, 0)
		if err != nil {
			return err
		}
		if len(rows) != domain.CandidateDiscoveryCap {
			t.Fatalf("page %d", len(rows))
		}
		if rows[0].ID != newest || rows[len(rows)-1].ID != oldestKept {
			t.Fatalf("order first %s last %s", rows[0].ID, rows[len(rows)-1].ID)
		}
		for i := 1; i < len(rows); i++ {
			if rows[i-1].CreatedAt.Before(rows[i].CreatedAt) {
				t.Fatalf("created_at order %s before %s", rows[i-1].ID, rows[i].ID)
			}
			if rows[i-1].ID == rows[i].ID {
				t.Fatal("duplicate id")
			}
		}
		tiedEarly := marketplaceLoad(owner, base.Add(2*time.Hour))
		tiedLate := marketplaceLoad(owner, base.Add(2*time.Hour))
		if err := tx.InsertLoad(context.Background(), tiedEarly); err != nil {
			return err
		}
		if err := tx.InsertLoad(context.Background(), tiedLate); err != nil {
			return err
		}
		tied, err := tx.ListMarketplaceLoads(context.Background(), viewer, nil, 2, 0)
		if err != nil {
			return err
		}
		if len(tied) != 2 || !tied[0].CreatedAt.Equal(tied[1].CreatedAt) {
			t.Fatalf("tie page %+v", tied)
		}
		if tied[0].ID.String() > tied[1].ID.String() {
			t.Fatalf("id tie-break %s %s", tied[0].ID, tied[1].ID)
		}
		canceled, cancel := context.WithCancel(context.Background())
		cancel()
		if _, err := tx.ListMarketplaceLoads(canceled, viewer, nil, 10, 0); !errors.Is(err, context.Canceled) {
			t.Fatalf("list cancellation %v", err)
		}
		if _, err := tx.CountMarketplaceLoads(canceled, viewer, nil); !errors.Is(err, context.Canceled) {
			t.Fatalf("count cancellation %v", err)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}

func marketplaceLoad(owner uuid.UUID, created time.Time) domain.LoadOpportunity {
	lat, lon := 55.0, 37.0
	return domain.LoadOpportunity{
		ID: uuid.New(), OwnerTenantID: owner, SourceType: domain.SourceTransportOrder, SourceID: uuid.New(),
		Pickup:          domain.Place{Latitude: &lat, Longitude: &lon, City: "A", CountryCode: "RU"},
		Delivery:        domain.Place{Latitude: &lat, Longitude: &lon, City: "B", CountryCode: "RU"},
		VisibilityScope: domain.VisMarketplace, Status: domain.LoadPublished, Version: 1,
		CreatedAt: created, UpdatedAt: created,
	}
}
