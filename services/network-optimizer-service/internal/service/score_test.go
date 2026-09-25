package service

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/freight-platform/network-optimizer-service/internal/domain"
	apperrors "github.com/freight-platform/network-optimizer-service/internal/platform/errors"
	"github.com/freight-platform/network-optimizer-service/internal/repository"
	"github.com/freight-platform/network-optimizer-service/internal/routing"
)

func TestBNO258_PROFILE_WEIGHTS_FROM_STORE(t *testing.T) {
	w := newWorld(t)
	profile, err := w.store.ActiveScoreProfile(context.Background(), domain.ProfileBalanced)
	if err != nil || profile.Components[0].WeightBps != 3500 {
		t.Fatalf("store profile %+v %v", profile, err)
	}
	cap := w.capacity(domain.CapacityAvailable, domain.SourceManual, 0, 0)
	near := w.load(domain.VisMarketplace, place(0, kmDeg(2), "Near"), place(0, 1, "D"), nil)
	far := w.load(domain.VisMarketplace, place(0, kmDeg(4), "Far"), place(0, 1, "D"), nil)
	w.routes.roads[roadKey(routing.Point{}, routing.Point{Longitude: kmDeg(2)})] = roadCell{m: 20000, sec: 600}
	w.routes.roads[roadKey(routing.Point{}, routing.Point{Longitude: kmDeg(4)})] = roadCell{m: 80000, sec: 600}
	w.store.ReplaceScoreProfile(domain.ScoreProfile{
		ID: profile.ID, Code: domain.ProfileMinDeadhead, Scope: domain.ProfileScopeSystem, Version: 1,
		Status: domain.ProfileStatusActive, AlgorithmVersion: domain.ScoringAlgorithmVersion,
		Components: []domain.ScoreProfileComponent{
			{Code: domain.ComponentDeadheadEfficiency, WeightBps: 5000, Required: true, Ordinal: 1},
			{Code: domain.ComponentNetworkValue, WeightBps: 5000, Required: false, Ordinal: 2},
		},
	})
	doc := w.search(w.actor(), cap.ID, radiusPolicy(500, 0))
	byID := map[uuid.UUID]SearchCandidateView{}
	for _, candidate := range doc.Candidates {
		byID[candidate.LoadOpportunityID] = candidate
	}
	if byID[near.ID].Score == nil || *byID[near.ID].Score != 7500 || *byID[far.ID].Score != 2500 {
		t.Fatalf("weights were not read from the store %+v", doc.Candidates)
	}
}

func TestBNO261_UNKNOWN_PROFILE_REJECTED(t *testing.T) {
	w := newWorld(t)
	cap := w.capacity(domain.CapacityAvailable, domain.SourceManual, 0, 0)
	policy := radiusPolicy(100, 0)
	policy.ObjectiveProfile = "CITY_SWITCH"
	_, err := w.svc.SearchNextLoad(context.Background(), w.actor(), SearchCommand{CapacityID: cap.ID, Policy: policy})
	if err == nil || reasonOf(err) == "OBJECTIVE_PROFILE_NOT_EXECUTABLE" {
		t.Fatalf("%v", err)
	}
}

func TestBNO262_MAX_CONTRIBUTION_RESERVED_FAILS_CLOSED(t *testing.T) {
	w := newWorld(t)
	cap := w.capacity(domain.CapacityAvailable, domain.SourceManual, 0, 0)
	policy := radiusPolicy(100, 0)
	policy.ObjectiveProfile = domain.ProfileMaxContribution
	_, err := w.svc.SearchNextLoad(context.Background(), w.actor(), SearchCommand{CapacityID: cap.ID, Policy: policy})
	if reasonOf(err) != "OBJECTIVE_PROFILE_NOT_EXECUTABLE" {
		t.Fatalf("%v", err)
	}
	var app *apperrors.AppError
	if !errors.As(err, &app) || app.Details["detail"] != "PLANNING_COST_PROVIDER_NOT_IMPLEMENTED" {
		t.Fatalf("%+v", app)
	}
}

func TestBNO263_MIN_DEADHEAD_ORDER(t *testing.T) {
	w := newWorld(t)
	cap := w.capacity(domain.CapacityAvailable, domain.SourceManual, 0, 0)
	ids := map[int]uuid.UUID{}
	for _, km := range []int{80, 20, 50} {
		pickup := routing.Point{Longitude: kmDeg(float64(km))}
		load := w.load(domain.VisMarketplace, pointPlace(pickup, "P"), place(0, 1, "D"), nil)
		w.routes.roads[roadKey(routing.Point{}, pickup)] = roadCell{m: km * 1000, sec: 600}
		ids[km] = load.ID
	}
	doc := w.search(w.actor(), cap.ID, radiusPolicy(500, 0))
	got := candidateIDs(doc)
	if len(got) != 3 || got[0] != ids[20] || got[1] != ids[50] || got[2] != ids[80] {
		t.Fatalf("%v", got)
	}
	if w.routes.sawHaversineSubstitute {
		t.Fatal("haversine used as road")
	}
	if doc.Candidates[0].Rank == nil || *doc.Candidates[0].Rank != 1 || *doc.Candidates[2].Rank != 3 {
		t.Fatalf("%+v", doc.Candidates)
	}
}

func TestBNO264_MAX_CAPACITY_UTILIZATION_ORDER(t *testing.T) {
	w := newWorld(t)
	cap := w.capacity(domain.CapacityAvailable, domain.SourceManual, 0, 0)
	w.reviseCapacity(cap, func(cap *domain.Capacity) {
		cap.PayloadRemainingKg = f64(10000)
		cap.VolumeRemainingM3 = f64(100)
	})
	w.routes.defaultM = 10000
	w.routes.defaultSec = 600
	high := w.load(domain.VisMarketplace, place(0, kmDeg(2), "A"), place(0, 1, "D"), nil)
	low := w.load(domain.VisMarketplace, place(0, kmDeg(3), "B"), place(0, 1, "D"), nil)
	w.reviseLoad(high, func(load *domain.LoadOpportunity) {
		load.WeightKg = f64(6000)
		load.VolumeM3 = f64(90)
	})
	w.reviseLoad(low, func(load *domain.LoadOpportunity) {
		load.WeightKg = f64(8000)
		load.VolumeM3 = f64(70)
	})
	policy := radiusPolicy(500, 0)
	policy.ObjectiveProfile = domain.ProfileMaxCapacityUtilization
	doc := w.search(w.actor(), cap.ID, policy)
	if len(doc.Candidates) != 2 || doc.Candidates[0].LoadOpportunityID != high.ID || doc.Candidates[1].LoadOpportunityID != low.ID {
		t.Fatalf("%+v", doc.Candidates)
	}
	if *component(t, doc.Candidates[0], domain.ComponentCapacityUtilization).RawValue < 0.89 {
		t.Fatalf("limiting dimension %+v", doc.Candidates[0].ScoreComponents)
	}
}

func TestBNO265_UTILIZATION_UNKNOWN_UNRANKED(t *testing.T) {
	w := newWorld(t)
	cap := w.capacity(domain.CapacityAvailable, domain.SourceManual, 0, 0)
	load := w.load(domain.VisMarketplace, place(0, kmDeg(2), "A"), place(0, 1, "D"), nil)
	w.routes.defaultM = 10000
	w.routes.defaultSec = 600
	policy := radiusPolicy(500, 0)
	policy.ObjectiveProfile = domain.ProfileMaxCapacityUtilization
	doc := w.search(w.actor(), cap.ID, policy)
	if doc.EligibleCandidateCount != 1 || doc.RankedCandidateCount != 0 || doc.UnrankedEligibleCount != 1 {
		t.Fatalf("%+v", doc)
	}
	if doc.Candidates[0].LoadOpportunityID != load.ID || doc.Candidates[0].Score != nil || doc.Candidates[0].Rank != nil {
		t.Fatalf("%+v", doc.Candidates[0])
	}
	if doc.UnrankedCountsByReason[domain.ReasonUtilizationUnknown] != 1 {
		t.Fatalf("%+v", doc.UnrankedCountsByReason)
	}
}

func TestBNO266_RETURN_HOME_ROAD_ORDER(t *testing.T) {
	doc, ids := returnHomeSearch(t, false)
	if doc.Candidates[0].LoadOpportunityID != ids[10] || doc.Candidates[1].LoadOpportunityID != ids[40] || doc.Candidates[2].LoadOpportunityID != ids[70] {
		t.Fatalf("%v", candidateIDs(doc))
	}
}

func TestBNO267_RETURN_HOME_TARGET_REQUIRED(t *testing.T) {
	w := newWorld(t)
	cap := w.capacity(domain.CapacityAvailable, domain.SourceManual, 0, 0)
	policy := radiusPolicy(100, 0)
	policy.ObjectiveProfile = domain.ProfileReturnHome
	_, err := w.svc.SearchNextLoad(context.Background(), w.actor(), SearchCommand{CapacityID: cap.ID, Policy: policy})
	if reasonOf(err) != "TARGET_LOCATION_REQUIRED_FOR_OBJECTIVE" {
		t.Fatalf("%v", err)
	}
}

func TestBNO268_RETURN_HOME_RADIUS_WITH_TARGET(t *testing.T) {
	doc, ids := returnHomeSearch(t, true)
	if doc.SearchMode != domain.SearchRadius || doc.Candidates[0].LoadOpportunityID != ids[10] {
		t.Fatalf("mode %s ids %v", doc.SearchMode, candidateIDs(doc))
	}
}

func TestBNO269_MAX_REVENUE_ORDER(t *testing.T) {
	doc, ids := revenueSearch(t)
	if doc.Candidates[0].LoadOpportunityID != ids[300] || doc.Candidates[1].LoadOpportunityID != ids[100] {
		t.Fatalf("%v", candidateIDs(doc))
	}
}

func TestBNO270_MAX_REVENUE_CURRENCY_REQUIRED(t *testing.T) {
	w := newWorld(t)
	cap := w.capacity(domain.CapacityAvailable, domain.SourceManual, 0, 0)
	policy := radiusPolicy(100, 0)
	policy.ObjectiveProfile = domain.ProfileMaxRevenue
	_, err := w.svc.SearchNextLoad(context.Background(), w.actor(), SearchCommand{CapacityID: cap.ID, Policy: policy})
	if reasonOf(err) != "RANKING_CURRENCY_REQUIRED" {
		t.Fatalf("%v", err)
	}
}

func TestBNO271_UNPRICED_REVENUE_UNRANKED(t *testing.T) {
	doc, ids := revenueSearch(t)
	var unpriced SearchCandidateView
	for _, candidate := range doc.Candidates {
		if candidate.LoadOpportunityID == ids[0] {
			unpriced = candidate
		}
	}
	if unpriced.Score != nil || !containsString(join(unpriced.UnrankedReasonCodes), domain.ReasonCommercialUnavailable) {
		t.Fatalf("%+v", unpriced)
	}
}

func TestBNO272_DIFFERENT_CURRENCY_NOT_COMPARED(t *testing.T) {
	w := newWorld(t)
	cap := w.capacity(domain.CapacityAvailable, domain.SourceManual, 0, 0)
	w.routes.defaultM = 10000
	w.routes.defaultSec = 600
	rub := pricedLoad(w, 100, "RUB", kmDeg(2))
	usd := pricedLoad(w, 100000, "USD", kmDeg(3))
	policy := radiusPolicy(500, 0)
	policy.ObjectiveProfile = domain.ProfileMaxRevenue
	policy.RankingCurrency = "RUB"
	doc := w.search(w.actor(), cap.ID, policy)
	for _, candidate := range doc.Candidates {
		if candidate.LoadOpportunityID == usd.ID {
			if candidate.ScoreStatus != domain.ScoreUnranked || !hasReason(candidate, domain.ReasonCurrencyNotComparable) {
				t.Fatalf("%+v", candidate)
			}
		}
		if candidate.LoadOpportunityID == rub.ID && candidate.ScoreStatus != domain.ScoreRanked {
			t.Fatalf("rub %+v", candidate)
		}
	}
}

func TestBNO273_MIN_RISK_PICKUP_SLACK_ORDER(t *testing.T) {
	w := newWorld(t)
	cap := w.capacity(domain.CapacityAvailable, domain.SourceManual, 0, 0)
	w.routes.defaultM = 10000
	w.routes.defaultSec = 1800
	arrivalSlack := func(hours int) uuid.UUID {
		end := w.at.Add(30*time.Minute + time.Duration(hours)*time.Hour)
		start := w.at
		load := w.load(domain.VisMarketplace, place(0, kmDeg(float64(hours)+1), "P"), place(0, 1, "D"), window(start, end))
		return load.ID
	}
	small := arrivalSlack(1)
	large := arrivalSlack(4)
	policy := radiusPolicy(500, 0)
	policy.ObjectiveProfile = domain.ProfileMinRisk
	doc := w.search(w.actor(), cap.ID, policy)
	if doc.Candidates[0].LoadOpportunityID != large || doc.Candidates[1].LoadOpportunityID != small {
		t.Fatalf("%v", candidateIDs(doc))
	}
}

func TestBNO274_PREDICTION_CONFIDENCE_USED(t *testing.T) {
	w := newWorld(t)
	cap := w.capacity(domain.CapacityAvailable, domain.SourceCurrentShipmentPrediction, 0, 0)
	if err := w.store.Within(context.Background(), func(tx repository.Tx) error {
		return tx.InsertPrediction(context.Background(), domain.PredictedCapacity{
			ID: uuid.New(), CapacityID: cap.ID, OwnerTenantID: w.carrier, IsCurrent: true,
			PredictedAvailableAt: w.at, AvailabilityWindowStart: w.at, AvailabilityWindowEnd: w.at.Add(8 * time.Hour),
			Confidence: 0.42, UncertaintySeconds: 900,
		})
	}); err != nil {
		t.Fatal(err)
	}
	w.routes.defaultM = 10000
	w.routes.defaultSec = 600
	end := w.at.Add(4 * time.Hour)
	w.load(domain.VisMarketplace, place(0, kmDeg(2), "P"), place(0, 1, "D"), window(w.at, end))
	policy := radiusPolicy(500, 0)
	policy.ObjectiveProfile = domain.ProfileMinRisk
	doc := w.search(w.actor(), cap.ID, policy)
	confidence := component(t, doc.Candidates[0], domain.ComponentPredictionConfidence)
	if confidence.Status != domain.ComponentAvailable || confidence.RawValue == nil || *confidence.RawValue != 0.42 {
		t.Fatalf("%+v", confidence)
	}
}

func TestBNO275_ETA_UNCERTAINTY_USED(t *testing.T) {
	w := newWorld(t)
	cap := w.capacity(domain.CapacityAvailable, domain.SourceCurrentShipmentPrediction, 0, 0)
	if err := w.store.Within(context.Background(), func(tx repository.Tx) error {
		return tx.InsertPrediction(context.Background(), domain.PredictedCapacity{
			ID: uuid.New(), CapacityID: cap.ID, OwnerTenantID: w.carrier, IsCurrent: true,
			PredictedAvailableAt: w.at, AvailabilityWindowStart: w.at, AvailabilityWindowEnd: w.at.Add(8 * time.Hour),
			Confidence: 0.5, UncertaintySeconds: 1800,
		})
	}); err != nil {
		t.Fatal(err)
	}
	w.routes.defaultM = 10000
	w.routes.defaultSec = 600
	w.load(domain.VisMarketplace, place(0, kmDeg(2), "P"), place(0, 1, "D"), window(w.at, w.at.Add(4*time.Hour)))
	policy := radiusPolicy(500, 0)
	policy.ObjectiveProfile = domain.ProfileMinRisk
	doc := w.search(w.actor(), cap.ID, policy)
	uncertainty := component(t, doc.Candidates[0], domain.ComponentETAUncertainty)
	if uncertainty.Status != domain.ComponentAvailable || uncertainty.RawValue == nil || *uncertainty.RawValue != 1800 {
		t.Fatalf("%+v", uncertainty)
	}
}

func TestBNO276_MANUAL_CAPACITY_DOES_NOT_INVENT_CONFIDENCE(t *testing.T) {
	w := newWorld(t)
	cap := w.capacity(domain.CapacityAvailable, domain.SourceManual, 0, 0)
	w.routes.defaultM = 10000
	w.routes.defaultSec = 600
	end := w.at.Add(3 * time.Hour)
	w.load(domain.VisMarketplace, place(0, kmDeg(2), "P"), place(0, 1, "D"), window(w.at, end))
	policy := radiusPolicy(500, 0)
	policy.ObjectiveProfile = domain.ProfileMinRisk
	doc := w.search(w.actor(), cap.ID, policy)
	item := component(t, doc.Candidates[0], domain.ComponentPredictionConfidence)
	if item.Status != domain.ComponentUnavailable || item.RawValue != nil || *doc.Candidates[0].ScoreEvidenceBps != 5000 {
		t.Fatalf("%+v evidence %v", item, doc.Candidates[0].ScoreEvidenceBps)
	}
}

func TestBNO277_KNOWN_ZERO_WAITING_DISTINCT_FROM_UNKNOWN(t *testing.T) {
	w := newWorld(t)
	cap := w.capacity(domain.CapacityAvailable, domain.SourceManual, 0, 0)
	w.routes.defaultM = 10000
	w.routes.defaultSec = 600
	known := w.load(domain.VisMarketplace, place(0, kmDeg(2), "Known"), place(0, 1, "D"), window(w.at.Add(-time.Hour), w.at.Add(4*time.Hour)))
	unknown := w.load(domain.VisMarketplace, place(0, kmDeg(3), "Unknown"), place(0, 1, "D"), nil)
	policy := radiusPolicy(500, 0)
	policy.ObjectiveProfile = domain.ProfileBalanced
	doc := w.search(w.actor(), cap.ID, policy)
	var knownView, unknownView SearchCandidateView
	for _, candidate := range doc.Candidates {
		if candidate.LoadOpportunityID == known.ID {
			knownView = candidate
		}
		if candidate.LoadOpportunityID == unknown.ID {
			unknownView = candidate
		}
	}
	knownWaiting := component(t, knownView, domain.ComponentWaitingEfficiency)
	unknownWaiting := component(t, unknownView, domain.ComponentWaitingEfficiency)
	if knownWaiting.Status != domain.ComponentAvailable || knownWaiting.RawValue == nil || *knownWaiting.RawValue != 0 {
		t.Fatalf("known %+v", knownWaiting)
	}
	if unknownWaiting.Status != domain.ComponentUnavailable {
		t.Fatalf("unknown %+v", unknownWaiting)
	}
	if knownView.WaitingMinutes == nil || *knownView.WaitingMinutes != 0 {
		t.Fatalf("public zero %+v", knownView.WaitingMinutes)
	}
}

func TestBNO278_WAITING_LOWER_IS_BETTER(t *testing.T) {
	w := newWorld(t)
	cap := w.capacity(domain.CapacityAvailable, domain.SourceManual, 0, 0)
	w.routes.defaultM = 10000
	w.routes.defaultSec = 3600
	zero := w.load(domain.VisMarketplace, place(0, kmDeg(2), "Zero"), place(0, 1, "D"), window(w.at, w.at.Add(6*time.Hour)))
	late := w.load(domain.VisMarketplace, place(0, kmDeg(3), "Late"), place(0, 1, "D"), window(w.at.Add(2*time.Hour), w.at.Add(8*time.Hour)))
	policy := radiusPolicy(500, 0)
	policy.ObjectiveProfile = domain.ProfileBalanced
	doc := w.search(w.actor(), cap.ID, policy)
	if doc.Candidates[0].LoadOpportunityID != zero.ID || doc.Candidates[1].LoadOpportunityID != late.ID {
		t.Fatalf("%v", candidateIDs(doc))
	}
}

func TestBNO288_REJECTED_CANDIDATE_NEVER_SCORED(t *testing.T) {
	doc, rows := rejectedHighRevenue(t)
	if doc.EligibleCandidateCount != 1 || len(rows) != 2 {
		t.Fatalf("eligible %d rows %d", doc.EligibleCandidateCount, len(rows))
	}
	for _, row := range rows {
		if row.Eligibility == "REJECTED" && (row.ScoreStatus != domain.ScoreNotApplicable || row.ScoreTotal != nil || row.Rank != nil) {
			t.Fatalf("%+v", row)
		}
	}
}

func TestBNO289_HIGH_SCORE_CANNOT_OVERRIDE_HARD_REJECT(t *testing.T) {
	doc, _ := rejectedHighRevenue(t)
	if doc.RankedCandidateCount != 1 || doc.Candidates[0].Score == nil {
		t.Fatalf("%+v", doc)
	}
	for _, candidate := range doc.Candidates {
		if candidate.Load.Commercial != nil && candidate.Load.Commercial.Amount != nil && *candidate.Load.Commercial.Amount > 1000 {
			t.Fatal("rejected high revenue was returned")
		}
	}
}

func TestBNO290_TOP_N_AFTER_SCORE(t *testing.T) {
	doc := revenueLimited(t, intPtr(1))
	if doc.ReturnedCandidateCount != 1 || doc.RankedCandidateCount != 2 || doc.EligibleCandidateCount != 3 || *doc.Candidates[0].Rank != 1 {
		t.Fatalf("%+v", doc)
	}
}

func TestBNO291_CANDIDATE_LIMIT_ZERO(t *testing.T) {
	doc := revenueLimited(t, intPtr(0))
	if len(doc.Candidates) != 0 || doc.ReturnedCandidateCount != 0 || doc.RankedCandidateCount != 2 || doc.EligibleCandidateCount != 3 {
		t.Fatalf("%+v", doc)
	}
}

func TestBNO292_NO_LIMIT_RETURNS_RANKED_THEN_UNRANKED(t *testing.T) {
	doc, ids := revenueSearch(t)
	if len(doc.Candidates) != 3 || doc.Candidates[2].LoadOpportunityID != ids[0] || doc.Candidates[2].ScoreStatus != domain.ScoreUnranked {
		t.Fatalf("%+v", doc.Candidates)
	}
	if doc.Candidates[0].ScoreStatus != domain.ScoreRanked || doc.Candidates[1].ScoreStatus != domain.ScoreRanked {
		t.Fatal("ranked prefix missing")
	}
}

func TestBNO293_TOP_N_DOES_NOT_FILL_WITH_UNRANKED(t *testing.T) {
	doc := revenueLimited(t, intPtr(3))
	if len(doc.Candidates) != 2 {
		t.Fatalf("filled with unranked %+v", doc.Candidates)
	}
	for _, candidate := range doc.Candidates {
		if candidate.ScoreStatus != domain.ScoreRanked {
			t.Fatalf("%+v", candidate)
		}
	}
}

func TestBNO294_SCORE_TIE_EVIDENCE_BREAK(t *testing.T) {
	w := newWorld(t)
	cap := w.capacity(domain.CapacityAvailable, domain.SourceManual, 0, 0)
	w.reviseCapacity(cap, func(cap *domain.Capacity) {
		cap.PayloadRemainingKg = f64(10000)
		cap.VolumeRemainingM3 = f64(100)
	})
	w.routes.defaultM = 20000
	w.routes.defaultSec = 600
	rich := w.load(domain.VisMarketplace, place(0, kmDeg(2), "Rich"), place(0, 1, "D"), window(w.at, w.at.Add(4*time.Hour)))
	thin := w.load(domain.VisMarketplace, place(0, kmDeg(3), "Thin"), place(0, 1, "D"), nil)
	w.reviseLoad(rich, func(load *domain.LoadOpportunity) {
		load.WeightKg = f64(5000)
		load.VolumeM3 = f64(50)
	})
	w.reviseLoad(thin, func(load *domain.LoadOpportunity) {
		load.WeightKg = f64(5000)
		load.VolumeM3 = f64(50)
	})
	policy := radiusPolicy(500, 0)
	policy.ObjectiveProfile = domain.ProfileBalanced
	doc := w.search(w.actor(), cap.ID, policy)
	if doc.Candidates[0].LoadOpportunityID != rich.ID || *doc.Candidates[0].Score != *doc.Candidates[1].Score {
		t.Fatalf("%+v", doc.Candidates)
	}
	if *doc.Candidates[0].ScoreEvidenceBps <= *doc.Candidates[1].ScoreEvidenceBps || doc.Candidates[1].LoadOpportunityID != thin.ID {
		t.Fatalf("evidence %+v", doc.Candidates)
	}
}

func TestBNO295_SCORE_AND_EVIDENCE_TIE_DEADHEAD_BREAK(t *testing.T) {
	w := newWorld(t)
	cap := w.capacity(domain.CapacityAvailable, domain.SourceManual, 0, 0)
	w.reviseCapacity(cap, func(cap *domain.Capacity) {
		cap.PayloadRemainingKg = f64(10000)
		cap.VolumeRemainingM3 = f64(100)
	})
	near := w.load(domain.VisMarketplace, place(0, kmDeg(8), "Near"), place(0, 1, "D"), nil)
	far := w.load(domain.VisMarketplace, place(0, kmDeg(2), "Far"), place(0, 1, "D"), nil)
	w.reviseLoad(near, func(load *domain.LoadOpportunity) {
		load.WeightKg = f64(5000)
		load.VolumeM3 = f64(50)
	})
	w.reviseLoad(far, func(load *domain.LoadOpportunity) {
		load.WeightKg = f64(5000)
		load.VolumeM3 = f64(50)
	})
	w.routes.roads[roadKey(routing.Point{}, routing.Point{Longitude: kmDeg(8)})] = roadCell{m: 20000, sec: 600}
	w.routes.roads[roadKey(routing.Point{}, routing.Point{Longitude: kmDeg(2)})] = roadCell{m: 80000, sec: 600}
	policy := radiusPolicy(500, 0)
	policy.ObjectiveProfile = domain.ProfileMaxCapacityUtilization
	doc := w.search(w.actor(), cap.ID, policy)
	if *doc.Candidates[0].Score != *doc.Candidates[1].Score || doc.Candidates[0].LoadOpportunityID != near.ID {
		t.Fatalf("%+v", doc.Candidates)
	}
}

func TestBNO296_FULL_TIE_UUID_BREAK(t *testing.T) {
	w := newWorld(t)
	cap := w.capacity(domain.CapacityAvailable, domain.SourceManual, 0, 0)
	w.routes.defaultM = 15000
	w.routes.defaultSec = 600
	for i := 0; i < 4; i++ {
		w.load(domain.VisMarketplace, place(0, kmDeg(float64(i+1)), "P"), place(0, 1, "D"), nil)
	}
	first := w.search(w.actor(), cap.ID, radiusPolicy(500, 0))
	second := w.search(w.actor(), cap.ID, radiusPolicy(500, 0))
	third := w.search(w.actor(), cap.ID, radiusPolicy(500, 0))
	if candidateKey(first) != candidateKey(second) || candidateKey(second) != candidateKey(third) || !sortIsIDOrder(first.Candidates) {
		t.Fatalf("%s", candidateKey(first))
	}
}

func TestBNO297_SCORE_PERSISTED(t *testing.T) {
	w := newWorld(t)
	cap := w.capacity(domain.CapacityAvailable, domain.SourceManual, 0, 0)
	w.load(domain.VisMarketplace, place(0, kmDeg(2), "A"), place(0, 1, "D"), nil)
	w.routes.defaultM = 10000
	w.routes.defaultSec = 600
	doc := w.search(w.actor(), cap.ID, radiusPolicy(500, 0))
	_, rows, err := w.store.GetSearch(context.Background(), w.carrier, doc.SearchID)
	if err != nil || rows[0].ScoreTotal == nil || *rows[0].ScoreTotal != *doc.Candidates[0].Score || rows[0].ScoreStatus != domain.ScoreRanked {
		t.Fatalf("%+v %v", rows, err)
	}
}

func TestBNO298_PROFILE_VERSION_PERSISTED(t *testing.T) {
	w := newWorld(t)
	cap := w.capacity(domain.CapacityAvailable, domain.SourceManual, 0, 0)
	w.load(domain.VisMarketplace, place(0, kmDeg(2), "A"), place(0, 1, "D"), nil)
	w.routes.defaultM = 10000
	w.routes.defaultSec = 600
	doc := w.search(w.actor(), cap.ID, radiusPolicy(500, 0))
	run, _, err := w.store.GetSearch(context.Background(), w.carrier, doc.SearchID)
	if err != nil || run.ScoreProfileCode != domain.ProfileMinDeadhead || run.ScoreProfileVersion != 1 || run.ScoringAlgorithmVersion != domain.ScoringAlgorithmVersion {
		t.Fatalf("%+v %v", run, err)
	}
}

func TestBNO299_SCORE_COMPONENTS_PERSISTED(t *testing.T) {
	w := newWorld(t)
	cap := w.capacity(domain.CapacityAvailable, domain.SourceManual, 0, 0)
	w.load(domain.VisMarketplace, place(0, kmDeg(2), "A"), place(0, 1, "D"), nil)
	w.routes.defaultM = 10000
	w.routes.defaultSec = 600
	doc := w.search(w.actor(), cap.ID, radiusPolicy(500, 0))
	_, rows, err := w.store.GetSearch(context.Background(), w.carrier, doc.SearchID)
	if err != nil || !containsString(string(rows[0].ScoreComponents), domain.ComponentDeadheadEfficiency) {
		t.Fatalf("%s %v", rows[0].ScoreComponents, err)
	}
}

func TestBNO304_ANONYMIZED_SCORE_ALLOWED(t *testing.T) {
	doc := anonymizedScore(t)
	if doc.Candidates[0].Score == nil || doc.Candidates[0].ScoreStatus != domain.ScoreRanked {
		t.Fatalf("%+v", doc.Candidates[0])
	}
}

func TestBNO305_ANONYMIZED_EXACT_DEADHEAD_NOT_EXPOSED(t *testing.T) {
	doc := anonymizedScore(t)
	raw, _ := json.Marshal(doc.Candidates[0].ScoreComponents)
	if doc.Candidates[0].RoadDeadheadKm != nil || containsString(string(raw), "64") {
		t.Fatalf("%s", raw)
	}
}

func TestBNO306_ANONYMIZED_TARGET_DISTANCE_NOT_EXPOSED(t *testing.T) {
	w := newWorld(t)
	targetID := uuid.New()
	w.dir.snaps[targetID] = domain.LocationSnapshot{ID: targetID, Latitude: f64(0), Longitude: f64(10), Status: "ACTIVE"}
	cap := w.capacity(domain.CapacityAvailable, domain.SourceManual, 0, 0)
	pickup := place(0, kmDeg(2), "Secret")
	delivery := routing.Point{Longitude: 1}
	load := w.load(domain.VisAnonymized, pickup, pointPlace(delivery, "Hidden"), nil)
	w.routes.defaultM = 10000
	w.routes.defaultSec = 600
	w.routes.roads[roadKey(delivery, routing.Point{Longitude: 10})] = roadCell{m: 123000, sec: 1}
	policy := radiusPolicy(500, 0)
	policy.ObjectiveProfile = domain.ProfileReturnHome
	policy.TargetLocationID = &targetID
	doc := w.search(w.actor(), cap.ID, policy)
	if doc.Candidates[0].LoadOpportunityID != load.ID || doc.Candidates[0].Score == nil {
		t.Fatalf("%+v", doc.Candidates)
	}
	raw, _ := json.Marshal(doc.Candidates[0])
	if containsString(string(raw), "123") || containsString(string(raw), "location_id") {
		t.Fatalf("%s", raw)
	}
}

func TestBNO307_SCORE_EXPLANATION_NO_EXACT_ANON_GEO(t *testing.T) {
	doc := anonymizedScore(t)
	for _, component := range doc.Candidates[0].ScoreComponents {
		if containsString(component.Explanation, "64") || containsString(component.Explanation, "latitude") {
			t.Fatalf("%+v", component)
		}
	}
}

func TestBNO308_REJECTED_LOAD_IDENTITY_STILL_NOT_EXPOSED(t *testing.T) {
	w := newWorld(t)
	cap := w.capacity(domain.CapacityAvailable, domain.SourceManual, 0, 0)
	hidden := w.load(domain.VisMarketplace, place(0, kmDeg(400), "Far"), place(0, 1, "D"), nil)
	doc := w.search(w.actor(), cap.ID, radiusPolicy(20, 80))
	raw, _ := json.Marshal(doc)
	if containsString(string(raw), hidden.ID.String()) {
		t.Fatalf("%s", raw)
	}
}

func TestBNO309_RANKING_CURRENCY_PRECEDENCE(t *testing.T) {
	carrier := domain.NextLoadSearchPolicy{SearchMode: domain.SearchRadius, RadiusKm: f64(100), ObjectiveProfile: domain.ProfileMaxRevenue, RankingCurrency: "USD"}
	capacity := domain.NextLoadSearchPolicy{RankingCurrency: "EUR"}
	request := domain.NextLoadSearchPolicy{RankingCurrency: "RUB"}
	got := domain.EffectiveSearchPolicy(carrier, capacity, request)
	if got.RankingCurrency != "RUB" {
		t.Fatalf("%s", got.RankingCurrency)
	}
	fell := domain.EffectiveSearchPolicy(carrier, capacity, domain.NextLoadSearchPolicy{})
	if fell.RankingCurrency != "EUR" {
		t.Fatalf("capacity %s", fell.RankingCurrency)
	}
}

func TestBNO310_PROFILE_REQUEST_PRECEDENCE(t *testing.T) {
	w := newWorld(t)
	cap := w.capacity(domain.CapacityAvailable, domain.SourceManual, 0, 0)
	if err := w.store.UpsertCarrierPolicy(context.Background(), w.carrier, domain.NextLoadSearchPolicy{
		SearchMode: domain.SearchRadius, RadiusKm: f64(500), ObjectiveProfile: domain.ProfileMinDeadhead,
	}); err != nil {
		t.Fatal(err)
	}
	if err := w.store.UpsertCapacityPolicy(context.Background(), w.carrier, cap.ID, domain.NextLoadSearchPolicy{ObjectiveProfile: domain.ProfileBalanced}); err != nil {
		t.Fatal(err)
	}
	w.load(domain.VisMarketplace, place(0, kmDeg(2), "A"), place(0, 1, "D"), nil)
	w.routes.defaultM = 10000
	w.routes.defaultSec = 600
	inherited := w.search(w.actor(), cap.ID, domain.NextLoadSearchPolicy{})
	if inherited.Ranking.ObjectiveProfile != domain.ProfileBalanced {
		t.Fatalf("capacity profile %s", inherited.Ranking.ObjectiveProfile)
	}
	requested := w.search(w.actor(), cap.ID, domain.NextLoadSearchPolicy{ObjectiveProfile: domain.ProfileMinRisk})
	if requested.Ranking.ObjectiveProfile != domain.ProfileMinRisk {
		t.Fatalf("request profile %s", requested.Ranking.ObjectiveProfile)
	}
}

func TestBNO311_PROFILE_FINGERPRINT_IN_POLICY_AUDIT(t *testing.T) {
	w := newWorld(t)
	cap := w.capacity(domain.CapacityAvailable, domain.SourceManual, 0, 0)
	w.load(domain.VisMarketplace, place(0, kmDeg(2), "A"), place(0, 1, "D"), nil)
	w.routes.defaultM = 10000
	w.routes.defaultSec = 600
	doc := w.search(w.actor(), cap.ID, radiusPolicy(500, 0))
	run, rows, err := w.store.GetSearch(context.Background(), w.carrier, doc.SearchID)
	profile, _ := w.store.ActiveScoreProfile(context.Background(), domain.ProfileMinDeadhead)
	if err != nil || run.ScoreProfileFingerprint != profile.Fingerprint() || run.ScoreProfileFingerprint != doc.Ranking.ProfileFingerprint {
		t.Fatalf("run %+v", run)
	}
	if rows[0].PolicyFingerprint != doc.PolicyFingerprint || rows[0].ScoreFingerprint == nil {
		t.Fatal("candidate audit missing")
	}
}

func TestScorePool100CandidatesDeterministic(t *testing.T) {
	w := newWorld(t)
	cap := w.capacity(domain.CapacityAvailable, domain.SourceManual, 0, 0)
	for i := 0; i < 100; i++ {
		w.load(domain.VisMarketplace, place(0, float64(i+1)/1000, "L"), place(0, 1, "D"), nil)
	}
	w.routes.defaultM = 12000
	w.routes.defaultSec = 600
	first := w.search(w.actor(), cap.ID, radiusPolicy(5000, 0))
	calls := w.routes.calls
	second := w.search(w.actor(), cap.ID, radiusPolicy(5000, 0))
	if first.EligibleCandidateCount != 100 || candidateKey(first) != candidateKey(second) || !sortIsIDOrder(first.Candidates) {
		t.Fatalf("count %d", first.EligibleCandidateCount)
	}
	if calls != 4 || w.routes.maxDests > routing.SyncMatrixLimit {
		t.Fatalf("calls %d max dests %d", calls, w.routes.maxDests)
	}
}

func returnHomeSearch(t *testing.T, radius bool) (SearchResponse, map[int]uuid.UUID) {
	t.Helper()
	w := newWorld(t)
	targetID := uuid.New()
	target := routing.Point{Longitude: 10}
	w.dir.snaps[targetID] = domain.LocationSnapshot{ID: targetID, Latitude: f64(0), Longitude: f64(10), Status: "ACTIVE"}
	w.routes.line = [][]float64{{0, 0}, {10, 0}}
	w.routes.baselineM = 1000000
	w.routes.defaultM = 10000
	w.routes.defaultSec = 600
	w.routes.roads[roadKey(routing.Point{Longitude: kmDeg(2)}, target)] = roadCell{m: 500000, sec: 1}
	cap := w.capacity(domain.CapacityAvailable, domain.SourceManual, 0, 0)
	ids := map[int]uuid.UUID{}
	for _, km := range []int{70, 10, 40} {
		drop := routing.Point{Longitude: float64(km) / 10}
		load := w.load(domain.VisMarketplace, place(0, kmDeg(2), "P"), pointPlace(drop, "D"), nil)
		w.routes.roads[roadKey(drop, target)] = roadCell{m: km * 1000, sec: 1}
		ids[km] = load.ID
	}
	policy := domain.NextLoadSearchPolicy{ObjectiveProfile: domain.ProfileReturnHome, TargetLocationID: &targetID}
	if radius {
		policy.SearchMode = domain.SearchRadius
		policy.RadiusKm = f64(500)
	} else {
		policy.SearchMode = domain.SearchDirectionalCorridor
		policy.ForwardSearchKm = f64(500)
		policy.CorridorDeviationKm = f64(80)
		policy.MaxDeadheadKm = f64(80)
	}
	return w.search(w.actor(), cap.ID, policy), ids
}

func revenueSearch(t *testing.T) (SearchResponse, map[int]uuid.UUID) {
	t.Helper()
	w := newWorld(t)
	cap := w.capacity(domain.CapacityAvailable, domain.SourceManual, 0, 0)
	w.routes.defaultM = 10000
	w.routes.defaultSec = 600
	ids := map[int]uuid.UUID{}
	ids[100] = pricedLoad(w, 100, "RUB", kmDeg(2)).ID
	ids[300] = pricedLoad(w, 300, "RUB", kmDeg(3)).ID
	ids[0] = pricedLoad(w, 0, "", kmDeg(4)).ID
	policy := radiusPolicy(500, 0)
	policy.ObjectiveProfile = domain.ProfileMaxRevenue
	policy.RankingCurrency = "RUB"
	return w.search(w.actor(), cap.ID, policy), ids
}

func revenueLimited(t *testing.T, limit *int) SearchResponse {
	t.Helper()
	w := newWorld(t)
	cap := w.capacity(domain.CapacityAvailable, domain.SourceManual, 0, 0)
	w.routes.defaultM = 10000
	w.routes.defaultSec = 600
	pricedLoad(w, 100, "RUB", kmDeg(2))
	pricedLoad(w, 300, "RUB", kmDeg(3))
	pricedLoad(w, 0, "", kmDeg(4))
	policy := radiusPolicy(500, 0)
	policy.ObjectiveProfile = domain.ProfileMaxRevenue
	policy.RankingCurrency = "RUB"
	result, err := w.svc.SearchNextLoad(context.Background(), w.actor(), SearchCommand{CapacityID: cap.ID, Policy: policy, CandidateLimit: limit})
	if err != nil {
		t.Fatal(err)
	}
	var doc SearchResponse
	if err := json.Unmarshal(result.Body, &doc); err != nil {
		t.Fatal(err)
	}
	return doc
}

func rejectedHighRevenue(t *testing.T) (SearchResponse, []repository.StoredCandidate) {
	t.Helper()
	w := newWorld(t)
	cap := w.capacity(domain.CapacityAvailable, domain.SourceManual, 0, 0)
	near := pricedLoad(w, 10, "RUB", kmDeg(2))
	_ = near
	farPickup := routing.Point{Longitude: kmDeg(5)}
	far := w.load(domain.VisMarketplace, pointPlace(farPickup, "Far"), place(0, 1, "D"), nil)
	w.reviseLoad(far, func(load *domain.LoadOpportunity) {
		load.Commercial = domain.Commercial{Mode: "PUBLISHED", Amount: f64(9000), Currency: "RUB"}
	})
	w.routes.roads[roadKey(routing.Point{}, routing.Point{Longitude: kmDeg(2)})] = roadCell{m: 20000, sec: 600}
	w.routes.roads[roadKey(routing.Point{}, farPickup)] = roadCell{m: 200000, sec: 600}
	policy := radiusPolicy(100, 40)
	policy.ObjectiveProfile = domain.ProfileMaxRevenue
	policy.RankingCurrency = "RUB"
	doc := w.search(w.actor(), cap.ID, policy)
	_, rows, err := w.store.GetSearch(context.Background(), w.carrier, doc.SearchID)
	if err != nil {
		t.Fatal(err)
	}
	return doc, rows
}

func anonymizedScore(t *testing.T) SearchResponse {
	t.Helper()
	w := newWorld(t)
	cap := w.capacity(domain.CapacityAvailable, domain.SourceManual, 0, 0)
	pickup := place(0, kmDeg(2), "Secret yard")
	w.load(domain.VisAnonymized, pickup, place(0, 1, "Far"), nil)
	w.routes.roads[roadKey(routing.Point{}, routing.Point{Latitude: 0, Longitude: kmDeg(2)})] = roadCell{m: 64000, sec: 3600}
	return w.search(w.actor(), cap.ID, radiusPolicy(500, 0))
}

func pricedLoad(w *world, amount float64, currency string, lon float64) domain.LoadOpportunity {
	w.t.Helper()
	load := w.load(domain.VisMarketplace, place(0, lon, "P"), place(0, 1, "D"), nil)
	if amount == 0 && currency == "" {
		return load
	}
	w.reviseLoad(load, func(load *domain.LoadOpportunity) {
		load.Commercial = domain.Commercial{Mode: "PUBLISHED", Amount: f64(amount), Currency: currency}
	})
	return load
}

func (w *world) reviseLoad(load domain.LoadOpportunity, edit func(*domain.LoadOpportunity)) {
	w.t.Helper()
	edit(&load)
	load.Version++
	if err := w.store.Within(context.Background(), func(tx repository.Tx) error {
		return tx.UpdateLoad(context.Background(), load, load.Version-1)
	}); err != nil {
		w.t.Fatal(err)
	}
}

func (w *world) reviseCapacity(cap domain.Capacity, edit func(*domain.Capacity)) {
	w.t.Helper()
	edit(&cap)
	cap.Version++
	if err := w.store.Within(context.Background(), func(tx repository.Tx) error {
		return tx.UpdateCapacity(context.Background(), cap, cap.Version-1)
	}); err != nil {
		w.t.Fatal(err)
	}
}

func component(t *testing.T, view SearchCandidateView, code string) domain.ScoreComponentResult {
	t.Helper()
	for _, item := range view.ScoreComponents {
		if item.Code == code {
			return item
		}
	}
	t.Fatalf("missing %s", code)
	return domain.ScoreComponentResult{}
}

func reasonOf(err error) string {
	var app *apperrors.AppError
	if errors.As(err, &app) {
		if value, ok := app.Details["reason"].(string); ok {
			return value
		}
	}
	return ""
}

func hasReason(view SearchCandidateView, reason string) bool {
	for _, item := range view.UnrankedReasonCodes {
		if item == reason {
			return true
		}
	}
	return false
}

func join(values []string) string {
	out := ""
	for _, value := range values {
		out += value + ","
	}
	return out
}
