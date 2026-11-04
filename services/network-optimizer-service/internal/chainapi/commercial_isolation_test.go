package chainapi

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/freight-platform/network-optimizer-service/internal/chain"
	"github.com/freight-platform/network-optimizer-service/internal/domain"
	"github.com/freight-platform/network-optimizer-service/internal/repository"
	"github.com/freight-platform/network-optimizer-service/internal/service"
)

func TestNLO09C_ChainDiscoveryOwnAndForeignOmitsCommercial(t *testing.T) {
	carrier := uuid.New()
	company := uuid.New()
	shipper := uuid.New()
	actor := service.Actor{TenantID: carrier, UserID: uuid.New(), CompanyID: &company}
	now := time.Date(2026, 10, 16, 12, 0, 0, 0, time.UTC)

	t.Run("own+own", func(t *testing.T) {
		store := repository.NewMemory()
		left := pricedPublished(carrier, now, true, false, domain.VisPrivate, nil, "SECRET_OWN_MODE", 111111, "ZZX")
		right := pricedPublished(carrier, now.Add(time.Second), true, false, domain.VisPrivate, nil, "SECRET_OWN_MODE_2", 111112, "ZZX")
		putLoad(t, store, left)
		putLoad(t, store, right)
		loads := mustLoads(t, MarketplacePool{Store: store}, actor)
		if !hasVisible(loads, left.ID) || !hasVisible(loads, right.ID) || len(loads) != 2 {
			t.Fatalf("own+own pool %+v", loads)
		}
		result := searchLoads(t, carrier, now, loads)
		if result.ReasonCounts[chain.ReasonConsolidation] != 0 || !resultHasLoad(result, left.ID) || !resultHasLoad(result, right.ID) {
			t.Fatalf("own+own discovery counts=%v candidates=%d", result.ReasonCounts, len(result.Candidates))
		}
		assertNoCommercial(t, result, "SECRET_OWN_MODE", "111111", "111112", "ZZX")
	})

	t.Run("own+foreign", func(t *testing.T) {
		store := repository.NewMemory()
		own := pricedPublished(carrier, now, true, false, domain.VisPrivate, nil, "SECRET_OWN_MODE", 111111, "ZZX")
		foreign := pricedPublished(shipper, now, false, true, domain.VisMarketplace, nil, "SECRET_FOREIGN_MODE", 222222, "ZZY")
		foreign.Pickup.Label = "SECRET_FOREIGN_DOCK"
		putLoad(t, store, own)
		putLoad(t, store, foreign)
		loads := mustLoads(t, MarketplacePool{Store: store}, actor)
		if !hasVisible(loads, own.ID) || !hasVisible(loads, foreign.ID) {
			t.Fatalf("own+foreign missing %+v", loads)
		}
		result := searchLoads(t, carrier, now, loads)
		if result.ReasonCounts[chain.ReasonConsolidation] != 0 || !resultHasLoad(result, own.ID) || !resultHasLoad(result, foreign.ID) {
			t.Fatalf("own+foreign discovery counts=%v candidates=%d", result.ReasonCounts, len(result.Candidates))
		}
		assertNoCommercial(t, result, "SECRET_OWN_MODE", "SECRET_FOREIGN_MODE", "SECRET_FOREIGN_DOCK", "111111", "222222", shipper.String(), "ZZY")
		encoded, err := json.Marshal(loads)
		if err != nil {
			t.Fatal(err)
		}
		if strings.Contains(string(encoded), "SECRET_FOREIGN_MODE") || strings.Contains(string(encoded), "222222") {
			t.Fatalf("visible pool copied commercial %s", encoded)
		}
	})

	t.Run("anonymized foreign stays searchable", func(t *testing.T) {
		store := repository.NewMemory()
		own := pricedPublished(carrier, now, true, false, domain.VisPrivate, nil, "SECRET_OWN_MODE", 111111, "ZZX")
		anon := pricedPublished(shipper, now, false, true, domain.VisAnonymized, nil, "SECRET_ANON_MODE", 333333, "ZZZ")
		anon.Pickup.Label = "SECRET_ANON_DOCK"
		putLoad(t, store, own)
		putLoad(t, store, anon)
		loads := mustLoads(t, MarketplacePool{Store: store}, actor)
		if !hasVisible(loads, own.ID) || !hasVisible(loads, anon.ID) {
			t.Fatal("anonymized foreign load disappeared from the chain pool")
		}
		raw, err := domain.MarshalMarketplaceLoad(anon)
		if err != nil {
			t.Fatal(err)
		}
		if strings.Contains(string(raw), "SECRET_ANON_MODE") || strings.Contains(string(raw), "333333") || strings.Contains(string(raw), `"commercial"`) {
			t.Fatalf("anonymized marketplace projection %s", raw)
		}
		result := searchLoads(t, carrier, now, loads)
		if !resultHasLoad(result, anon.ID) || !resultHasLoad(result, own.ID) {
			t.Fatal("anonymized pair disappeared from chain discovery")
		}
		assertNoCommercial(t, result, "SECRET_ANON_MODE", "SECRET_ANON_DOCK", "333333", "ZZZ")
	})

	t.Run("ineligible foreign stays out", func(t *testing.T) {
		store := repository.NewMemory()
		noCross := pricedPublished(shipper, now, true, false, domain.VisMarketplace, nil, "SECRET_DENIED", 444444, "ZZZ")
		private := pricedPublished(shipper, now, false, true, domain.VisPrivate, nil, "SECRET_PRIVATE", 444445, "ZZZ")
		draft := pricedPublished(shipper, now, false, true, domain.VisMarketplace, nil, "SECRET_DRAFT", 444446, "ZZZ")
		draft.Status = domain.LoadDraft
		uninvited := pricedPublished(shipper, now, false, true, domain.VisInvited, []uuid.UUID{uuid.New()}, "SECRET_UNINVITED", 444447, "ZZZ")
		for _, load := range []domain.LoadOpportunity{noCross, private, draft, uninvited} {
			putLoad(t, store, load)
		}
		loads := mustLoads(t, MarketplacePool{Store: store}, actor)
		if len(loads) != 0 {
			t.Fatalf("ineligible foreign entered the pool %+v", loads)
		}
	})
}

func TestNLO09C_ChainDiscoveryBoundKeepsNewestEligible(t *testing.T) {
	ctx := context.Background()
	store := repository.NewMemory()
	viewer, company, shipper := uuid.New(), uuid.New(), uuid.New()
	now := time.Date(2026, 10, 16, 12, 0, 0, 0, time.UTC)
	older := pricedPublished(viewer, now, true, false, domain.VisPrivate, nil, "SECRET_OLD", 555551, "ZZX")
	middle := pricedPublished(shipper, now.Add(time.Second), false, true, domain.VisMarketplace, nil, "SECRET_MID", 555552, "ZZY")
	newest := pricedPublished(viewer, now.Add(2*time.Second), true, false, domain.VisPrivate, nil, "SECRET_NEW", 555553, "ZZX")
	for _, load := range []domain.LoadOpportunity{older, middle, newest} {
		putLoad(t, store, load)
	}
	var rows []domain.LoadOpportunity
	err := store.Within(ctx, func(tx repository.Tx) error {
		listed, err := tx.ListChainDiscoveryLoads(ctx, viewer, &company, 2, 0)
		rows = listed
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 2 || rows[0].ID != newest.ID || rows[1].ID != middle.ID {
		t.Fatalf("bounded pool %+v", rows)
	}
	if rows[0].Commercial.Mode != "SECRET_NEW" || rows[1].Commercial.Amount == nil {
		t.Fatal("internal chain read dropped commercial facts required for scoring")
	}
}

func pricedPublished(owner uuid.UUID, now time.Time, consolidation, cross bool, scope string, invited []uuid.UUID, mode string, amount float64, currency string) domain.LoadOpportunity {
	load := publishedLoad(owner, now, consolidation, cross, scope, invited)
	value := amount
	load.Commercial = domain.Commercial{Mode: mode, Amount: &value, Currency: currency}
	return load
}

func hasVisible(loads []chain.VisibleLoad, id uuid.UUID) bool {
	for _, load := range loads {
		if load.ID == id {
			return true
		}
	}
	return false
}

func resultHasLoad(result chain.Result, id uuid.UUID) bool {
	for _, candidate := range result.Candidates {
		for _, load := range candidate.Loads {
			if load.LoadID == id {
				return true
			}
		}
	}
	return false
}

func assertNoCommercial(t *testing.T, result chain.Result, needles ...string) {
	t.Helper()
	body, err := json.Marshal(result)
	if err != nil {
		t.Fatal(err)
	}
	text := string(body)
	for _, needle := range needles {
		if strings.Contains(text, needle) {
			t.Fatalf("chain result leaked %s in %s", needle, text)
		}
	}
	if strings.Contains(text, `"commercial"`) || strings.Contains(text, "revenue_amount") || strings.Contains(text, "contract") {
		t.Fatalf("chain result exposed commercial fields %s", text)
	}
}
