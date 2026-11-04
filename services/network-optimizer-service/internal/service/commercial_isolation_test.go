package service

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/google/uuid"

	"github.com/freight-platform/network-optimizer-service/internal/domain"
)

func TestNLO09C_AnonymizedNextLoadOmitsCommercialAndKeepsLoad(t *testing.T) {
	w := newWorld(t)
	cap := w.capacity(domain.CapacityAvailable, domain.SourceManual, 0, 0)
	w.routes.defaultM = 10000
	w.routes.defaultSec = 600
	load := w.load(domain.VisAnonymized, place(0, kmDeg(2), "Kazan"), place(0, 1, "Ufa"), nil)
	w.reviseLoad(load, func(load *domain.LoadOpportunity) {
		load.Pickup.Label = "SECRET_DOCK"
		load.Commercial = domain.Commercial{Mode: "SECRET_MODE_9C", Amount: f64(771199), Currency: "RUB"}
	})

	body, doc := revenueBody(t, w, cap.ID)
	for _, leaked := range []string{"SECRET_MODE_9C", "771199", "SECRET_DOCK", w.shipper.String(), `"commercial"`} {
		if strings.Contains(body, leaked) {
			t.Fatalf("next-load leaked %s in %s", leaked, body)
		}
	}
	if len(doc.Candidates) != 1 || doc.Candidates[0].LoadOpportunityID != load.ID {
		t.Fatalf("anonymized load left the search %+v", candidateIDs(doc))
	}
	if doc.Candidates[0].Load.Commercial != nil {
		t.Fatalf("commercial %+v", doc.Candidates[0].Load.Commercial)
	}
	if doc.Candidates[0].Score == nil || doc.Candidates[0].ScoreStatus != domain.ScoreRanked {
		t.Fatalf("score changed %+v", doc.Candidates[0])
	}
	revenue := component(t, doc.Candidates[0], domain.ComponentRevenue)
	if revenue.RawValue != nil || revenue.RawUnit != "" {
		t.Fatalf("raw revenue %+v", revenue)
	}
	if strings.Contains(revenue.Explanation, "771199") || strings.Contains(revenue.Explanation, "SECRET_MODE_9C") {
		t.Fatal(revenue.Explanation)
	}

	listed, err := w.svc.GetMarketplaceLoad(context.Background(), w.actor(), load.ID)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(listed.Body), "771199") || strings.Contains(string(listed.Body), "SECRET_MODE_9C") || strings.Contains(string(listed.Body), `"commercial"`) {
		t.Fatalf("marketplace get leaked %s", listed.Body)
	}
}

func TestNLO09C_AnonymizedModeWithoutAmountIsAbsent(t *testing.T) {
	w := newWorld(t)
	cap := w.capacity(domain.CapacityAvailable, domain.SourceManual, 0, 0)
	w.routes.defaultM = 10000
	w.routes.defaultSec = 600
	load := w.load(domain.VisAnonymized, place(0, kmDeg(2), "P"), place(0, 1, "D"), nil)
	w.reviseLoad(load, func(load *domain.LoadOpportunity) {
		load.Commercial = domain.Commercial{Mode: "SECRET_MODE_9C", Currency: "ZZX"}
	})
	body, doc := revenueBody(t, w, cap.ID)
	if strings.Contains(body, "SECRET_MODE_9C") || strings.Contains(body, "ZZX") || strings.Contains(body, `"commercial"`) {
		t.Fatalf("%s", body)
	}
	if len(doc.Candidates) != 1 || doc.Candidates[0].LoadOpportunityID != load.ID || doc.Candidates[0].Load.Commercial != nil {
		t.Fatalf("mode-only load changed %+v", doc.Candidates)
	}
}

func TestNLO09C_AnonymizedCurrencyOnlyStaysAbsent(t *testing.T) {
	w := newWorld(t)
	cap := w.capacity(domain.CapacityAvailable, domain.SourceManual, 0, 0)
	w.routes.defaultM = 10000
	w.routes.defaultSec = 600
	load := w.load(domain.VisAnonymized, place(0, kmDeg(2), "P"), place(0, 1, "D"), nil)
	w.reviseLoad(load, func(load *domain.LoadOpportunity) {
		load.Commercial = domain.Commercial{Currency: "NOK"}
	})
	body, doc := revenueBody(t, w, cap.ID)
	if strings.Contains(body, "NOK") || strings.Contains(body, `"commercial"`) {
		t.Fatalf("%s", body)
	}
	if len(doc.Candidates) != 1 || doc.Candidates[0].LoadOpportunityID != load.ID {
		t.Fatalf("currency-only load disappeared %+v", candidateIDs(doc))
	}
}

func TestNLO09C_MarketplaceAndInvitedKeepPublishedCommercial(t *testing.T) {
	w := newWorld(t)
	cap := w.capacity(domain.CapacityAvailable, domain.SourceManual, 0, 0)
	w.routes.defaultM = 10000
	w.routes.defaultSec = 600
	company := uuid.New()
	market := pricedLoad(w, 880011, "RUB", kmDeg(2))
	invited := w.loadOwned(w.shipper, domain.VisInvited, []uuid.UUID{company}, place(0, kmDeg(3), "Invite"), place(0, 1, "D"), nil)
	w.reviseLoad(invited, func(load *domain.LoadOpportunity) {
		load.Commercial = domain.Commercial{Mode: "PUBLISHED", Amount: f64(660022), Currency: "RUB"}
	})
	actor := serviceActor(w.carrier, &company)
	body, doc := revenueBodyActor(t, w, actor, cap.ID)
	foundMarket, foundInvited := false, false
	for _, candidate := range doc.Candidates {
		switch candidate.LoadOpportunityID {
		case market.ID:
			foundMarket = candidate.Load.Commercial != nil && candidate.Load.Commercial.Amount != nil && *candidate.Load.Commercial.Amount == 880011
			revenue := component(t, candidate, domain.ComponentRevenue)
			if revenue.RawValue == nil || *revenue.RawValue != 880011 || revenue.RawUnit != "RUB" {
				t.Fatalf("marketplace raw revenue %+v", revenue)
			}
		case invited.ID:
			foundInvited = candidate.Load.Commercial != nil && candidate.Load.Commercial.Amount != nil && *candidate.Load.Commercial.Amount == 660022
		}
	}
	if !foundMarket || !foundInvited {
		t.Fatalf("identified commercial dropped market=%v invited=%v body=%s", foundMarket, foundInvited, body)
	}
}

func revenueBody(t *testing.T, w *world, capacityID uuid.UUID) (string, SearchResponse) {
	t.Helper()
	return revenueBodyActor(t, w, w.actor(), capacityID)
}

func revenueBodyActor(t *testing.T, w *world, actor Actor, capacityID uuid.UUID) (string, SearchResponse) {
	t.Helper()
	policy := radiusPolicy(500, 0)
	policy.ObjectiveProfile = domain.ProfileMaxRevenue
	policy.RankingCurrency = "RUB"
	result, err := w.svc.SearchNextLoad(context.Background(), actor, SearchCommand{CapacityID: capacityID, Policy: policy})
	if err != nil {
		t.Fatal(err)
	}
	var doc SearchResponse
	if err := json.Unmarshal(result.Body, &doc); err != nil {
		t.Fatal(err)
	}
	return string(result.Body), doc
}
