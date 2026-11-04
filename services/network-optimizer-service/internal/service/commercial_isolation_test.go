package service

import (
	"context"
	"encoding/json"
	"strconv"
	"strings"
	"testing"

	"github.com/google/uuid"

	"github.com/freight-platform/network-optimizer-service/internal/domain"
	"github.com/freight-platform/network-optimizer-service/internal/repository"
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
	if doc.Candidates[0].Score != nil || doc.Candidates[0].Rank != nil || doc.Candidates[0].ScoreStatus == domain.ScoreRanked {
		t.Fatalf("public score still depends on the hidden rate %+v", doc.Candidates[0])
	}
	revenue := component(t, doc.Candidates[0], domain.ComponentRevenue)
	if revenue.RawValue != nil || revenue.RawUnit != "" || revenue.NormalizedPoints != nil || revenue.WeightedPoints != nil {
		t.Fatalf("revenue points %+v", revenue)
	}
	if strings.Contains(revenue.Explanation, "771199") || strings.Contains(revenue.Explanation, "SECRET_MODE_9C") {
		t.Fatal(revenue.Explanation)
	}
	_, stored, err := w.store.GetSearch(context.Background(), w.carrier, doc.SearchID)
	if err != nil || len(stored) != 1 || stored[0].ScoreTotal == nil || *stored[0].ScoreTotal != 5000 {
		t.Fatalf("private score changed %+v %v", stored, err)
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
	otherCompany := uuid.New()
	otherBody, otherDoc := revenueBodyActor(t, w, serviceActor(w.carrier, &otherCompany), cap.ID)
	if strings.Contains(otherBody, "660022") {
		t.Fatalf("foreign company saw invited commercial %s", otherBody)
	}
	for _, candidate := range otherDoc.Candidates {
		if candidate.LoadOpportunityID == invited.ID {
			t.Fatal("foreign company received the invited load")
		}
	}
	if _, err := w.svc.SearchNextLoad(context.Background(), serviceActor(uuid.New(), &company), SearchCommand{CapacityID: cap.ID, Policy: maxRevenuePolicy()}); !notFound(err) {
		t.Fatalf("foreign tenant searched the capacity: %v", err)
	}
}

func TestNLO09C_HiddenRevenueDoesNotMovePublicNormalization(t *testing.T) {
	base := revenuePool(t, []float64{100, 400}, nil)
	high := revenuePool(t, []float64{100, 400}, []float64{97531})
	low := revenuePool(t, []float64{100, 400}, []float64{80})
	several := revenuePool(t, []float64{100, 400}, []float64{97531, 13579})
	for _, pool := range []revenueFixture{base, high, low, several} {
		assertPublishedRevenueScale(t, pool)
	}
	assertHiddenRevenueSilent(t, high, "97531")
	assertHiddenRevenueSilent(t, low, "SECRET_MODE_9C")
	assertHiddenRevenueSilent(t, several, "97531")
	assertHiddenRevenueSilent(t, several, "13579")
	if high.doc.EligibleCandidateCount != 3 || len(high.doc.Candidates) != 3 {
		t.Fatalf("anonymized load left discovery %+v", high.doc)
	}
	if several.doc.EligibleCandidateCount != 4 || len(several.doc.Candidates) != 4 {
		t.Fatalf("second anonymized load left discovery %+v", several.doc)
	}
	_, stored, err := high.world.store.GetSearch(context.Background(), high.world.carrier, high.doc.SearchID)
	if err != nil || !storedContains(stored, "97531") {
		t.Fatal("private scoring no longer sees the hidden amount")
	}
	if strings.Contains(high.body, "97531") {
		t.Fatal("stored amount was copied into the public body")
	}

	limit := 1
	limited := revenuePoolLimit(t, []float64{100, 400}, []float64{97531}, &limit)
	if limited.doc.EligibleCandidateCount != 3 || limited.doc.ReturnedCandidateCount != 1 || len(limited.doc.Candidates) != 1 {
		t.Fatalf("limited search dropped the pool %+v", limited.doc)
	}
	if limited.doc.Candidates[0].LoadOpportunityID != limited.identified[400] {
		t.Fatalf("hidden rate won the public rank %+v", limited.doc.Candidates[0].LoadOpportunityID)
	}
	if strings.Contains(limited.body, "97531") {
		t.Fatal(limited.body)
	}
}

func TestNLO09C_AllAnonymizedRevenueStaysUnpublished(t *testing.T) {
	first := revenuePool(t, nil, []float64{11111, 22222, 33333})
	second := revenuePool(t, nil, []float64{44444, 55555, 66666})
	for _, pool := range []revenueFixture{first, second} {
		if pool.doc.RankedCandidateCount != 0 || len(pool.doc.Candidates) != 3 || pool.doc.EligibleCandidateCount != 3 {
			t.Fatalf("anonymized pool changed %+v", pool.doc)
		}
		for _, secret := range pool.secrets {
			assertHiddenRevenueSilent(t, pool, secret)
		}
		for _, candidate := range pool.doc.Candidates {
			if candidate.Score != nil || candidate.Rank != nil || candidate.ScoreStatus == domain.ScoreRanked {
				t.Fatalf("public rank reveals anonymized revenue %+v", candidate)
			}
		}
	}
}

func TestNLO09C_BalancedProfileDoesNotPublishHiddenRate(t *testing.T) {
	left := balancedAnonScore(t, 424242)
	right := balancedAnonScore(t, 818181)
	if left != right {
		t.Fatalf("BALANCED score changed with the hidden rate %d vs %d", left, right)
	}
}

func TestNLO09C_RevenueBearingProfileOmitsHiddenRate(t *testing.T) {
	high := mixedRevenueScore(t, 97531)
	low := mixedRevenueScore(t, 99.5)
	if high.anonScore != low.anonScore || high.anonEvidence != low.anonEvidence || high.identifiedScore != low.identifiedScore {
		t.Fatalf("public score moved with the hidden rate high=%+v low=%+v", high, low)
	}
	if high.anonScore == 0 || high.identifiedRaw != 250 || high.anonPoints != nil || high.identifiedRawPoints == nil {
		t.Fatalf("projection substituted a zero or dropped the published rate %+v", high)
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

type revenueFixture struct {
	world      *world
	body       string
	doc        SearchResponse
	identified map[float64]uuid.UUID
	anonymized []uuid.UUID
	secrets    []string
}

type mixedRevenue struct {
	anonScore           int
	anonEvidence        int
	identifiedScore     int
	identifiedRaw       float64
	identifiedRawPoints *int
	anonPoints          *int
}

func maxRevenuePolicy() domain.NextLoadSearchPolicy {
	policy := radiusPolicy(500, 0)
	policy.ObjectiveProfile = domain.ProfileMaxRevenue
	policy.RankingCurrency = "RUB"
	return policy
}

func revenuePool(t *testing.T, identified, anonymized []float64) revenueFixture {
	t.Helper()
	return buildRevenuePool(t, identified, anonymized, nil)
}

func revenuePoolLimit(t *testing.T, identified, anonymized []float64, limit *int) revenueFixture {
	t.Helper()
	return buildRevenuePool(t, identified, anonymized, limit)
}

func buildRevenuePool(t *testing.T, identified, anonymized []float64, limit *int) revenueFixture {
	t.Helper()
	w := newWorld(t)
	cap := w.capacity(domain.CapacityAvailable, domain.SourceManual, 0, 0)
	w.routes.defaultM = 10000
	w.routes.defaultSec = 600
	fixture := revenueFixture{world: w, identified: map[float64]uuid.UUID{}}
	lon := 2
	for _, amount := range identified {
		load := pricedLoad(w, amount, "RUB", kmDeg(float64(lon)))
		fixture.identified[amount] = load.ID
		lon++
	}
	for _, amount := range anonymized {
		load := w.load(domain.VisAnonymized, place(0, kmDeg(float64(lon)), "Area"), place(0, 1, "Far"), nil)
		w.reviseLoad(load, func(load *domain.LoadOpportunity) {
			load.Commercial = domain.Commercial{Mode: "SECRET_MODE_9C", Amount: f64(amount), Currency: "RUB"}
		})
		fixture.anonymized = append(fixture.anonymized, load.ID)
		fixture.secrets = append(fixture.secrets, strconv.FormatFloat(amount, 'f', -1, 64))
		lon++
	}
	policy := maxRevenuePolicy()
	result, err := w.svc.SearchNextLoad(context.Background(), w.actor(), SearchCommand{CapacityID: cap.ID, Policy: policy, CandidateLimit: limit})
	if err != nil {
		t.Fatal(err)
	}
	fixture.body = string(result.Body)
	if err := json.Unmarshal(result.Body, &fixture.doc); err != nil {
		t.Fatal(err)
	}
	return fixture
}

func assertPublishedRevenueScale(t *testing.T, pool revenueFixture) {
	t.Helper()
	low := candidateByID(t, pool.doc, pool.identified[100])
	high := candidateByID(t, pool.doc, pool.identified[400])
	if low.Load.Commercial == nil || low.Load.Commercial.Amount == nil || *low.Load.Commercial.Amount != 100 {
		t.Fatalf("published low rate dropped %+v", low.Load.Commercial)
	}
	if high.Load.Commercial == nil || high.Load.Commercial.Amount == nil || *high.Load.Commercial.Amount != 400 {
		t.Fatalf("published high rate dropped %+v", high.Load.Commercial)
	}
	lowRev := component(t, low, domain.ComponentRevenue)
	highRev := component(t, high, domain.ComponentRevenue)
	if lowRev.NormalizedPoints == nil || *lowRev.NormalizedPoints != 0 || lowRev.WeightedPoints == nil || *lowRev.WeightedPoints != 0 || lowRev.RawValue == nil || *lowRev.RawValue != 100 {
		t.Fatalf("low public revenue %+v", lowRev)
	}
	if highRev.NormalizedPoints == nil || *highRev.NormalizedPoints != 10000 || highRev.WeightedPoints == nil || *highRev.WeightedPoints != 10000 || highRev.RawValue == nil || *highRev.RawValue != 400 {
		t.Fatalf("high public revenue %+v", highRev)
	}
	if low.Rank == nil || high.Rank == nil || *high.Rank != 1 || *low.Rank != 2 || low.Score == nil || high.Score == nil {
		t.Fatalf("public ranks low=%v high=%v", low.Rank, high.Rank)
	}
}

func assertHiddenRevenueSilent(t *testing.T, pool revenueFixture, secret string) {
	t.Helper()
	if strings.Contains(pool.body, secret) || strings.Contains(pool.body, "SECRET_MODE_9C") || strings.Contains(pool.body, "score_fingerprint") {
		t.Fatalf("public body leaked %s in %s", secret, pool.body)
	}
	for _, id := range pool.anonymized {
		view := candidateByID(t, pool.doc, id)
		if view.Load.Commercial != nil || view.Score != nil || view.Rank != nil || view.ScoreEvidenceBps != nil || view.ScoreStatus == domain.ScoreRanked {
			t.Fatalf("hidden revenue inference surface %+v", view)
		}
		revenue := component(t, view, domain.ComponentRevenue)
		if revenue.RawValue != nil || revenue.RawUnit != "" || revenue.NormalizedPoints != nil || revenue.WeightedPoints != nil {
			t.Fatalf("hidden revenue points %+v", revenue)
		}
		if strings.Contains(revenue.Explanation, secret) {
			t.Fatal(revenue.Explanation)
		}
	}
}

func candidateByID(t *testing.T, doc SearchResponse, id uuid.UUID) SearchCandidateView {
	t.Helper()
	for _, candidate := range doc.Candidates {
		if candidate.LoadOpportunityID == id {
			return candidate
		}
	}
	t.Fatalf("load %s left the search", id)
	return SearchCandidateView{}
}

func storedContains(rows []repository.StoredCandidate, needle string) bool {
	for _, row := range rows {
		if strings.Contains(string(row.ScoreComponents), needle) {
			return true
		}
	}
	return false
}

func balancedAnonScore(t *testing.T, amount float64) int {
	t.Helper()
	w := newWorld(t)
	cap := w.capacity(domain.CapacityAvailable, domain.SourceManual, 0, 0)
	w.routes.defaultM = 10000
	w.routes.defaultSec = 600
	load := w.load(domain.VisAnonymized, place(0, kmDeg(2), "Area"), place(0, 1, "Far"), nil)
	w.reviseLoad(load, func(load *domain.LoadOpportunity) {
		load.Commercial = domain.Commercial{Mode: "SECRET_MODE_9C", Amount: f64(amount), Currency: "RUB"}
	})
	policy := radiusPolicy(500, 0)
	policy.ObjectiveProfile = domain.ProfileBalanced
	result, err := w.svc.SearchNextLoad(context.Background(), w.actor(), SearchCommand{CapacityID: cap.ID, Policy: policy})
	if err != nil {
		t.Fatal(err)
	}
	body := string(result.Body)
	if strings.Contains(body, "SECRET_MODE_9C") || strings.Contains(body, strconv.FormatFloat(amount, 'f', -1, 64)) || strings.Contains(body, `"commercial"`) {
		t.Fatalf("BALANCED leaked %s", body)
	}
	var doc SearchResponse
	if err := json.Unmarshal(result.Body, &doc); err != nil {
		t.Fatal(err)
	}
	if len(doc.Candidates) != 1 || doc.Candidates[0].LoadOpportunityID != load.ID || doc.Candidates[0].Score == nil {
		t.Fatalf("BALANCED dropped the anonymized load %+v", doc.Candidates)
	}
	return *doc.Candidates[0].Score
}

func mixedRevenueScore(t *testing.T, hidden float64) mixedRevenue {
	t.Helper()
	w := newWorld(t)
	w.store.ReplaceScoreProfile(domain.ScoreProfile{
		ID: uuid.MustParse("00000000-0000-4000-8000-000000000206"), Code: domain.ProfileBalanced,
		Scope: domain.ProfileScopeSystem, Version: 1, Status: domain.ProfileStatusActive,
		AlgorithmVersion: domain.ScoringAlgorithmVersion,
		Components: []domain.ScoreProfileComponent{
			{Code: domain.ComponentDeadheadEfficiency, WeightBps: 5000, Required: true, Ordinal: 1},
			{Code: domain.ComponentRevenue, WeightBps: 5000, Required: false, Ordinal: 2},
		},
	})
	cap := w.capacity(domain.CapacityAvailable, domain.SourceManual, 0, 0)
	w.routes.defaultM = 10000
	w.routes.defaultSec = 600
	pickup := place(0, kmDeg(2), "Area")
	delivery := place(0, 1, "Far")
	identified := w.load(domain.VisMarketplace, pickup, delivery, nil)
	w.reviseLoad(identified, func(load *domain.LoadOpportunity) {
		load.Commercial = domain.Commercial{Mode: "PUBLISHED", Amount: f64(250), Currency: "RUB"}
	})
	hiddenLoad := w.load(domain.VisAnonymized, pickup, delivery, nil)
	w.reviseLoad(hiddenLoad, func(load *domain.LoadOpportunity) {
		load.Commercial = domain.Commercial{Mode: "SECRET_MODE_9C", Amount: f64(hidden), Currency: "RUB"}
	})
	policy := radiusPolicy(500, 0)
	policy.ObjectiveProfile = domain.ProfileBalanced
	policy.RankingCurrency = "RUB"
	result, err := w.svc.SearchNextLoad(context.Background(), w.actor(), SearchCommand{CapacityID: cap.ID, Policy: policy})
	if err != nil {
		t.Fatal(err)
	}
	body := string(result.Body)
	secret := strconv.FormatFloat(hidden, 'f', -1, 64)
	if strings.Contains(body, secret) || strings.Contains(body, "SECRET_MODE_9C") {
		t.Fatalf("mixed profile leaked %s", body)
	}
	var doc SearchResponse
	if err := json.Unmarshal(result.Body, &doc); err != nil {
		t.Fatal(err)
	}
	anon := candidateByID(t, doc, hiddenLoad.ID)
	known := candidateByID(t, doc, identified.ID)
	if anon.Score == nil || anon.ScoreEvidenceBps == nil || known.Score == nil || known.Load.Commercial == nil || known.Load.Commercial.Amount == nil {
		t.Fatalf("mixed profile dropped a load anon=%+v known=%+v", anon, known)
	}
	anonRevenue := component(t, anon, domain.ComponentRevenue)
	knownRevenue := component(t, known, domain.ComponentRevenue)
	return mixedRevenue{
		anonScore: *anon.Score, anonEvidence: *anon.ScoreEvidenceBps, identifiedScore: *known.Score,
		identifiedRaw: *known.Load.Commercial.Amount, identifiedRawPoints: knownRevenue.NormalizedPoints,
		anonPoints: anonRevenue.NormalizedPoints,
	}
}
