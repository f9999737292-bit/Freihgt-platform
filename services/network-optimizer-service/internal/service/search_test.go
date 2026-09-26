package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/freight-platform/network-optimizer-service/internal/domain"
	"github.com/freight-platform/network-optimizer-service/internal/locationclient"
	apperrors "github.com/freight-platform/network-optimizer-service/internal/platform/errors"
	"github.com/freight-platform/network-optimizer-service/internal/profilefixture"
	"github.com/freight-platform/network-optimizer-service/internal/repository"
	"github.com/freight-platform/network-optimizer-service/internal/routing"
)

func TestBNO206To240NextLoadSearch(t *testing.T) {
	t.Run("BNO206_SEARCH_OWN_CAPACITY_ONLY", func(t *testing.T) {
		w := newWorld(t)
		own := w.capacity(domain.CapacityAvailable, domain.SourceManual, 0, 0)
		otherTenant := uuid.New()
		if err := w.store.Within(context.Background(), func(tx repository.Tx) error {
			return tx.InsertCapacity(context.Background(), domain.Capacity{
				ID: uuid.New(), OwnerTenantID: otherTenant, Latitude: f64(1), Longitude: f64(1),
				AvailableFrom: w.at, AvailableUntil: w.at.Add(4 * time.Hour), Source: domain.SourceManual,
				VisibilityScope: domain.CapVisPrivate, Status: domain.CapacityAvailable, Version: 1,
			})
		}); err != nil {
			t.Fatal(err)
		}
		doc := w.search(w.actor(), own.ID, radiusPolicy(100, 80))
		if doc.CapacityID != own.ID || doc.SearchMode != domain.SearchRadius {
			t.Fatalf("%+v", doc)
		}
		_, err := w.svc.SearchNextLoad(context.Background(), serviceActor(otherTenant, nil), SearchCommand{CapacityID: own.ID, Policy: radiusPolicy(100, 80)})
		if !notFound(err) {
			t.Fatalf("other tenant %v", err)
		}
	})

	t.Run("BNO207_FOREIGN_CAPACITY_404", func(t *testing.T) {
		w := newWorld(t)
		foreign := uuid.New()
		if err := w.store.Within(context.Background(), func(tx repository.Tx) error {
			return tx.InsertCapacity(context.Background(), domain.Capacity{
				ID: foreign, OwnerTenantID: uuid.New(), Latitude: f64(0), Longitude: f64(0),
				AvailableFrom: w.at, AvailableUntil: w.at.Add(time.Hour), Source: domain.SourceManual,
				Status: domain.CapacityAvailable, VisibilityScope: domain.CapVisPrivate, Version: 1,
			})
		}); err != nil {
			t.Fatal(err)
		}
		_, err := w.svc.SearchNextLoad(context.Background(), w.actor(), SearchCommand{CapacityID: foreign, Policy: radiusPolicy(100, 80)})
		if !notFound(err) {
			t.Fatalf("%v", err)
		}
		_, missing := w.svc.SearchNextLoad(context.Background(), w.actor(), SearchCommand{CapacityID: uuid.New(), Policy: radiusPolicy(100, 80)})
		if !notFound(missing) {
			t.Fatalf("missing %v", missing)
		}
	})

	t.Run("BNO208_ONLY_VISIBLE_PUBLISHED_LOADS", func(t *testing.T) {
		w := newWorld(t)
		cap := w.capacity(domain.CapacityAvailable, domain.SourceManual, 0, 0)
		visible := w.load(domain.VisMarketplace, place(0, 0.02, "Visible"), place(0, 1, "Drop"), nil)
		w.load(domain.VisPrivate, place(0, 0.02, "Hidden"), place(0, 1, "Drop"), nil)
		w.load(domain.VisNetworkOnly, place(0, 0.02, "Internal"), place(0, 1, "Drop"), nil)
		w.load(domain.LoadDraft, place(0, 0.02, "Draft"), place(0, 1, "Drop"), nil)
		company := uuid.New()
		invited := w.loadOwned(w.shipper, domain.VisInvited, []uuid.UUID{company}, place(0, 0.03, "Invited"), place(0, 1, "Drop"), nil)
		w.loadOwned(w.shipper, domain.VisInvited, []uuid.UUID{uuid.New()}, place(0, 0.04, "Other"), place(0, 1, "Drop"), nil)
		w.routes.defaultM = 10000
		w.routes.defaultSec = 600
		doc := w.search(serviceActor(w.carrier, &company), cap.ID, radiusPolicy(500, 0))
		ids := candidateIDs(doc)
		if len(ids) != 2 || !containsID(ids, visible.ID) || !containsID(ids, invited.ID) {
			t.Fatalf("visible %+v", ids)
		}
	})

	t.Run("BNO209_BNO210_ANONYMIZED_GEO", func(t *testing.T) {
		w := newWorld(t)
		cap := w.capacity(domain.CapacityAvailable, domain.SourceManual, 0, 0)
		loc := uuid.New()
		pickup := place(0, 0.02, "Secret yard")
		pickup.LocationID = &loc
		pickup.Label = "Dock 4"
		load := w.load(domain.VisAnonymized, pickup, place(0, 1, "Far"), nil)
		w.routes.defaultM = 64000
		w.routes.defaultSec = 3600
		doc := w.search(w.actor(), cap.ID, radiusPolicy(500, 0))
		if len(doc.Candidates) != 1 || doc.Candidates[0].DeadheadBucket != "50-100" || doc.Candidates[0].RoadDeadheadKm != nil {
			t.Fatalf("public %+v", doc.Candidates)
		}
		raw, _ := json.Marshal(doc.Candidates[0].Load)
		for _, leaked := range []string{"latitude", "longitude", "location_id", "label", "address", "postal"} {
			if containsString(string(raw), leaked) {
				t.Fatalf("BNO210 leaked %s %s", leaked, raw)
			}
		}
		_, rows, err := w.store.GetSearch(context.Background(), w.carrier, doc.SearchID)
		if err != nil || len(rows) != 1 || rows[0].RoadDeadheadKm == nil || *rows[0].RoadDeadheadKm != 64 {
			t.Fatalf("BNO209 internal %+v %v", rows, err)
		}
		if rows[0].LoadOpportunityID != load.ID {
			t.Fatal("identity")
		}
	})

	t.Run("BNO211_BNO212_BNO222_RADIUS", func(t *testing.T) {
		w := newWorld(t)
		cap := w.capacity(domain.CapacityAvailable, domain.SourceManual, 0, 0)
		near := w.load(domain.VisMarketplace, place(0, kmDeg(5), "Near"), place(0, 1, "Drop"), nil)
		far := w.load(domain.VisMarketplace, place(0, kmDeg(300), "Far"), place(0, 1, "Drop"), nil)
		w.routes.roads[roadKey(routing.Point{}, routing.Point{Longitude: kmDeg(5)})] = roadCell{m: 90000, sec: 600}
		doc := w.search(w.actor(), cap.ID, radiusPolicy(50, 80))
		if doc.EligibleCandidateCount != 0 || doc.RejectionCountsByReason[domain.ReasonOutsideRadius] != 1 || doc.RejectionCountsByReason[domain.ReasonDeadheadExceeded] != 1 {
			t.Fatalf("%+v", doc.RejectionCountsByReason)
		}
		if containsID(candidateIDs(doc), far.ID) || containsID(candidateIDs(doc), near.ID) {
			t.Fatal("rejected loads leaked")
		}
		if w.routes.sawHaversineSubstitute {
			t.Fatal("haversine used as road")
		}
	})

	t.Run("BNO222_HAVERSINE_NEVER_ROAD_DISTANCE", func(t *testing.T) {
		w := newWorld(t)
		cap := w.capacity(domain.CapacityAvailable, domain.SourceManual, 0, 0)
		load := w.load(domain.VisMarketplace, place(0, kmDeg(5), "Near"), place(0, 1, "Drop"), nil)
		w.routes.roads[roadKey(routing.Point{}, routing.Point{Longitude: kmDeg(5)})] = roadCell{m: 40000, sec: 600}
		doc := w.search(w.actor(), cap.ID, radiusPolicy(100, 80))
		if len(doc.Candidates) != 1 || doc.Candidates[0].RoadDeadheadKm == nil || *doc.Candidates[0].RoadDeadheadKm != 40 {
			t.Fatalf("%+v", doc.Candidates)
		}
		if doc.Candidates[0].LoadOpportunityID != load.ID {
			t.Fatal("identity")
		}
	})

	t.Run("BNO223_ROUTING_FAILURE_FAILS_CLOSED", func(t *testing.T) {
		w := newWorld(t)
		cap := w.capacity(domain.CapacityAvailable, domain.SourceManual, 0, 0)
		w.load(domain.VisMarketplace, place(0, kmDeg(5), "Near"), place(0, 1, "Drop"), nil)
		w.routes.fail = true
		doc := w.search(w.actor(), cap.ID, radiusPolicy(100, 80))
		if doc.EligibleCandidateCount != 0 || doc.RejectionCountsByReason[domain.ReasonRoadDistanceUnknown] != 1 || len(doc.Candidates) != 0 {
			t.Fatalf("%+v", doc)
		}
	})

	t.Run("BNO213_TO_BNO217_DIRECTIONAL_CORRIDOR", func(t *testing.T) {
		w := newWorld(t)
		targetID := uuid.New()
		w.dir.snaps[targetID] = domain.LocationSnapshot{ID: targetID, Latitude: f64(0), Longitude: f64(10), Status: "ACTIVE"}
		w.routes.line = [][]float64{{0, 0}, {10, 0}}
		w.routes.baselineM = 1800000
		cap := w.capacity(domain.CapacityAvailable, domain.SourceManual, 0, 0)
		aPickup := routing.Point{Latitude: kmDeg(22), Longitude: kmDeg(240)}
		bPickup := routing.Point{Latitude: kmDeg(35), Longitude: kmDeg(200)}
		cPickup := routing.Point{Latitude: kmDeg(10), Longitude: kmDeg(180)}
		aDrop := routing.Point{Longitude: 8}
		bDrop := routing.Point{Longitude: 7}
		cDrop := routing.Point{Longitude: 1}
		target := routing.Point{Longitude: 10}
		release := routing.Point{}
		a := w.load(domain.VisMarketplace, pointPlace(aPickup, "A"), pointPlace(aDrop, "Ad"), window(w.at.Add(-time.Hour), w.at.Add(4*time.Hour)))
		b := w.load(domain.VisMarketplace, pointPlace(bPickup, "B"), pointPlace(bDrop, "Bd"), window(w.at.Add(-time.Hour), w.at.Add(8*time.Hour)))
		c := w.load(domain.VisMarketplace, pointPlace(cPickup, "C"), pointPlace(cDrop, "Cd"), window(w.at.Add(-time.Hour), w.at.Add(8*time.Hour)))
		w.routes.roads[roadKey(release, aPickup)] = roadCell{m: 64000, sec: 3600}
		w.routes.roads[roadKey(release, bPickup)] = roadCell{m: 97000, sec: 3600}
		w.routes.roads[roadKey(release, cPickup)] = roadCell{m: 40000, sec: 1800}
		w.routes.roads[roadKey(aPickup, target)] = roadCell{m: 800000, sec: 1}
		w.routes.roads[roadKey(aDrop, target)] = roadCell{m: 400000, sec: 1}
		w.routes.roads[roadKey(bPickup, target)] = roadCell{m: 600000, sec: 1}
		w.routes.roads[roadKey(bDrop, target)] = roadCell{m: 300000, sec: 1}
		w.routes.roads[roadKey(cPickup, target)] = roadCell{m: 500000, sec: 1}
		w.routes.roads[roadKey(cDrop, target)] = roadCell{m: 900000, sec: 1}
		policy := domain.NextLoadSearchPolicy{
			SearchMode: domain.SearchDirectionalCorridor, TargetLocationID: &targetID,
			ForwardSearchKm: f64(500), CorridorDeviationKm: f64(50), MaxDeadheadKm: f64(80),
			ObjectiveProfile: "MIN_DEADHEAD",
		}
		doc := w.search(w.actor(), cap.ID, policy)
		if doc.EligibleCandidateCount != 1 || len(doc.Candidates) != 1 || doc.Candidates[0].LoadOpportunityID != a.ID {
			t.Fatalf("eligible %+v counts %+v", doc.Candidates, doc.RejectionCountsByReason)
		}
		got := doc.Candidates[0]
		if got.RoadDeadheadKm == nil || *got.RoadDeadheadKm != 64 || got.ForwardProgressKm == nil || got.LateralDistanceKm == nil {
			t.Fatalf("A %+v", got)
		}
		if math.Abs(*got.ForwardProgressKm-240) > 8 || math.Abs(*got.LateralDistanceKm-22) > 8 {
			t.Fatalf("corridor geometry %+v", got)
		}
		if doc.RejectionCountsByReason[domain.ReasonDeadheadExceeded] != 1 || doc.RejectionCountsByReason[domain.ReasonDeliveryNotTowardTarget] != 1 {
			t.Fatalf("counts %+v", doc.RejectionCountsByReason)
		}
		raw, _ := json.Marshal(doc)
		if containsString(string(raw), b.ID.String()) || containsString(string(raw), c.ID.String()) {
			t.Fatal("rejected load leaked")
		}
	})

	t.Run("BNO214_BNO216_CORRIDOR_REJECTS", func(t *testing.T) {
		w := newWorld(t)
		targetID := uuid.New()
		w.dir.snaps[targetID] = domain.LocationSnapshot{ID: targetID, Latitude: f64(0), Longitude: f64(10)}
		w.routes.line = [][]float64{{0, 0}, {10, 0}}
		cap := w.capacity(domain.CapacityAvailable, domain.SourceManual, 0, 0)
		behind := w.load(domain.VisMarketplace, place(0, -kmDeg(20), "Back"), place(0, 1, "Drop"), nil)
		side := w.load(domain.VisMarketplace, place(kmDeg(80), kmDeg(100), "Side"), place(0, 1, "Drop"), nil)
		past := w.load(domain.VisMarketplace, place(0, kmDeg(600), "Past"), place(0, 1, "Drop"), nil)
		policy := domain.NextLoadSearchPolicy{
			SearchMode: domain.SearchDirectionalCorridor, TargetLocationID: &targetID,
			ForwardSearchKm: f64(500), CorridorDeviationKm: f64(50), MaxDeadheadKm: f64(80),
			ObjectiveProfile: "MIN_DEADHEAD",
		}
		doc := w.search(w.actor(), cap.ID, policy)
		if doc.RejectionCountsByReason[domain.ReasonBacktrackRejected] != 1 || doc.RejectionCountsByReason[domain.ReasonLateralExceeded] != 1 || doc.RejectionCountsByReason[domain.ReasonForwardExceeded] != 1 {
			t.Fatalf("%+v", doc.RejectionCountsByReason)
		}
		if containsID(candidateIDs(doc), behind.ID) || containsID(candidateIDs(doc), side.ID) || containsID(candidateIDs(doc), past.ID) {
			t.Fatal("geometry rejects leaked")
		}
	})

	t.Run("BNO218_BNO219_ROUTE_ELLIPSE", func(t *testing.T) {
		w := newWorld(t)
		targetID := uuid.New()
		w.dir.snaps[targetID] = domain.LocationSnapshot{ID: targetID, Latitude: f64(0), Longitude: f64(10)}
		w.routes.line = [][]float64{{0, 0}, {10, 0}}
		w.routes.baselineM = 1000000
		cap := w.capacity(domain.CapacityAvailable, domain.SourceManual, 0, 0)
		pickup := routing.Point{Longitude: 1}
		delivery := routing.Point{Longitude: 2}
		target := routing.Point{Longitude: 10}
		release := routing.Point{}
		load := w.load(domain.VisMarketplace, pointPlace(pickup, "P"), pointPlace(delivery, "D"), nil)
		w.routes.roads[roadKey(release, pickup)] = roadCell{m: 100000, sec: 600}
		w.routes.roads[roadKey(pickup, delivery)] = roadCell{m: 200000, sec: 600}
		w.routes.roads[roadKey(delivery, target)] = roadCell{m: 800000, sec: 600}
		w.routes.roads[roadKey(pickup, target)] = roadCell{m: 900000, sec: 600}
		pass := domain.NextLoadSearchPolicy{
			SearchMode: domain.SearchRouteEllipse, TargetLocationID: &targetID, MaxRouteIncreaseKm: f64(150),
			ObjectiveProfile: "MIN_DEADHEAD",
		}
		doc := w.search(w.actor(), cap.ID, pass)
		if len(doc.Candidates) != 1 || doc.Candidates[0].RouteIncreaseKm == nil || *doc.Candidates[0].RouteIncreaseKm != 100 {
			t.Fatalf("ellipse %+v counts %+v", doc.Candidates, doc.RejectionCountsByReason)
		}
		if doc.Candidates[0].LoadOpportunityID != load.ID {
			t.Fatal("identity")
		}
		failPolicy := pass
		failPolicy.MaxRouteIncreaseKm = f64(50)
		rejected := w.search(w.actor(), cap.ID, failPolicy)
		if rejected.EligibleCandidateCount != 0 || rejected.RejectionCountsByReason[domain.ReasonRouteIncreaseExceeded] != 1 {
			t.Fatalf("limit %+v", rejected.RejectionCountsByReason)
		}
	})

	t.Run("BNO220_BNO221_DEADHEAD_LIMITS", func(t *testing.T) {
		w := newWorld(t)
		cap := w.capacity(domain.CapacityAvailable, domain.SourceManual, 0, 0)
		pickup := routing.Point{Longitude: kmDeg(5)}
		w.load(domain.VisMarketplace, pointPlace(pickup, "P"), place(0, 1, "D"), nil)
		w.routes.roads[roadKey(routing.Point{}, pickup)] = roadCell{m: 90000, sec: 7200}
		km := radiusPolicy(100, 80)
		doc := w.search(w.actor(), cap.ID, km)
		if doc.RejectionCountsByReason[domain.ReasonDeadheadExceeded] != 1 {
			t.Fatalf("km %+v", doc.RejectionCountsByReason)
		}
		minutes := radiusPolicy(100, 0)
		minutes.MaxDeadheadKm = nil
		minutes.MaxDeadheadMinutes = f64(60)
		timed := w.search(w.actor(), cap.ID, minutes)
		if timed.RejectionCountsByReason[domain.ReasonDeadheadTimeExceeded] != 1 {
			t.Fatalf("minutes %+v", timed.RejectionCountsByReason)
		}
	})

	t.Run("BNO224_BNO226_PICKUP_TIME", func(t *testing.T) {
		w := newWorld(t)
		cap := w.capacity(domain.CapacityAvailable, domain.SourceManual, 0, 0)
		pickup := routing.Point{Longitude: kmDeg(4)}
		passLoad := w.load(domain.VisMarketplace, pointPlace(pickup, "Pass"), place(0, 1, "D"), window(w.at.Add(-time.Hour), w.at.Add(2*time.Hour)))
		miss := w.load(domain.VisMarketplace, pointPlace(routing.Point{Longitude: kmDeg(6)}, "Miss"), place(0, 1, "D"), window(w.at.Add(-2*time.Hour), w.at.Add(20*time.Minute)))
		wait := w.load(domain.VisMarketplace, pointPlace(routing.Point{Longitude: kmDeg(8)}, "Wait"), place(0, 1, "D"), window(w.at.Add(90*time.Minute), w.at.Add(4*time.Hour)))
		w.routes.defaultM = 30000
		w.routes.defaultSec = 1800
		doc := w.search(w.actor(), cap.ID, radiusPolicy(500, 0))
		if doc.RejectionCountsByReason[domain.ReasonPickupWindowMissed] != 1 {
			t.Fatalf("miss %+v", doc.RejectionCountsByReason)
		}
		var passed, waiting *SearchCandidateView
		for i := range doc.Candidates {
			switch doc.Candidates[i].LoadOpportunityID {
			case passLoad.ID:
				passed = &doc.Candidates[i]
			case wait.ID:
				waiting = &doc.Candidates[i]
			}
		}
		if passed == nil || passed.Timing != "FEASIBLE" || waiting == nil || waiting.WaitingMinutes == nil || *waiting.WaitingMinutes != 60 {
			t.Fatalf("pass %+v wait %+v", passed, waiting)
		}
		if containsID(candidateIDs(doc), miss.ID) {
			t.Fatal("missed window leaked")
		}
	})

	t.Run("BNO224_PREDICTED_AVAILABLE_AT", func(t *testing.T) {
		w := newWorld(t)
		cap := w.capacity(domain.CapacityAvailable, domain.SourceCurrentShipmentPrediction, 0, 0)
		if err := w.store.Within(context.Background(), func(tx repository.Tx) error {
			return tx.InsertPrediction(context.Background(), domain.PredictedCapacity{
				ID: uuid.New(), CapacityID: cap.ID, OwnerTenantID: w.carrier, IsCurrent: true,
				PredictedAvailableAt:    w.at.Add(10 * time.Hour),
				AvailabilityWindowStart: w.at.Add(9 * time.Hour),
				AvailabilityWindowEnd:   w.at.Add(12 * time.Hour),
			})
		}); err != nil {
			t.Fatal(err)
		}
		pickup := routing.Point{Longitude: kmDeg(3)}
		w.load(domain.VisMarketplace, pointPlace(pickup, "P"), place(0, 1, "D"), window(w.at, w.at.Add(2*time.Hour)))
		w.routes.defaultM = 10000
		w.routes.defaultSec = 1800
		doc := w.search(w.actor(), cap.ID, radiusPolicy(100, 0))
		if doc.RejectionCountsByReason[domain.ReasonPickupWindowMissed] != 1 || doc.EligibleCandidateCount != 0 {
			t.Fatalf("prediction timing %+v", doc.RejectionCountsByReason)
		}
		predicted := cap
		predicted.ID = uuid.New()
		predicted.Status = domain.CapacityPredicted
		if err := w.store.Within(context.Background(), func(tx repository.Tx) error {
			return tx.InsertCapacity(context.Background(), predicted)
		}); err != nil {
			t.Fatal(err)
		}
		_, err := w.svc.SearchNextLoad(context.Background(), w.actor(), SearchCommand{CapacityID: predicted.ID, Policy: radiusPolicy(100, 0)})
		var app *apperrors.AppError
		if !errors.As(err, &app) || app.Details["reason"] != "CAPACITY_NOT_EXECUTABLE" {
			t.Fatalf("predicted %v", err)
		}
	})

	t.Run("BNO227_BNO228_PAYLOAD_VOLUME", func(t *testing.T) {
		w := newWorld(t)
		cap := w.capacity(domain.CapacityAvailable, domain.SourceManual, 0, 0)
		cap.PayloadRemainingKg = f64(1000)
		cap.VolumeRemainingM3 = f64(10)
		cap.Version = 2
		if err := w.store.Within(context.Background(), func(tx repository.Tx) error {
			return tx.UpdateCapacity(context.Background(), cap, 1)
		}); err != nil {
			t.Fatal(err)
		}
		heavy := w.load(domain.VisMarketplace, place(0, kmDeg(2), "H"), place(0, 1, "D"), nil)
		heavy.WeightKg = f64(1500)
		heavy.Version = 2
		bulky := w.load(domain.VisMarketplace, place(0, kmDeg(3), "B"), place(0, 1, "D"), nil)
		bulky.VolumeM3 = f64(12)
		bulky.Version = 2
		if err := w.store.Within(context.Background(), func(tx repository.Tx) error {
			if err := tx.UpdateLoad(context.Background(), heavy, 1); err != nil {
				return err
			}
			return tx.UpdateLoad(context.Background(), bulky, 1)
		}); err != nil {
			t.Fatal(err)
		}
		w.routes.defaultM = 10000
		w.routes.defaultSec = 600
		doc := w.search(w.actor(), cap.ID, radiusPolicy(100, 0))
		if doc.RejectionCountsByReason[domain.ReasonPayloadExceeded] != 1 || doc.RejectionCountsByReason[domain.ReasonVolumeExceeded] != 1 {
			t.Fatalf("%+v", doc.RejectionCountsByReason)
		}
	})

	t.Run("BNO229_BNO231_COMPATIBILITY", func(t *testing.T) {
		w := newWorld(t)
		cap := w.capacity(domain.CapacityAvailable, domain.SourceManual, 0, 0)
		cap.BodyType = "TENT"
		cap.Version = 2
		if err := w.store.Within(context.Background(), func(tx repository.Tx) error {
			return tx.UpdateCapacity(context.Background(), cap, 1)
		}); err != nil {
			t.Fatal(err)
		}
		okLoad := w.load(domain.VisMarketplace, place(0, kmDeg(1), "Ok"), place(0, 1, "D"), nil)
		okLoad.Cargo.RequiredBodyTypes = []string{"TENT"}
		okLoad.Version = 2
		bad := w.load(domain.VisMarketplace, place(0, kmDeg(1.2), "Bad"), place(0, 1, "D"), nil)
		bad.Cargo.RequiredBodyTypes = []string{"REEFER"}
		bad.Version = 2
		unknown := w.load(domain.VisMarketplace, place(0, kmDeg(1.4), "Unknown"), place(0, 1, "D"), nil)
		unknown.Cargo.RequiredBodyTypes = []string{"TENT"}
		unknown.Version = 2
		bare := cap
		bare.ID = uuid.New()
		bare.BodyType = ""
		bare.Version = 1
		if err := w.store.Within(context.Background(), func(tx repository.Tx) error {
			if err := tx.UpdateLoad(context.Background(), okLoad, 1); err != nil {
				return err
			}
			if err := tx.UpdateLoad(context.Background(), bad, 1); err != nil {
				return err
			}
			if err := tx.UpdateLoad(context.Background(), unknown, 1); err != nil {
				return err
			}
			return tx.InsertCapacity(context.Background(), bare)
		}); err != nil {
			t.Fatal(err)
		}
		w.routes.defaultM = 10000
		w.routes.defaultSec = 600
		doc := w.search(w.actor(), cap.ID, radiusPolicy(100, 0))
		if doc.EligibleCandidateCount != 2 || doc.RejectionCountsByReason[domain.ReasonCargoIncompatible] != 1 {
			t.Fatalf("compatible %+v %+v", doc.Candidates, doc.RejectionCountsByReason)
		}
		var matched bool
		for _, candidate := range doc.Candidates {
			if candidate.LoadOpportunityID == okLoad.ID && candidate.Compatibility == "COMPATIBLE" {
				matched = true
			}
		}
		if !matched {
			t.Fatalf("compatible candidate missing %+v", doc.Candidates)
		}
		if doc.RejectionCountsByReason[domain.ReasonCargoIncompatible] != 1 {
			t.Fatalf("incompatible %+v", doc.RejectionCountsByReason)
		}
		unknownDoc := w.search(w.actor(), bare.ID, radiusPolicy(100, 0))
		if unknownDoc.RejectionCountsByReason[domain.ReasonCargoIndeterminate] < 1 || unknownDoc.EligibleCandidateCount != 0 {
			t.Fatalf("indeterminate %+v", unknownDoc.RejectionCountsByReason)
		}
	})

	t.Run("BNO232_BNO233_SEARCH_BATCH_IDENTITY", func(t *testing.T) {
		w := newWorld(t)
		cap := w.capacity(domain.CapacityAvailable, domain.SourceManual, 0, 0)
		for i := 0; i < 30; i++ {
			w.load(domain.VisMarketplace, place(0, float64(i+1)/1000, fmt.Sprintf("L%d", i)), place(0, 1, "D"), nil)
		}
		w.routes.defaultM = 10000
		w.routes.defaultSec = 600
		doc := w.search(w.actor(), cap.ID, radiusPolicy(5000, 0))
		if doc.EligibleCandidateCount != 30 || w.routes.maxDests > routing.SyncMatrixLimit || w.routes.calls < 2 {
			t.Fatalf("count %d calls %d max %d", doc.EligibleCandidateCount, w.routes.calls, w.routes.maxDests)
		}
		if !sortIsIDOrder(doc.Candidates) {
			t.Fatal("order")
		}
	})

	t.Run("BNO234_BNO236_PERSIST_PRIVACY", func(t *testing.T) {
		w := newWorld(t)
		cap := w.capacity(domain.CapacityAvailable, domain.SourceManual, 0, 0)
		hidden := w.load(domain.VisMarketplace, place(0, kmDeg(400), "Far"), place(0, 1, "D"), nil)
		doc := w.search(w.actor(), cap.ID, radiusPolicy(20, 80))
		run, rows, err := w.store.GetSearch(context.Background(), w.carrier, doc.SearchID)
		if err != nil || run.CapacityID != cap.ID || run.RoutingProvider == "" || len(rows) != 1 || rows[0].Eligibility != "REJECTED" {
			t.Fatalf("persisted %+v %+v %v", run, rows, err)
		}
		if _, _, err := w.store.GetSearch(context.Background(), uuid.New(), doc.SearchID); !errors.Is(err, repository.ErrNotFound) {
			t.Fatalf("foreign search %v", err)
		}
		raw, _ := json.Marshal(doc)
		if containsString(string(raw), hidden.ID.String()) || doc.RejectionCountsByReason[domain.ReasonOutsideRadius] != 1 {
			t.Fatalf("leak %s %+v", raw, doc.RejectionCountsByReason)
		}
	})

	t.Run("BNO237_BNO238_HARD_LIMIT_MIN", func(t *testing.T) {
		w := newWorld(t)
		cap := w.capacity(domain.CapacityAvailable, domain.SourceManual, 0, 0)
		carrierLimit, capacityLimit, requestLimit := 100.0, 70.0, 90.0
		objective := domain.NextLoadSearchPolicy{SearchMode: domain.SearchRadius, RadiusKm: f64(500), ObjectiveProfile: "MIN_DEADHEAD", MaxDeadheadKm: &carrierLimit}
		if err := w.store.UpsertCarrierPolicy(context.Background(), w.carrier, objective); err != nil {
			t.Fatal(err)
		}
		capacityPolicy := domain.NextLoadSearchPolicy{MaxDeadheadKm: &capacityLimit}
		if err := w.store.UpsertCapacityPolicy(context.Background(), w.carrier, cap.ID, capacityPolicy); err != nil {
			t.Fatal(err)
		}
		pickup := routing.Point{Longitude: kmDeg(2)}
		w.load(domain.VisMarketplace, pointPlace(pickup, "P"), place(0, 1, "D"), nil)
		w.routes.roads[roadKey(routing.Point{}, pickup)] = roadCell{m: 80000, sec: 600}
		doc := w.search(w.actor(), cap.ID, domain.NextLoadSearchPolicy{MaxDeadheadKm: &requestLimit})
		if doc.EffectivePolicy.MaxDeadheadKm == nil || *doc.EffectivePolicy.MaxDeadheadKm != 70 || doc.RejectionCountsByReason[domain.ReasonDeadheadExceeded] != 1 {
			t.Fatalf("min %+v %+v", doc.EffectivePolicy, doc.RejectionCountsByReason)
		}
		widen := 200.0
		widened := w.search(w.actor(), cap.ID, domain.NextLoadSearchPolicy{MaxDeadheadKm: &widen})
		if widened.EffectivePolicy.MaxDeadheadKm == nil || *widened.EffectivePolicy.MaxDeadheadKm != 70 {
			t.Fatalf("widen %+v", widened.EffectivePolicy)
		}
	})

	t.Run("BNO239_DETERMINISTIC_REPEAT", func(t *testing.T) {
		w := newWorld(t)
		cap := w.capacity(domain.CapacityAvailable, domain.SourceManual, 0, 0)
		w.load(domain.VisMarketplace, place(0, kmDeg(3), "A"), place(0, 1, "D"), nil)
		w.load(domain.VisMarketplace, place(0, kmDeg(4), "B"), place(0, 1, "D"), nil)
		w.routes.defaultM = 20000
		w.routes.defaultSec = 600
		first := w.search(w.actor(), cap.ID, radiusPolicy(100, 0))
		second := w.search(w.actor(), cap.ID, radiusPolicy(100, 0))
		if first.PolicyFingerprint != second.PolicyFingerprint || candidateKey(first) != candidateKey(second) {
			t.Fatalf("%s vs %s", candidateKey(first), candidateKey(second))
		}
	})

	t.Run("BNO240_TENANT_ISOLATION", func(t *testing.T) {
		w := newWorld(t)
		cap := w.capacity(domain.CapacityAvailable, domain.SourceManual, 0, 0)
		w.load(domain.VisMarketplace, place(0, kmDeg(2), "A"), place(0, 1, "D"), nil)
		w.routes.defaultM = 10000
		w.routes.defaultSec = 600
		doc := w.search(w.actor(), cap.ID, radiusPolicy(100, 0))
		other := serviceActor(uuid.New(), nil)
		if _, err := w.svc.SearchNextLoad(context.Background(), other, SearchCommand{CapacityID: cap.ID, Policy: radiusPolicy(100, 0)}); !notFound(err) {
			t.Fatalf("capacity %v", err)
		}
		if _, _, err := w.store.GetSearch(context.Background(), other.TenantID, doc.SearchID); !errors.Is(err, repository.ErrNotFound) {
			t.Fatalf("search %v", err)
		}
	})
}

type world struct {
	t       *testing.T
	store   *repository.Memory
	routes  *scripted
	dir     *dirMap
	svc     *Service
	carrier uuid.UUID
	shipper uuid.UUID
	at      time.Time
}

func newWorld(t *testing.T) *world {
	t.Helper()
	store := repository.NewMemory()
	routes := &scripted{roads: map[string]roadCell{}}
	dir := &dirMap{snaps: map[uuid.UUID]domain.LocationSnapshot{}}
	store.SeedScoreProfiles(profilefixture.V1())
	svc := New(store, nil)
	svc.UseRouting(routes)
	svc.UsePolicies(store)
	svc.UseScoreProfiles(store)
	svc.UseDirectory(dir)
	return &world{t: t, store: store, routes: routes, dir: dir, svc: svc, carrier: uuid.New(), shipper: uuid.New(), at: time.Date(2026, 9, 25, 8, 0, 0, 0, time.UTC)}
}

func (w *world) actor() Actor { return serviceActor(w.carrier, nil) }

func serviceActor(tenant uuid.UUID, company *uuid.UUID) Actor {
	return Actor{TenantID: tenant, UserID: uuid.New(), CompanyID: company}
}

func (w *world) capacity(status, source string, lat, lon float64) domain.Capacity {
	w.t.Helper()
	cap := domain.Capacity{
		ID: uuid.New(), OwnerTenantID: w.carrier, Latitude: f64(lat), Longitude: f64(lon),
		AvailableFrom: w.at, AvailableUntil: w.at.Add(24 * time.Hour), Source: source,
		VisibilityScope: domain.CapVisPrivate, Status: status, Version: 1,
	}
	if err := w.store.Within(context.Background(), func(tx repository.Tx) error {
		return tx.InsertCapacity(context.Background(), cap)
	}); err != nil {
		w.t.Fatal(err)
	}
	return cap
}

func (w *world) load(visibility string, pickup, delivery domain.Place, pickupWindow *domain.TimeWindow) domain.LoadOpportunity {
	w.t.Helper()
	return w.loadOwned(w.shipper, visibility, nil, pickup, delivery, pickupWindow)
}

func (w *world) loadOwned(owner uuid.UUID, visibility string, invited []uuid.UUID, pickup, delivery domain.Place, pickupWindow *domain.TimeWindow) domain.LoadOpportunity {
	w.t.Helper()
	status := domain.LoadPublished
	if visibility == domain.LoadDraft {
		status = domain.LoadDraft
		visibility = domain.VisPrivate
	}
	load := domain.LoadOpportunity{
		ID: uuid.New(), OwnerTenantID: owner, SourceType: domain.SourceTransportOrder, SourceID: uuid.New(),
		Pickup: pickup, Delivery: delivery, VisibilityScope: visibility, InvitedCarrierCompanyIDs: invited,
		Status: status, Version: 1, CreatedAt: w.at, UpdatedAt: w.at,
	}
	if pickupWindow != nil {
		load.PickupWindow = *pickupWindow
	}
	if err := w.store.Within(context.Background(), func(tx repository.Tx) error {
		return tx.InsertLoad(context.Background(), load)
	}); err != nil {
		w.t.Fatal(err)
	}
	return load
}

func (w *world) search(actor Actor, capacityID uuid.UUID, policy domain.NextLoadSearchPolicy) SearchResponse {
	w.t.Helper()
	result, err := w.svc.SearchNextLoad(context.Background(), actor, SearchCommand{CapacityID: capacityID, Policy: policy})
	if err != nil {
		w.t.Fatal(err)
	}
	var doc SearchResponse
	if err := json.Unmarshal(result.Body, &doc); err != nil {
		w.t.Fatal(err)
	}
	if doc.SearchID == uuid.Nil || containsString(string(result.Body), "match_score") {
		w.t.Fatalf("response %s", result.Body)
	}
	return doc
}

func radiusPolicy(radius, deadhead float64) domain.NextLoadSearchPolicy {
	policy := domain.NextLoadSearchPolicy{SearchMode: domain.SearchRadius, RadiusKm: f64(radius), ObjectiveProfile: "MIN_DEADHEAD"}
	if deadhead > 0 {
		policy.MaxDeadheadKm = f64(deadhead)
	}
	return policy
}

func place(lat, lon float64, city string) domain.Place {
	return domain.Place{Latitude: f64(lat), Longitude: f64(lon), City: city, CountryCode: "RU"}
}

func pointPlace(p routing.Point, city string) domain.Place {
	return place(p.Latitude, p.Longitude, city)
}

func window(start, end time.Time) *domain.TimeWindow {
	return &domain.TimeWindow{Start: &start, End: &end}
}

func f64(v float64) *float64 { return &v }

func kmDeg(km float64) float64 { return km / (6371 * math.Pi / 180) }

func notFound(err error) bool {
	var app *apperrors.AppError
	return errors.As(err, &app) && app.Code == apperrors.CodeNotFound
}

func candidateIDs(doc SearchResponse) []uuid.UUID {
	ids := make([]uuid.UUID, 0, len(doc.Candidates))
	for _, candidate := range doc.Candidates {
		ids = append(ids, candidate.LoadOpportunityID)
	}
	return ids
}

func containsID(ids []uuid.UUID, id uuid.UUID) bool {
	for _, item := range ids {
		if item == id {
			return true
		}
	}
	return false
}

func containsString(raw, needle string) bool {
	return needle != "" && len(raw) >= len(needle) && (raw == needle || len(raw) > 0 && (func() bool { return stringContains(raw, needle) })())
}

func stringContains(raw, needle string) bool {
	return len(needle) > 0 && (len(raw) >= len(needle) && (raw == needle || indexOf(raw, needle) >= 0))
}

func indexOf(raw, needle string) int {
	for i := 0; i+len(needle) <= len(raw); i++ {
		if raw[i:i+len(needle)] == needle {
			return i
		}
	}
	return -1
}

func sortIsIDOrder(rows []SearchCandidateView) bool {
	for i := 1; i < len(rows); i++ {
		if rows[i-1].LoadOpportunityID.String() > rows[i].LoadOpportunityID.String() {
			return false
		}
	}
	return true
}

func candidateKey(doc SearchResponse) string {
	raw, _ := json.Marshal(struct {
		IDs    []string
		Counts map[string]int
		Mode   string
	}{IDs: func() []string {
		out := make([]string, 0, len(doc.Candidates))
		for _, candidate := range doc.Candidates {
			out = append(out, candidate.LoadOpportunityID.String())
		}
		return out
	}(), Counts: doc.RejectionCountsByReason, Mode: doc.SearchMode})
	return string(raw)
}

type roadCell struct{ m, sec int }

type scripted struct {
	roads                  map[string]roadCell
	defaultM               int
	defaultSec             int
	fail                   bool
	line                   [][]float64
	baselineM              int
	calls                  int
	maxDests               int
	sawHaversineSubstitute bool
}

func (s *scripted) Route(context.Context, routing.RouteRequest) (routing.RouteResult, error) {
	if s.fail {
		return routing.RouteResult{}, routing.ErrProviderUnavailable
	}
	line := s.line
	if len(line) == 0 {
		line = [][]float64{{0, 0}, {1, 0}}
	}
	return routing.RouteResult{DistanceM: s.baselineM, DurationSeconds: 1, Geometry: routing.Geometry{Type: "LineString", Coordinates: line}}, nil
}

func (s *scripted) Matrix(_ context.Context, req routing.MatrixRequest) (routing.MatrixResult, error) {
	s.calls++
	if len(req.Destinations) > s.maxDests {
		s.maxDests = len(req.Destinations)
	}
	if len(req.Origins) > routing.SyncMatrixLimit || len(req.Destinations) > routing.SyncMatrixLimit {
		return routing.MatrixResult{}, routing.ErrInvalidResponse
	}
	if s.fail {
		return routing.MatrixResult{}, routing.ErrProviderUnavailable
	}
	cells := make([]routing.MatrixCell, 0, len(req.Origins)*len(req.Destinations))
	for i, origin := range req.Origins {
		for j, dest := range req.Destinations {
			cell := routing.MatrixCell{OriginIndex: i, DestinationIndex: j, Err: routing.ErrRouteNotFound}
			if item, ok := s.roads[roadKey(origin, dest)]; ok {
				cell = routing.MatrixCell{OriginIndex: i, DestinationIndex: j, DistanceM: item.m, DurationSeconds: item.sec}
			} else if s.defaultM > 0 {
				cell = routing.MatrixCell{OriginIndex: i, DestinationIndex: j, DistanceM: s.defaultM, DurationSeconds: s.defaultSec}
			}
			cells = append(cells, cell)
		}
	}
	return routing.MatrixResult{Cells: cells}, nil
}

func roadKey(a, b routing.Point) string {
	return fmt.Sprintf("%.6f,%.6f->%.6f,%.6f", a.Latitude, a.Longitude, b.Latitude, b.Longitude)
}

type dirMap struct {
	snaps map[uuid.UUID]domain.LocationSnapshot
}

func (d dirMap) Projection(_ context.Context, _, locationID uuid.UUID) (domain.LocationSnapshot, error) {
	snap, ok := d.snaps[locationID]
	if !ok {
		return domain.LocationSnapshot{}, locationclient.ErrNotFound
	}
	return snap, nil
}

func (d dirMap) SourceEndpoints(context.Context, uuid.UUID, string, uuid.UUID) (uuid.UUID, uuid.UUID, error) {
	return uuid.Nil, uuid.Nil, locationclient.ErrNotFound
}
