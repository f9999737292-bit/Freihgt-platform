package service

import (
	"context"
	"encoding/json"
	"strconv"
	"strings"
	"testing"

	"github.com/google/uuid"

	"github.com/freight-platform/network-optimizer-service/internal/domain"
	"github.com/freight-platform/network-optimizer-service/internal/profilefixture"
)

// TestNLO09CR1_StockBalancedHasNoRevenueComponent records why the BALANCED
// hidden-rate test cannot prove revenue isolation. Issue #337 stays open.
func TestNLO09CR1_StockBalancedHasNoRevenueComponent(t *testing.T) {
	var found bool
	for _, profile := range profilefixture.V1() {
		if profile.Code != domain.ProfileBalanced {
			continue
		}
		found = true
		for _, item := range profile.Components {
			if item.Code == domain.ComponentRevenue {
				t.Fatalf("stock BALANCED contains REVENUE weight %d", item.WeightBps)
			}
		}
	}
	if !found {
		t.Fatal("stock BALANCED profile is missing")
	}
}

// TestNLO09CR1_MaxRevenueHiddenRateDoesNotMovePublicNormalization uses the
// stock MAX_REVENUE profile. Only the anonymized amount changes. This test
// fails on a62b25a9f94a16ba856b1e2caa225f0247d65b6d, before public revenue
// projection, and passes on the accepted fix.
func TestNLO09CR1_MaxRevenueHiddenRateDoesNotMovePublicNormalization(t *testing.T) {
	high := maxRevenueSnapshot(t, 97531)
	low := maxRevenueSnapshot(t, 13579)
	if high.lowNorm != 0 || high.highNorm != 10000 || low.lowNorm != high.lowNorm || low.highNorm != high.highNorm {
		t.Fatalf("hidden rate moved public normalization high=%+v low=%+v", high, low)
	}
	if high.lowRank != 2 || high.highRank != 1 || low.lowRank != high.lowRank || low.highRank != high.highRank {
		t.Fatalf("hidden rate moved public rank high=%+v low=%+v", high, low)
	}
	if high.lowScore != low.lowScore || high.highScore != low.highScore {
		t.Fatalf("hidden rate moved public score high=%+v low=%+v", high, low)
	}
	if high.anonNorm != nil || high.anonWeighted != nil || high.anonRaw != nil || low.anonNorm != nil || low.anonWeighted != nil || low.anonRaw != nil {
		t.Fatalf("hidden rate published revenue points high=%+v low=%+v", high, low)
	}
	if high.publishedLow != 100 || high.publishedHigh != 400 || low.publishedLow != 100 || low.publishedHigh != 400 {
		t.Fatalf("published rates changed high=%+v low=%+v", high, low)
	}
}

type revenueSnapshot struct {
	lowNorm       int
	highNorm      int
	lowRank       int
	highRank      int
	lowScore      int
	highScore     int
	publishedLow  float64
	publishedHigh float64
	anonNorm      *int
	anonWeighted  *int
	anonRaw       *float64
}

func maxRevenueSnapshot(t *testing.T, hidden float64) revenueSnapshot {
	t.Helper()
	w := newWorld(t)
	if w.carrier == w.shipper {
		t.Fatal("viewer tenant and load owner are the same")
	}
	cap := w.capacity(domain.CapacityAvailable, domain.SourceManual, 0, 0)
	w.routes.defaultM = 10000
	w.routes.defaultSec = 600
	pickup := place(0, kmDeg(2), "Area")
	delivery := place(0, 1, "Far")
	low := w.load(domain.VisMarketplace, pickup, delivery, nil)
	w.reviseLoad(low, func(load *domain.LoadOpportunity) {
		load.Commercial = domain.Commercial{Mode: "PUBLISHED", Amount: f64(100), Currency: "RUB"}
	})
	high := w.load(domain.VisMarketplace, pickup, delivery, nil)
	w.reviseLoad(high, func(load *domain.LoadOpportunity) {
		load.Commercial = domain.Commercial{Mode: "PUBLISHED", Amount: f64(400), Currency: "RUB"}
	})
	anon := w.load(domain.VisAnonymized, pickup, delivery, nil)
	w.reviseLoad(anon, func(load *domain.LoadOpportunity) {
		load.Commercial = domain.Commercial{Mode: "SECRET_MODE_9C", Amount: f64(hidden), Currency: "RUB"}
	})
	own := w.loadOwned(w.carrier, domain.VisMarketplace, nil, pickup, delivery, nil)
	w.reviseLoad(own, func(load *domain.LoadOpportunity) {
		load.Commercial = domain.Commercial{Mode: "PUBLISHED", Amount: f64(24680), Currency: "RUB"}
	})

	policy := radiusPolicy(500, 0)
	policy.ObjectiveProfile = domain.ProfileMaxRevenue
	policy.RankingCurrency = "RUB"
	result, err := w.svc.SearchNextLoad(context.Background(), w.actor(), SearchCommand{CapacityID: cap.ID, Policy: policy})
	if err != nil {
		t.Fatal(err)
	}
	body := string(result.Body)
	secret := strconv.FormatFloat(hidden, 'f', -1, 64)
	if strings.Contains(body, secret) || strings.Contains(body, "SECRET_MODE_9C") || strings.Contains(body, "24680") {
		t.Fatalf("public body leaked a hidden or owner rate: %s", body)
	}
	var doc SearchResponse
	if err := json.Unmarshal(result.Body, &doc); err != nil {
		t.Fatal(err)
	}
	if doc.Ranking.ObjectiveProfile != domain.ProfileMaxRevenue {
		t.Fatalf("search did not use MAX_REVENUE: %s", doc.Ranking.ObjectiveProfile)
	}
	if doc.EligibleCandidateCount != 3 || len(doc.Candidates) != 3 {
		t.Fatalf("marketplace visibility changed %+v", doc)
	}
	for _, candidate := range doc.Candidates {
		if candidate.LoadOpportunityID == own.ID {
			t.Fatalf("owner tenant load entered marketplace search %s", own.ID)
		}
	}
	lowView := r1Candidate(t, doc, low.ID)
	highView := r1Candidate(t, doc, high.ID)
	anonView := r1Candidate(t, doc, anon.ID)
	if lowView.Load.OwnerTenantID == nil || *lowView.Load.OwnerTenantID != w.shipper {
		t.Fatalf("identified owner changed %+v", lowView.Load.OwnerTenantID)
	}
	if anonView.Load.OwnerTenantID != nil || anonView.Load.Commercial != nil {
		t.Fatalf("anonymized publication changed %+v %+v", anonView.Load.OwnerTenantID, anonView.Load.Commercial)
	}
	if lowView.Load.Commercial == nil || lowView.Load.Commercial.Amount == nil || highView.Load.Commercial == nil || highView.Load.Commercial.Amount == nil {
		t.Fatal("published commercial terms were removed")
	}
	lowRevenue := component(t, lowView, domain.ComponentRevenue)
	highRevenue := component(t, highView, domain.ComponentRevenue)
	anonRevenue := component(t, anonView, domain.ComponentRevenue)
	if lowRevenue.NormalizedPoints == nil || highRevenue.NormalizedPoints == nil || lowView.Rank == nil || highView.Rank == nil || lowView.Score == nil || highView.Score == nil {
		t.Fatalf("published revenue was not scored low=%+v high=%+v", lowRevenue, highRevenue)
	}
	return revenueSnapshot{
		lowNorm: *lowRevenue.NormalizedPoints, highNorm: *highRevenue.NormalizedPoints,
		lowRank: *lowView.Rank, highRank: *highView.Rank,
		lowScore: *lowView.Score, highScore: *highView.Score,
		publishedLow: *lowView.Load.Commercial.Amount, publishedHigh: *highView.Load.Commercial.Amount,
		anonNorm: anonRevenue.NormalizedPoints, anonWeighted: anonRevenue.WeightedPoints, anonRaw: anonRevenue.RawValue,
	}
}

func r1Candidate(t *testing.T, doc SearchResponse, id uuid.UUID) SearchCandidateView {
	t.Helper()
	for _, candidate := range doc.Candidates {
		if candidate.LoadOpportunityID == id {
			return candidate
		}
	}
	t.Fatalf("load %s left the search", id)
	return SearchCandidateView{}
}
