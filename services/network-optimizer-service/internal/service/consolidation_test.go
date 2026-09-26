package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/freight-platform/network-optimizer-service/internal/compat"
	"github.com/freight-platform/network-optimizer-service/internal/domain"
	apperrors "github.com/freight-platform/network-optimizer-service/internal/platform/errors"
	"github.com/freight-platform/network-optimizer-service/internal/repository"
	"github.com/freight-platform/network-optimizer-service/internal/sourceverify"
)

func TestNLO03BPairwiseConsolidation(t *testing.T) {
	t.Run("NLO03B_001_OPT_IN_DEFAULT_FALSE", func(t *testing.T) {
		body := createOwned(t, false, false)
		if !strings.Contains(body, `"consolidation_allowed":false`) || !strings.Contains(body, `"cross_shipper_consolidation_allowed":false`) {
			t.Fatalf("defaults %s", body)
		}
	})
	t.Run("NLO03B_002_CREATE_LOAD_SAME_OWNER_OPT_IN", func(t *testing.T) {
		body := createOwned(t, true, false)
		if !strings.Contains(body, `"consolidation_allowed":true`) || !strings.Contains(body, `"cross_shipper_consolidation_allowed":false`) {
			t.Fatalf("create %s", body)
		}
	})
	t.Run("NLO03B_003_PATCH_LOAD_OPT_IN_FALSE_TO_TRUE", func(t *testing.T) {
		svc, actor, id := createdLoad(t, false, false)
		body := patchOptIn(t, svc, actor, id, 1, boolPtr(true), nil)
		if !strings.Contains(body, `"consolidation_allowed":true`) || !strings.Contains(body, `"version":2`) {
			t.Fatalf("patch %s", body)
		}
	})
	t.Run("NLO03B_004_PATCH_LOAD_OPT_IN_TRUE_TO_FALSE", func(t *testing.T) {
		svc, actor, id := createdLoad(t, true, true)
		body := patchOptIn(t, svc, actor, id, 1, boolPtr(false), boolPtr(false))
		if !strings.Contains(body, `"consolidation_allowed":false`) || !strings.Contains(body, `"cross_shipper_consolidation_allowed":false`) {
			t.Fatalf("patch %s", body)
		}
	})
	t.Run("NLO03B_005_OPT_IN_CHANGE_BUMPS_VERSION", func(t *testing.T) {
		svc, actor, body := createdLoad(t, false, false)
		patchOptIn(t, svc, actor, body, 1, boolPtr(true), nil)
		var doc struct {
			ID uuid.UUID `json:"id"`
		}
		if err := json.Unmarshal([]byte(body), &doc); err != nil {
			t.Fatal(err)
		}
		_, err := svc.UpdateLoad(context.Background(), actor, doc.ID, LoadPatch{Version: 1, ConsolidationAllowed: boolPtr(false), Changed: true}, "", "")
		var app *apperrors.AppError
		if !errors.As(err, &app) || app.Code != apperrors.CodeConflict {
			t.Fatalf("stale version %v", err)
		}
	})

	t.Run("NLO03B_006_SAME_OWNER_BOTH_OPTED_IN", func(t *testing.T) {
		doc := feasiblePair(t, true)
		if doc.FeasibleCandidateCount != 1 || doc.Candidates[0].Status != ConsolidationFeasible || doc.Candidates[0].ExecutionSupported {
			t.Fatalf("%+v", doc)
		}
	})
	t.Run("NLO03B_007_SAME_OWNER_ONE_NOT_OPTED_IN_EXCLUDED", func(t *testing.T) {
		w := newWorld(t)
		cap := w.readyCapacity()
		origin, dest := uuid.New(), uuid.New()
		win := span(w.at, w.at.Add(time.Hour))
		w.flagged(w.shipper, domain.VisMarketplace, nil, origin, dest, true, false, win, win, f64(10), domain.CargoConstraints{})
		w.flagged(w.shipper, domain.VisMarketplace, nil, origin, dest, false, true, win, win, f64(10), domain.CargoConstraints{})
		doc := w.consolidate(w.actor(), cap.ID, nil)
		if doc.EvaluatedPairCount != 0 || doc.ExcludedCountsByReason[ReasonSameOwnerOptInAbsent] != 1 || len(doc.Candidates) != 0 {
			t.Fatalf("%+v", doc)
		}
	})
	t.Run("NLO03B_008_CROSS_SHIPPER_BOTH_OPTED_IN", func(t *testing.T) {
		w, cap := crossPair(t, true, true)
		doc := w.consolidate(w.actor(), cap.ID, nil)
		if doc.EvaluatedPairCount != 1 || doc.FeasibleCandidateCount != 0 || doc.IndeterminateCandidateCount != 1 {
			t.Fatalf("%+v", doc)
		}
		if !has(doc.Candidates[0].IndeterminateReasonCodes, ReasonMultiPartyUnavailable) {
			t.Fatalf("%+v", doc.Candidates[0])
		}
	})
	t.Run("NLO03B_009_CROSS_SHIPPER_ONE_NOT_OPTED_IN_EXCLUDED", func(t *testing.T) {
		w, cap := crossPair(t, true, false)
		doc := w.consolidate(w.actor(), cap.ID, nil)
		if doc.EvaluatedPairCount != 0 || doc.ExcludedCountsByReason[ReasonCrossShipperOptInAbsent] != 1 {
			t.Fatalf("%+v", doc)
		}
	})
	t.Run("NLO03B_010_CROSS_FLAG_INDEPENDENT_FROM_GENERAL_FLAG", func(t *testing.T) {
		w := newWorld(t)
		cap := w.readyCapacity()
		origin, dest := uuid.New(), uuid.New()
		win := span(w.at, w.at.Add(2*time.Hour))
		w.flagged(w.shipper, domain.VisMarketplace, nil, origin, dest, false, true, win, win, f64(100), domain.CargoConstraints{})
		w.flagged(uuid.New(), domain.VisMarketplace, nil, origin, dest, false, true, win, win, f64(100), domain.CargoConstraints{})
		doc := w.consolidate(w.actor(), cap.ID, nil)
		if doc.EvaluatedPairCount != 1 || doc.ExcludedCountsByReason[ReasonCrossShipperOptInAbsent] != 0 {
			t.Fatalf("%+v", doc)
		}
	})
	t.Run("NLO03B_012_CROSS_SHIPPER_REFERENCE_CONTEXT_FAILS_CLOSED", func(t *testing.T) {
		w, cap := crossPair(t, true, true)
		doc := w.consolidate(w.actor(), cap.ID, nil)
		if doc.Candidates[0].Status == ConsolidationFeasible || !has(doc.Candidates[0].IndeterminateReasonCodes, ReasonMultiPartyUnavailable) {
			t.Fatalf("fail closed %+v", doc.Candidates[0])
		}
	})
	t.Run("NLO03B_012_PHYSICAL_HARD_REJECT_SURVIVES_UNPROVEN_CONTEXT", func(t *testing.T) {
		w := newWorld(t)
		cap := w.readyCapacity()
		origin, dest := uuid.New(), uuid.New()
		win := span(w.at, w.at.Add(2*time.Hour))
		w.flagged(w.shipper, domain.VisMarketplace, nil, origin, dest, false, true, win, win, f64(900), domain.CargoConstraints{})
		w.flagged(uuid.New(), domain.VisMarketplace, nil, origin, dest, false, true, win, win, f64(900), domain.CargoConstraints{})
		doc := w.consolidate(w.actor(), cap.ID, nil)
		if doc.HardRejectCandidateCount != 1 || doc.FeasibleCandidateCount != 0 || len(doc.Candidates) != 0 {
			t.Fatalf("%+v", doc)
		}
		if doc.HardRejectCountsByReason["PAYLOAD_EXCEEDED"] != 1 {
			t.Fatalf("%+v", doc.HardRejectCountsByReason)
		}
	})
	t.Run("NLO03B_012_MULTI_PARTY_RESTRICTIONS_REPRESENTED", func(t *testing.T) {
		w, cap, left, right := crossLoads(t)
		deny := right.OwnerTenantID
		w.svc.UseCatalog(tenantDenyCatalog{deny: deny})
		doc := w.consolidate(w.actor(), cap.ID, nil)
		if doc.FeasibleCandidateCount != 0 || doc.HardRejectCandidateCount != 1 || doc.HardRejectCountsByReason["TENANT_HARD_DENY"] != 1 {
			t.Fatalf("deny %+v", doc)
		}
		w.svc.UseCatalog(emptyCatalog{})
		doc = w.consolidate(w.actor(), cap.ID, nil)
		if doc.FeasibleCandidateCount != 1 || doc.Candidates[0].Status != ConsolidationFeasible {
			t.Fatalf("all contexts %+v", doc)
		}
		_ = left
	})

	t.Run("NLO03B_013_CANONICAL_SAME_OD_PAIR", func(t *testing.T) {
		doc := feasiblePair(t, true)
		if doc.EvaluatedPairCount != 1 || doc.Candidates[0].Members[0].Load.Pickup.LocationID == nil {
			t.Fatalf("%+v", doc.Candidates[0].Members[0].Load.Pickup)
		}
	})
	t.Run("NLO03B_014_SAME_LABEL_DIFFERENT_LOCATION_ID_NOT_PAIR", func(t *testing.T) {
		w := newWorld(t)
		cap := w.readyCapacity()
		win := span(w.at, w.at.Add(time.Hour))
		a := located(uuid.New(), "Same")
		b := located(uuid.New(), "Same")
		w.flagged(w.shipper, domain.VisMarketplace, nil, *a.LocationID, *b.LocationID, true, false, win, win, f64(10), domain.CargoConstraints{})
		w.flagged(w.shipper, domain.VisMarketplace, nil, uuid.New(), *b.LocationID, true, false, win, win, f64(10), domain.CargoConstraints{})
		doc := w.consolidate(w.actor(), cap.ID, nil)
		if doc.EvaluatedPairCount != 0 {
			t.Fatalf("%+v", doc)
		}
	})
	t.Run("NLO03B_015_SAME_CITY_NOT_PAIR", func(t *testing.T) {
		w := newWorld(t)
		cap := w.readyCapacity()
		win := span(w.at, w.at.Add(time.Hour))
		w.flagged(w.shipper, domain.VisMarketplace, nil, uuid.New(), uuid.New(), true, false, win, win, f64(10), domain.CargoConstraints{})
		w.flagged(w.shipper, domain.VisMarketplace, nil, uuid.New(), uuid.New(), true, false, win, win, f64(10), domain.CargoConstraints{})
		doc := w.consolidate(w.actor(), cap.ID, nil)
		if doc.EvaluatedPairCount != 0 || doc.PoolLoadCount != 2 {
			t.Fatalf("%+v", doc)
		}
	})
	t.Run("NLO03B_016_ORIGIN_IDENTITY_UNPROVEN_EXCLUDED", func(t *testing.T) {
		w := newWorld(t)
		cap := w.readyCapacity()
		win := span(w.at, w.at.Add(time.Hour))
		load := w.flagged(w.shipper, domain.VisMarketplace, nil, uuid.New(), uuid.New(), true, false, win, win, f64(10), domain.CargoConstraints{})
		load.Pickup.LocationID = nil
		replaceLoad(t, w, load)
		doc := w.consolidate(w.actor(), cap.ID, nil)
		raw := mustJSON(doc)
		if doc.ExcludedCountsByReason[ReasonOriginUnproven] != 1 || strings.Contains(raw, load.ID.String()) {
			t.Fatalf("%s", raw)
		}
	})
	t.Run("NLO03B_017_DESTINATION_IDENTITY_UNPROVEN_EXCLUDED", func(t *testing.T) {
		w := newWorld(t)
		cap := w.readyCapacity()
		win := span(w.at, w.at.Add(time.Hour))
		load := w.flagged(w.shipper, domain.VisMarketplace, nil, uuid.New(), uuid.New(), true, false, win, win, f64(10), domain.CargoConstraints{})
		load.Delivery.LocationID = nil
		replaceLoad(t, w, load)
		doc := w.consolidate(w.actor(), cap.ID, nil)
		if doc.ExcludedCountsByReason[ReasonDestinationUnproven] != 1 || doc.EvaluatedPairCount != 0 {
			t.Fatalf("%+v", doc.ExcludedCountsByReason)
		}
	})
	t.Run("NLO03B_018_ANON_EXACT_LOCATION_ID_NOT_EXPOSED", func(t *testing.T) {
		w := newWorld(t)
		cap := w.readyCapacity()
		origin, dest := uuid.New(), uuid.New()
		win := span(w.at, w.at.Add(2*time.Hour))
		left := located(origin, "Origin")
		left.Latitude = f64(55.123456)
		left.Longitude = f64(37.654321)
		w.insertFlagged(w.shipper, domain.VisAnonymized, nil, left, located(dest, "Dest"), true, false, win, win, f64(10), domain.CargoConstraints{}, uuid.New())
		w.insertFlagged(w.shipper, domain.VisAnonymized, nil, located(origin, "Origin"), located(dest, "Dest"), true, false, win, win, f64(10), domain.CargoConstraints{}, uuid.New())
		doc := w.consolidate(w.actor(), cap.ID, nil)
		raw := mustJSON(doc)
		if doc.FeasibleCandidateCount != 1 || strings.Contains(raw, origin.String()) || strings.Contains(raw, "55.123456") || strings.Contains(raw, w.shipper.String()) {
			t.Fatalf("%s", raw)
		}
	})

	t.Run("NLO03B_019_PICKUP_WINDOW_OVERLAP", func(t *testing.T) {
		doc := feasiblePair(t, true)
		if doc.Candidates[0].PickupWindowOverlap == nil || !doc.Candidates[0].PickupWindowOverlap.End.After(*doc.Candidates[0].PickupWindowOverlap.Start) {
			t.Fatal("missing pickup overlap")
		}
	})
	t.Run("NLO03B_020_DELIVERY_WINDOW_OVERLAP", func(t *testing.T) {
		doc := feasiblePair(t, true)
		if doc.Candidates[0].DeliveryWindowOverlap == nil {
			t.Fatal("missing delivery overlap")
		}
	})
	t.Run("NLO03B_021_PICKUP_WINDOWS_DISJOINT_HARD_REJECT", func(t *testing.T) {
		w, cap := windowPair(t, span(wAt(t), wAt(t).Add(time.Hour)), span(wAt(t).Add(3*time.Hour), wAt(t).Add(4*time.Hour)), span(wAt(t), wAt(t).Add(4*time.Hour)), span(wAt(t), wAt(t).Add(4*time.Hour)))
		doc := w.consolidate(w.actor(), cap.ID, nil)
		if doc.HardRejectCountsByReason[ReasonPickupDisjoint] != 1 || len(doc.Candidates) != 0 {
			t.Fatalf("%+v", doc)
		}
	})
	t.Run("NLO03B_022_DELIVERY_WINDOWS_DISJOINT_HARD_REJECT", func(t *testing.T) {
		w, cap := windowPair(t, span(wAt(t), wAt(t).Add(4*time.Hour)), span(wAt(t), wAt(t).Add(4*time.Hour)), span(wAt(t), wAt(t).Add(time.Hour)), span(wAt(t).Add(2*time.Hour), wAt(t).Add(3*time.Hour)))
		doc := w.consolidate(w.actor(), cap.ID, nil)
		if doc.HardRejectCountsByReason[ReasonDeliveryDisjoint] != 1 || len(doc.Candidates) != 0 {
			t.Fatalf("%+v", doc)
		}
	})
	t.Run("NLO03B_023_PICKUP_WINDOW_UNKNOWN_INDETERMINATE", func(t *testing.T) {
		w, cap := windowPair(t, domain.TimeWindow{}, span(wAt(t), wAt(t).Add(time.Hour)), span(wAt(t), wAt(t).Add(time.Hour)), span(wAt(t), wAt(t).Add(time.Hour)))
		doc := w.consolidate(w.actor(), cap.ID, nil)
		if doc.IndeterminateCandidateCount != 1 || !has(doc.Candidates[0].IndeterminateReasonCodes, ReasonPickupUnknown) || doc.Candidates[0].Status == ConsolidationFeasible {
			t.Fatalf("%+v", doc.Candidates)
		}
	})
	t.Run("NLO03B_024_DELIVERY_WINDOW_UNKNOWN_INDETERMINATE", func(t *testing.T) {
		w, cap := windowPair(t, span(wAt(t), wAt(t).Add(time.Hour)), span(wAt(t), wAt(t).Add(time.Hour)), domain.TimeWindow{}, span(wAt(t), wAt(t).Add(time.Hour)))
		doc := w.consolidate(w.actor(), cap.ID, nil)
		if !has(doc.Candidates[0].IndeterminateReasonCodes, ReasonDeliveryUnknown) {
			t.Fatalf("%+v", doc.Candidates[0].IndeterminateReasonCodes)
		}
	})
	t.Run("NLO03B_025_BOUNDARY_TOUCH_IS_NOT_POSITIVE_OVERLAP", func(t *testing.T) {
		touch := wAt(t).Add(time.Hour)
		w, cap := windowPair(t, span(wAt(t), touch), span(touch, touch.Add(time.Hour)), span(wAt(t), wAt(t).Add(4*time.Hour)), span(wAt(t), wAt(t).Add(4*time.Hour)))
		doc := w.consolidate(w.actor(), cap.ID, nil)
		if doc.HardRejectCountsByReason[ReasonPickupDisjoint] != 1 || doc.FeasibleCandidateCount != 0 {
			t.Fatalf("%+v", doc)
		}
	})

	t.Run("NLO03B_045_FOREIGN_CAPACITY_NOT_FOUND", func(t *testing.T) {
		w := newWorld(t)
		cap := w.readyCapacity()
		foreign := serviceActor(uuid.New(), nil)
		_, missing := w.svc.SearchConsolidation(context.Background(), w.actor(), ConsolidationCommand{CapacityID: uuid.New(), Pattern: PatternSameOriginDestination})
		_, other := w.svc.SearchConsolidation(context.Background(), foreign, ConsolidationCommand{CapacityID: cap.ID, Pattern: PatternSameOriginDestination})
		if !notFound(missing) || !notFound(other) {
			t.Fatalf("missing %v other %v", missing, other)
		}
	})
	t.Run("NLO03B_046_WITHDRAWN_CAPACITY_REJECTED", func(t *testing.T) {
		w := newWorld(t)
		cap := w.capacity(domain.CapacityWithdrawn, domain.SourceManual, 0, 0)
		_, err := w.svc.SearchConsolidation(context.Background(), w.actor(), ConsolidationCommand{CapacityID: cap.ID, Pattern: PatternSameOriginDestination})
		var app *apperrors.AppError
		if !errors.As(err, &app) || app.Details["reason"] != "CAPACITY_NOT_AVAILABLE" {
			t.Fatalf("%v", err)
		}
	})
	t.Run("NLO03B_047_PREDICTED_CAPACITY_EFFECTIVE_EQUIPMENT", func(t *testing.T) {
		w := newWorld(t)
		cap := w.capacity(domain.CapacityAvailable, domain.SourceCurrentShipmentPrediction, 0, 0)
		if err := w.store.Within(context.Background(), func(tx repository.Tx) error {
			return tx.InsertPrediction(context.Background(), domain.PredictedCapacity{
				ID: uuid.New(), CapacityID: cap.ID, OwnerTenantID: w.carrier, IsCurrent: true,
				PredictedAvailableAt: w.at, AvailabilityWindowStart: w.at, AvailabilityWindowEnd: w.at.Add(time.Hour),
				CapacityWeightKg: f64(2000), LoadingAccess: []string{"REAR"},
			})
		}); err != nil {
			t.Fatal(err)
		}
		origin, dest := uuid.New(), uuid.New()
		win := span(w.at, w.at.Add(2*time.Hour))
		w.flagged(w.shipper, domain.VisMarketplace, nil, origin, dest, true, false, win, win, f64(400), domain.CargoConstraints{})
		w.flagged(w.shipper, domain.VisMarketplace, nil, origin, dest, true, false, win, win, f64(400), domain.CargoConstraints{})
		doc := w.consolidate(w.actor(), cap.ID, nil)
		if doc.FeasibleCandidateCount != 1 || doc.Candidates[0].CapacityUsage == nil || doc.Candidates[0].CapacityUsage.WeightUsed == nil || *doc.Candidates[0].CapacityUsage.WeightUsed != 800 {
			t.Fatalf("%+v", doc)
		}
	})
	t.Run("NLO03B_048_RAW_CROSS_TENANT_TRANSPORT_SCAN_NOT_USED", func(t *testing.T) {
		raw, err := os.ReadFile("consolidation.go")
		if err != nil {
			t.Fatal(err)
		}
		text := string(raw)
		for _, forbidden := range []string{"transport.shipments", "transport.transport_orders", "transport.cargoes"} {
			if strings.Contains(text, forbidden) {
				t.Fatal(forbidden)
			}
		}
	})
	t.Run("NLO03B_049_NETWORK_OPTIMIZATION_ONLY_HIDDEN_FROM_PUBLIC_SEARCH", func(t *testing.T) {
		w := newWorld(t)
		cap := w.readyCapacity()
		origin, dest := uuid.New(), uuid.New()
		win := span(w.at, w.at.Add(time.Hour))
		hidden := w.flagged(w.shipper, domain.VisNetworkOnly, nil, origin, dest, true, true, win, win, f64(10), domain.CargoConstraints{})
		w.flagged(w.shipper, domain.VisNetworkOnly, nil, origin, dest, true, true, win, win, f64(10), domain.CargoConstraints{})
		doc := w.consolidate(w.actor(), cap.ID, nil)
		listed, err := w.svc.ListMarketplaceLoads(context.Background(), w.actor(), 20, 0)
		if err != nil {
			t.Fatal(err)
		}
		if doc.PoolLoadCount != 0 || strings.Contains(string(listed.Body), hidden.ID.String()) {
			t.Fatalf("pool %d list %s", doc.PoolLoadCount, listed.Body)
		}
	})
	t.Run("NLO03B_050_PRIVATE_LOAD_HIDDEN_FROM_PUBLIC_SEARCH", func(t *testing.T) {
		w := newWorld(t)
		cap := w.readyCapacity()
		win := span(w.at, w.at.Add(time.Hour))
		w.flagged(w.shipper, domain.VisPrivate, nil, uuid.New(), uuid.New(), true, true, win, win, f64(10), domain.CargoConstraints{})
		w.flagged(w.shipper, domain.VisPrivate, nil, uuid.New(), uuid.New(), true, true, win, win, f64(10), domain.CargoConstraints{})
		doc := w.consolidate(w.actor(), cap.ID, nil)
		if doc.PoolLoadCount != 0 {
			t.Fatalf("%+v", doc)
		}
	})
	t.Run("NLO03B_051_INVITED_LOAD_REQUIRES_ACTOR_COMPANY", func(t *testing.T) {
		w := newWorld(t)
		cap := w.readyCapacity()
		company := uuid.New()
		origin, dest := uuid.New(), uuid.New()
		win := span(w.at, w.at.Add(time.Hour))
		w.flagged(w.shipper, domain.VisInvited, []uuid.UUID{company}, origin, dest, true, false, win, win, f64(10), domain.CargoConstraints{})
		w.flagged(w.shipper, domain.VisInvited, []uuid.UUID{company}, origin, dest, true, false, win, win, f64(10), domain.CargoConstraints{})
		if hidden := w.consolidate(w.actor(), cap.ID, nil); hidden.PoolLoadCount != 0 {
			t.Fatalf("uninvited %+v", hidden)
		}
		actor := serviceActor(w.carrier, &company)
		doc := w.consolidate(actor, cap.ID, nil)
		if doc.PoolLoadCount != 2 || doc.FeasibleCandidateCount != 1 {
			t.Fatalf("%+v", doc)
		}
	})

	t.Run("NLO03B_052_PAIR_A_B_ONLY_ONCE", func(t *testing.T) {
		doc := feasiblePair(t, true)
		if doc.EvaluatedPairCount != 1 || doc.ReturnedCandidateCount != 1 {
			t.Fatalf("%+v", doc)
		}
	})
	t.Run("NLO03B_053_DETERMINISTIC_PAIR_ORDER", func(t *testing.T) {
		w := newWorld(t)
		cap := w.readyCapacity()
		origin, dest := uuid.New(), uuid.New()
		win := span(w.at, w.at.Add(2*time.Hour))
		for i := 0; i < 3; i++ {
			w.flagged(w.shipper, domain.VisMarketplace, nil, origin, dest, true, false, win, win, f64(10), domain.CargoConstraints{})
		}
		doc := w.consolidate(w.actor(), cap.ID, nil)
		if doc.EvaluatedPairCount != 3 || len(doc.Candidates) != 3 {
			t.Fatalf("%+v", doc)
		}
		for i := 1; i < len(doc.Candidates); i++ {
			prev := doc.Candidates[i-1].Members[0].LoadOpportunityID.String() + doc.Candidates[i-1].Members[1].LoadOpportunityID.String()
			next := doc.Candidates[i].Members[0].LoadOpportunityID.String() + doc.Candidates[i].Members[1].LoadOpportunityID.String()
			if prev > next || doc.Candidates[i].Members[0].LoadOpportunityID.String() > doc.Candidates[i].Members[1].LoadOpportunityID.String() {
				t.Fatalf("order %+v", doc.Candidates)
			}
		}
	})
	t.Run("NLO03B_054_ONLY_SAME_OD_GROUPS_COMPARED", func(t *testing.T) {
		w := newWorld(t)
		cap := w.readyCapacity()
		win := span(w.at, w.at.Add(time.Hour))
		for g := 0; g < 2; g++ {
			origin, dest := uuid.New(), uuid.New()
			w.flagged(w.shipper, domain.VisMarketplace, nil, origin, dest, true, false, win, win, f64(10), domain.CargoConstraints{})
			w.flagged(w.shipper, domain.VisMarketplace, nil, origin, dest, true, false, win, win, f64(10), domain.CargoConstraints{})
		}
		doc := w.consolidate(w.actor(), cap.ID, nil)
		if doc.PoolLoadCount != 4 || doc.EvaluatedPairCount != 2 {
			t.Fatalf("%+v", doc)
		}
	})
	t.Run("NLO03B_055_SET_SIZE_ALWAYS_TWO", func(t *testing.T) {
		doc := feasiblePair(t, true)
		if len(doc.Candidates[0].Members) != 2 {
			t.Fatal(len(doc.Candidates[0].Members))
		}
	})
	t.Run("NLO03B_056_CANDIDATE_LIMIT_AFTER_EVALUATION", func(t *testing.T) {
		w := newWorld(t)
		cap := w.readyCapacity()
		origin, dest := uuid.New(), uuid.New()
		win := span(w.at, w.at.Add(2*time.Hour))
		for i := 0; i < 3; i++ {
			w.flagged(w.shipper, domain.VisMarketplace, nil, origin, dest, true, false, win, win, f64(10), domain.CargoConstraints{})
		}
		limit := 1
		doc := w.consolidate(w.actor(), cap.ID, &limit)
		if doc.EvaluatedPairCount != 3 || doc.FeasibleCandidateCount != 3 || doc.ReturnedCandidateCount != 1 {
			t.Fatalf("%+v", doc)
		}
		_, rows, err := w.store.GetConsolidation(context.Background(), w.carrier, doc.SearchID)
		if err != nil || len(rows) != 3 {
			t.Fatalf("persisted %d %v", len(rows), err)
		}
	})
	t.Run("NLO03B_057_CANDIDATE_LIMIT_ZERO", func(t *testing.T) {
		w, cap, _, _ := sameOwnerPair(t)
		limit := 0
		doc := w.consolidate(w.actor(), cap.ID, &limit)
		if doc.ReturnedCandidateCount != 0 || doc.FeasibleCandidateCount != 1 || len(doc.Candidates) != 0 {
			t.Fatalf("%+v", doc)
		}
	})
	t.Run("NLO03B_058_HARD_REJECT_NOT_RETURNED_AS_FEASIBLE", func(t *testing.T) {
		w := newWorld(t)
		cap := w.readyCapacity()
		origin, dest := uuid.New(), uuid.New()
		win := span(w.at, w.at.Add(time.Hour))
		w.flagged(w.shipper, domain.VisMarketplace, nil, origin, dest, true, false, win, win, f64(800), domain.CargoConstraints{})
		w.flagged(w.shipper, domain.VisMarketplace, nil, origin, dest, true, false, win, win, f64(800), domain.CargoConstraints{})
		doc := w.consolidate(w.actor(), cap.ID, nil)
		raw := mustJSON(doc)
		if doc.HardRejectCandidateCount != 1 || doc.FeasibleCandidateCount != 0 || strings.Contains(raw, `"status":"FEASIBLE"`) || strings.Contains(raw, `"status":"HARD_REJECT"`) {
			t.Fatalf("%s", raw)
		}
	})

	t.Run("NLO03B_067_OTHER_SHIPPER_SOURCE_ID_NOT_EXPOSED", func(t *testing.T) {
		w, cap, left, _ := sameOwnerPair(t)
		doc := w.consolidate(w.actor(), cap.ID, nil)
		if strings.Contains(mustJSON(doc), left.SourceID.String()) {
			t.Fatal("source id leaked")
		}
	})
	t.Run("NLO03B_068_ANON_OWNER_ID_NOT_EXPOSED", func(t *testing.T) {
		w := newWorld(t)
		cap := w.readyCapacity()
		origin, dest := uuid.New(), uuid.New()
		win := span(w.at, w.at.Add(time.Hour))
		w.flagged(w.shipper, domain.VisAnonymized, nil, origin, dest, true, false, win, win, f64(10), domain.CargoConstraints{})
		w.flagged(w.shipper, domain.VisAnonymized, nil, origin, dest, true, false, win, win, f64(10), domain.CargoConstraints{})
		doc := w.consolidate(w.actor(), cap.ID, nil)
		if strings.Contains(mustJSON(doc), w.shipper.String()) || strings.Contains(mustJSON(doc), `"owner_tenant_id"`) {
			t.Fatal(mustJSON(doc))
		}
	})
	t.Run("NLO03B_069_ANON_EXACT_GEO_NOT_EXPOSED", func(t *testing.T) {
		w := newWorld(t)
		cap := w.readyCapacity()
		origin, dest := uuid.New(), uuid.New()
		win := span(w.at, w.at.Add(time.Hour))
		place := located(origin, "City")
		place.Latitude = f64(48.111111)
		place.Longitude = f64(11.222222)
		w.insertFlagged(w.shipper, domain.VisAnonymized, nil, place, located(dest, "Other"), true, false, win, win, f64(10), domain.CargoConstraints{}, uuid.New())
		w.flagged(w.shipper, domain.VisAnonymized, nil, origin, dest, true, false, win, win, f64(10), domain.CargoConstraints{})
		doc := w.consolidate(w.actor(), cap.ID, nil)
		raw := mustJSON(doc)
		if strings.Contains(raw, "48.111111") || strings.Contains(raw, "11.222222") || strings.Contains(raw, origin.String()) {
			t.Fatal(raw)
		}
	})
	t.Run("NLO03B_070_INTERNAL_MEMBER_OWNER_NOT_PUBLIC", func(t *testing.T) {
		doc := feasiblePair(t, true)
		if strings.Contains(mustJSON(doc), "load_owner_tenant_id") {
			t.Fatal(mustJSON(doc))
		}
	})
	t.Run("NLO03B_071_PUBLIC_COMPAT_TRACE_TENANT_IDS_SANITIZED", func(t *testing.T) {
		w, cap, _, _ := sameOwnerPair(t)
		secret := uuid.New()
		source := "source-" + secret.String()
		w.svc.UseCatalog(traceCatalog{tenant: secret.String(), source: source})
		doc := w.consolidate(w.actor(), cap.ID, nil)
		_, rows, err := w.store.GetConsolidation(context.Background(), w.carrier, doc.SearchID)
		if err != nil || len(rows) != 1 {
			t.Fatal(err)
		}
		raw := mustJSON(doc) + string(rows[0].CompatibilityTrace)
		if strings.Contains(raw, secret.String()) || strings.Contains(raw, source) {
			t.Fatalf("trace leaked %s", raw)
		}
	})

	t.Run("NLO03B_072_EXECUTION_SUPPORTED_ALWAYS_FALSE", func(t *testing.T) {
		doc := feasiblePair(t, true)
		if doc.Candidates[0].ExecutionSupported || strings.Contains(mustJSON(doc), `"execution_supported":true`) {
			t.Fatal(mustJSON(doc))
		}
	})
	t.Run("NLO03B_073_NO_SHIPMENT_CREATED", func(t *testing.T) {
		raw, err := os.ReadFile("consolidation.go")
		if err != nil {
			t.Fatal(err)
		}
		if strings.Contains(string(raw), "InsertShipment") || strings.Contains(string(raw), "shipment-service") {
			t.Fatal("shipment mutation")
		}
	})
	t.Run("NLO03B_074_NO_ASSIGNMENT_CREATED", func(t *testing.T) {
		doc := feasiblePair(t, true)
		if strings.Contains(mustJSON(doc), "ASSIGNED") {
			t.Fatal(mustJSON(doc))
		}
	})
	t.Run("NLO03B_075_NO_RESERVATION_CREATED", func(t *testing.T) {
		doc := feasiblePair(t, true)
		if strings.Contains(mustJSON(doc), "reservation") {
			t.Fatal(mustJSON(doc))
		}
	})
	t.Run("NLO03B_076_PLACEMENT_NOT_EVALUATED", func(t *testing.T) {
		doc := feasiblePair(t, true)
		if doc.Candidates[0].PlacementCheck != PlacementNotEvaluated {
			t.Fatal(doc.Candidates[0].PlacementCheck)
		}
	})
	t.Run("NLO03B_PATTERN_NOT_IMPLEMENTED", func(t *testing.T) {
		w := newWorld(t)
		cap := w.readyCapacity()
		_, err := w.svc.SearchConsolidation(context.Background(), w.actor(), ConsolidationCommand{CapacityID: cap.ID, Pattern: PatternCurrentTripFill})
		var app *apperrors.AppError
		if !errors.As(err, &app) || app.Details["reason"] != "PATTERN_NOT_IMPLEMENTED" {
			t.Fatalf("%v", err)
		}
	})
}

func TestNLO03BPersistence(t *testing.T) {
	w, cap, left, right := sameOwnerPair(t)
	doc := w.consolidate(w.actor(), cap.ID, nil)
	run, rows, err := w.store.GetConsolidation(context.Background(), w.carrier, doc.SearchID)
	if err != nil {
		t.Fatal(err)
	}
	t.Run("NLO03B_059_SEARCH_RUN_PERSISTED", func(t *testing.T) {
		if run.Status != "COMPLETED" || run.CapacityID != cap.ID || run.Pattern != PatternSameOriginDestination || run.EvaluatedPairCount != 1 {
			t.Fatalf("%+v", run)
		}
	})
	t.Run("NLO03B_060_PAIR_MEMBERS_PERSISTED", func(t *testing.T) {
		if len(rows) != 1 || len(rows[0].Members) != 2 {
			t.Fatalf("%+v", rows)
		}
	})
	t.Run("NLO03B_062_CAPACITY_VERSION_PINNED", func(t *testing.T) {
		if run.CapacityVersion != cap.Version {
			t.Fatal(run.CapacityVersion)
		}
	})
	t.Run("NLO03B_063_COMPATIBILITY_FINGERPRINT_PERSISTED", func(t *testing.T) {
		if rows[0].CompatibilityFingerprint == "" || rows[0].CompatibilityFingerprint == "NOT_EVALUATED" {
			t.Fatal(rows[0].CompatibilityFingerprint)
		}
	})
	t.Run("NLO03B_064_CANDIDATE_FINGERPRINT_DETERMINISTIC", func(t *testing.T) {
		second := w.consolidate(w.actor(), cap.ID, nil)
		_, more, err := w.store.GetConsolidation(context.Background(), w.carrier, second.SearchID)
		if err != nil || more[0].CandidateFingerprint != rows[0].CandidateFingerprint || more[0].ID == rows[0].ID {
			t.Fatalf("%s %s", more[0].CandidateFingerprint, rows[0].CandidateFingerprint)
		}
	})
	t.Run("NLO03B_061_LOAD_VERSIONS_PINNED", func(t *testing.T) {
		if rows[0].Members[0].LoadVersion != left.Version || rows[0].Members[1].LoadVersion != right.Version {
			t.Fatalf("%+v", rows[0].Members)
		}
		left.Version = 2
		left.WeightKg = f64(50)
		replaceLoad(t, w, left)
		_, again, err := w.store.GetConsolidation(context.Background(), w.carrier, doc.SearchID)
		if err != nil || again[0].Members[0].LoadVersion != 1 {
			t.Fatalf("%+v %v", again, err)
		}
	})
	t.Run("NLO03B_065_TWO_MEMBER_UNIQUENESS", func(t *testing.T) {
		if rows[0].Members[0].Ordinal != 1 || rows[0].Members[1].Ordinal != 2 || rows[0].Members[0].LoadOpportunityID == rows[0].Members[1].LoadOpportunityID {
			t.Fatalf("%+v", rows[0].Members)
		}
	})
	t.Run("NLO03B_066_SEARCH_PERSISTENCE_ATOMIC", func(t *testing.T) {
		w.svc.consolidations = boomStore{}
		_, err := w.svc.SearchConsolidation(context.Background(), w.actor(), ConsolidationCommand{CapacityID: cap.ID, Pattern: PatternSameOriginDestination})
		if err == nil {
			t.Fatal("expected save failure")
		}
	})
}

func TestNLO03BGroupage(t *testing.T) {
	equipment := compat.Equipment{PayloadKg: f64(1000), PalletPositions: intPtr(10), UsableLinearMeters: f64(13.6), InternalHeightMM: intPtr(2700), TemperatureMinC: f64(-20), TemperatureMaxC: f64(20), TemperatureControlMode: strPtr("ACTIVE"), ADRCapability: boolPtr(true), FoodGradeCapability: boolPtr(true), LoadingAccess: []string{"REAR"}, UnloadingAccess: []string{"REAR"}}
	t.Run("NLO03B_026_PAIR_WEIGHT_AGGREGATED", func(t *testing.T) {
		got := pairItems(equipment, cargo("a", 400), cargo("b", 500), compat.AccessNeed{}, compat.Context{})
		if got.Status != compat.StatusCompatible || got.CapacityUsage == nil || *got.CapacityUsage.WeightUsed != 900 {
			t.Fatalf("%+v", got.CapacityUsage)
		}
	})
	t.Run("NLO03B_027_PAIR_VOLUME_AGGREGATED", func(t *testing.T) {
		left, right := cargo("a", 10), cargo("b", 10)
		left.VolumeM3, right.VolumeM3 = f64(6), f64(6)
		withVolume := equipment
		withVolume.VolumeM3 = f64(20)
		got := pairItems(withVolume, left, right, compat.AccessNeed{}, compat.Context{})
		if *got.CapacityUsage.VolumeUsed != 12 {
			t.Fatal(*got.CapacityUsage.VolumeUsed)
		}
	})
	t.Run("NLO03B_028_PAIR_PALLETS_AGGREGATED", func(t *testing.T) {
		left, right := cargo("a", 10), cargo("b", 10)
		left.PalletCount, right.PalletCount = intPtr(3), intPtr(4)
		left.PalletTypeCode, right.PalletTypeCode = strPtr("EUR"), strPtr("EUR")
		basis := equipment
		basis.PalletBasisCode = strPtr("EUR")
		got := pairItems(basis, left, right, compat.AccessNeed{}, compat.Context{})
		if got.Status != compat.StatusCompatible || *got.CapacityUsage.PalletsUsed != 7 {
			t.Fatalf("%s %+v", got.Status, got.CapacityUsage)
		}
	})
	t.Run("NLO03B_029_UNKNOWN_PALLETS_NOT_ZERO", func(t *testing.T) {
		left, right := cargo("a", 10), cargo("b", 10)
		left.PalletCount, left.PalletTypeCode = intPtr(3), strPtr("EUR")
		right.PalletTypeCode = strPtr("EUR")
		got := pairItems(equipment, left, right, compat.AccessNeed{}, compat.Context{})
		if got.Status != compat.StatusIndeterminate || !reason(got, "PALLET_COUNT_UNKNOWN") || (got.CapacityUsage.PalletsUsed != nil && *got.CapacityUsage.PalletsUsed == 0) {
			t.Fatalf("%+v", got)
		}
	})
	t.Run("NLO03B_030_PAIR_LINEAR_METRES_AGGREGATED", func(t *testing.T) {
		left, right := cargo("a", 10), cargo("b", 10)
		left.LinearMeters, right.LinearMeters = f64(4), f64(5)
		got := pairItems(equipment, left, right, compat.AccessNeed{}, compat.Context{})
		if *got.CapacityUsage.LinearMetersUsed != 9 {
			t.Fatal(*got.CapacityUsage.LinearMetersUsed)
		}
	})
	t.Run("NLO03B_031_TEMPERATURE_COMMON_INTERSECTION", func(t *testing.T) {
		left, right := cargo("a", 10), cargo("b", 10)
		left.TemperatureMinC, left.TemperatureMaxC = f64(2), f64(8)
		right.TemperatureMinC, right.TemperatureMaxC = f64(4), f64(6)
		got := pairItems(equipment, left, right, compat.AccessNeed{}, compat.Context{})
		if got.Temperature == nil || *got.Temperature.CommonMinC != 4 || *got.Temperature.CommonMaxC != 6 || got.Status != compat.StatusCompatible {
			t.Fatalf("%+v %s", got.Temperature, got.Status)
		}
	})
	t.Run("NLO03B_032_TEMPERATURE_CONFLICT_HARD_REJECT", func(t *testing.T) {
		left, right := cargo("a", 10), cargo("b", 10)
		left.TemperatureMinC, left.TemperatureMaxC = f64(2), f64(4)
		right.TemperatureMinC, right.TemperatureMaxC = f64(8), f64(10)
		got := pairItems(equipment, left, right, compat.AccessNeed{}, compat.Context{})
		if got.Status != compat.StatusIncompatible || !reason(got, "TEMPERATURE_RANGES_INCOMPATIBLE") {
			t.Fatal(got.Status)
		}
	})
	t.Run("NLO03B_033_MULTI_ZONE_INDETERMINATE", func(t *testing.T) {
		left, right := cargo("a", 10), cargo("b", 10)
		left.TemperatureMinC, left.TemperatureMaxC = f64(2), f64(4)
		right.TemperatureMinC, right.TemperatureMaxC = f64(8), f64(10)
		zoned := equipment
		zoned.TemperatureZoneCount = intPtr(2)
		zoned.IndependentTemperatureControl = boolPtr(true)
		got := pairItems(zoned, left, right, compat.AccessNeed{}, compat.Context{})
		if got.Status != compat.StatusIndeterminate || !reason(got, "MULTI_ZONE_ALLOCATION_REQUIRED") {
			t.Fatalf("%s", got.Status)
		}
	})
	t.Run("NLO03B_034_ADR_FALSE_HARD_REJECT", func(t *testing.T) {
		left := cargo("a", 10)
		left.DangerousGoods = boolPtr(true)
		denied := equipment
		denied.ADRCapability = boolPtr(false)
		got := pairItems(denied, left, cargo("b", 10), compat.AccessNeed{}, compat.Context{})
		if got.Status != compat.StatusIncompatible || !reason(got, "ADR_INCOMPATIBLE") {
			t.Fatal(got.Status)
		}
	})
	t.Run("NLO03B_035_ADR_UNKNOWN_INDETERMINATE", func(t *testing.T) {
		left := cargo("a", 10)
		left.DangerousGoods = boolPtr(true)
		unknown := equipment
		unknown.ADRCapability = nil
		got := pairItems(unknown, left, cargo("b", 10), compat.AccessNeed{}, compat.Context{})
		if got.Status != compat.StatusIndeterminate || !reason(got, "ADR_CAPABILITY_UNKNOWN") {
			t.Fatal(got.Status)
		}
	})
	t.Run("NLO03B_036_ADR_RULE_UNAVAILABLE_INDETERMINATE", func(t *testing.T) {
		left := cargo("a", 10)
		left.DangerousGoods = boolPtr(true)
		got := pairItems(equipment, left, cargo("b", 10), compat.AccessNeed{}, compat.Context{})
		if got.Status != compat.StatusIndeterminate || !reason(got, "ADR_COMPATIBILITY_RULE_UNAVAILABLE") {
			t.Fatal(got.Status)
		}
	})
	t.Run("NLO03B_037_FOOD_GRADE_REUSED", func(t *testing.T) {
		left := cargo("a", 10)
		left.FoodGradeRequired = boolPtr(true)
		plain := equipment
		plain.FoodGradeCapability = boolPtr(false)
		got := pairItems(plain, left, cargo("b", 10), compat.AccessNeed{}, compat.Context{})
		if !reason(got, "FOOD_GRADE_REQUIRED") || got.Status != compat.StatusIncompatible {
			t.Fatal(got.Status)
		}
	})
	t.Run("NLO03B_038_ODOR_RULE_REUSED", func(t *testing.T) {
		left, right := cargo("a", 10), cargo("b", 10)
		left.OdorEmissionClass = strPtr("STRONG")
		right.OdorSensitive = boolPtr(true)
		rule := compat.Rule{RuleCode: "ODOR", RuleKind: compat.KindCargoCargo, Layer: compat.LayerPlatform, LeftSelectorType: "ODOR_EMISSION_CLASS", LeftSelectorValue: "STRONG", RightSelectorType: "ODOR_SENSITIVE", RightSelectorValue: "true", Decision: compat.DecisionDeny, ReasonCode: "ODOR_INCOMPATIBLE", Severity: compat.SeverityHard}
		got := pairItems(equipment, left, right, compat.AccessNeed{}, compat.Context{Rules: []compat.Rule{rule}})
		if !reason(got, "ODOR_INCOMPATIBLE") {
			t.Fatal(got.Status)
		}
	})
	t.Run("NLO03B_039_CONTAMINATION_RULE_REUSED", func(t *testing.T) {
		left, right := cargo("a", 10), cargo("b", 10)
		left.ContaminationClass = strPtr("HIGH")
		right.ContaminationClass = strPtr("FOOD")
		rule := compat.Rule{RuleCode: "CONTAM", RuleKind: compat.KindCargoCargo, Layer: compat.LayerPlatform, LeftSelectorType: "CONTAMINATION_CLASS", LeftSelectorValue: "HIGH", RightSelectorType: "CONTAMINATION_CLASS", RightSelectorValue: "FOOD", Decision: compat.DecisionDeny, ReasonCode: "CONTAMINATION_COMPATIBILITY_RULE", Severity: compat.SeverityHard}
		got := pairItems(equipment, left, right, compat.AccessNeed{}, compat.Context{Rules: []compat.Rule{rule}})
		if !reason(got, "CONTAMINATION_COMPATIBILITY_RULE") {
			t.Fatal(got.Status)
		}
	})
	t.Run("NLO03B_040_REQUIRE_SEPARATION_INDETERMINATE", func(t *testing.T) {
		rule := compat.Rule{RuleCode: "SEP", RuleKind: compat.KindCargoCargo, Layer: compat.LayerRegulatory, LeftSelectorType: "ANY", RightSelectorType: "ANY", Decision: compat.DecisionRequireSeparation, ReasonCode: "PHYSICAL_PARTITION_REQUIRED", Severity: compat.SeverityHard}
		got := pairItems(equipment, cargo("a", 10), cargo("b", 10), compat.AccessNeed{}, compat.Context{Rules: []compat.Rule{rule}})
		if got.Status != compat.StatusIndeterminate || !reason(got, "PHYSICAL_PARTITION_REQUIRED") {
			t.Fatal(got.Status)
		}
	})
	t.Run("NLO03B_041_PER_LOAD_REQUIRED_LOADING_ACCESS", func(t *testing.T) {
		need := compat.AccessNeed{RequiredLoadingAccess: []string{"SIDE"}}
		got := pairItems(equipment, cargo("a", 10), cargo("b", 10), need, compat.Context{})
		if got.Status != compat.StatusIncompatible || !reason(got, "LOADING_ACCESS_UNSUPPORTED") {
			t.Fatal(got.Status)
		}
	})
	t.Run("NLO03B_042_PER_LOAD_ALLOWED_LOADING_ACCESS", func(t *testing.T) {
		items := []compat.GroupageItem{
			{Cargo: cargo("a", 10), AccessNeed: compat.AccessNeed{AllowedLoadingAccess: []string{"REAR"}}},
			{Cargo: cargo("b", 10), AccessNeed: compat.AccessNeed{AllowedLoadingAccess: []string{"SIDE"}}},
		}
		got := compat.EvaluateGroupageItems(equipment, items, compat.Context{})
		union := compat.EvaluateGroupage(equipment, []compat.Cargo{items[0].Cargo, items[1].Cargo}, compat.AccessNeed{AllowedLoadingAccess: []string{"REAR", "SIDE"}}, compat.Context{})
		if got.Status != compat.StatusIncompatible || union.Status == compat.StatusIncompatible {
			t.Fatalf("per-load %s union %s", got.Status, union.Status)
		}
	})
	t.Run("NLO03B_043_PER_LOAD_REQUIRED_UNLOADING_ACCESS", func(t *testing.T) {
		items := []compat.GroupageItem{
			{Cargo: cargo("a", 10), AccessNeed: compat.AccessNeed{RequiredUnloadingAccess: []string{"SIDE"}}},
			{Cargo: cargo("b", 10), AccessNeed: compat.AccessNeed{}},
		}
		got := compat.EvaluateGroupageItems(equipment, items, compat.Context{})
		if !reason(got, "UNLOADING_ACCESS_UNSUPPORTED") {
			t.Fatal(got.Status)
		}
	})
	t.Run("NLO03B_044_PER_LOAD_ALLOWED_UNLOADING_ACCESS", func(t *testing.T) {
		items := []compat.GroupageItem{
			{Cargo: cargo("a", 10), AccessNeed: compat.AccessNeed{AllowedUnloadingAccess: []string{"REAR"}}},
			{Cargo: cargo("b", 10), AccessNeed: compat.AccessNeed{AllowedUnloadingAccess: []string{"SIDE"}}},
		}
		got := compat.EvaluateGroupageItems(equipment, items, compat.Context{})
		if got.Status != compat.StatusIncompatible {
			t.Fatal(got.Status)
		}
	})
}

func TestNLO03BPerformanceFixtures(t *testing.T) {
	for _, n := range []int{10, 50, 100, 500} {
		n := n
		t.Run(fmt.Sprintf("FIXTURE_%d", n), func(t *testing.T) {
			w := newWorld(t)
			cap := w.readyCapacity()
			win := span(w.at, w.at.Add(4*time.Hour))
			groups := 2
			if n >= 50 {
				groups = 5
			}
			if n >= 100 {
				groups = 10
			}
			for i := 0; i < n; i++ {
				origin := uuid.UUID{}
				dest := uuid.UUID{}
				origin[0] = byte(i % groups)
				dest[0] = byte(i % groups)
				origin[15] = 1
				dest[15] = 2
				w.flagged(w.shipper, domain.VisMarketplace, nil, origin, dest, true, false, win, win, f64(1), domain.CargoConstraints{})
			}
			started := time.Now()
			limit := 0
			doc := w.consolidate(w.actor(), cap.ID, &limit)
			elapsed := time.Since(started)
			if doc.PoolLoadCount != n || doc.ReturnedCandidateCount != 0 || doc.FeasibleCandidateCount == 0 {
				t.Fatalf("pool %d pairs %d feasible %d", doc.PoolLoadCount, doc.EvaluatedPairCount, doc.FeasibleCandidateCount)
			}
			t.Logf("pool=%d groups=%d pairs=%d groupage_calls=%d duration=%s", doc.PoolLoadCount, groups, doc.EvaluatedPairCount, doc.EvaluatedPairCount, elapsed)
		})
	}
}

func createOwned(t *testing.T, general, cross bool) string {
	t.Helper()
	_, _, body := createdLoad(t, general, cross)
	return body
}

func createdLoad(t *testing.T, general, cross bool) (*Service, Actor, string) {
	t.Helper()
	store := repository.NewMemory()
	shipper := uuid.New()
	source := uuid.New()
	svc := New(store, sourceverify.MapVerifier{Owned: map[string]bool{fmt.Sprintf("%s|%s|%s", shipper, domain.SourceTransportOrder, source): true}})
	actor := Actor{TenantID: shipper, UserID: uuid.New()}
	result, err := svc.CreateLoad(context.Background(), actor, "", "", CreateLoadCommand{Publish: true, Load: domain.LoadOpportunity{
		SourceType: domain.SourceTransportOrder, SourceID: source,
		Pickup: domain.Place{Label: "A", CountryCode: "RU", City: "Moscow"}, Delivery: domain.Place{Label: "B", CountryCode: "RU", City: "Kazan"},
		VisibilityScope: domain.VisPrivate, ConsolidationAllowed: general, CrossShipperConsolidationAllowed: cross,
	}})
	if err != nil {
		t.Fatal(err)
	}
	return svc, actor, string(result.Body)
}

func patchOptIn(t *testing.T, svc *Service, actor Actor, body string, version int, general, cross *bool) string {
	t.Helper()
	var doc struct {
		ID uuid.UUID `json:"id"`
	}
	if err := json.Unmarshal([]byte(body), &doc); err != nil {
		t.Fatal(err)
	}
	result, err := svc.UpdateLoad(context.Background(), actor, doc.ID, LoadPatch{Version: version, ConsolidationAllowed: general, CrossShipperConsolidationAllowed: cross, Changed: true}, "", "")
	if err != nil {
		t.Fatal(err)
	}
	return string(result.Body)
}

func feasiblePair(t *testing.T, opted bool) ConsolidationResponse {
	t.Helper()
	w, cap, _, _ := sameOwnerPair(t)
	if !opted {
		t.Fatal("caller")
	}
	doc := w.consolidate(w.actor(), cap.ID, nil)
	if doc.FeasibleCandidateCount != 1 || doc.Candidates[0].CandidateID == uuid.Nil {
		t.Fatalf("%+v", doc)
	}
	return doc
}

func sameOwnerPair(t *testing.T) (*world, domain.Capacity, domain.LoadOpportunity, domain.LoadOpportunity) {
	t.Helper()
	w := newWorld(t)
	cap := w.readyCapacity()
	origin, dest := uuid.New(), uuid.New()
	win := span(w.at, w.at.Add(2*time.Hour))
	left := w.flagged(w.shipper, domain.VisMarketplace, nil, origin, dest, true, false, win, win, f64(100), domain.CargoConstraints{})
	right := w.flagged(w.shipper, domain.VisMarketplace, nil, origin, dest, true, false, win, win, f64(100), domain.CargoConstraints{})
	return w, cap, left, right
}

func crossPair(t *testing.T, leftCross, rightCross bool) (*world, domain.Capacity) {
	t.Helper()
	w, cap, _, _ := crossLoadsOpt(t, leftCross, rightCross)
	return w, cap
}

func crossLoads(t *testing.T) (*world, domain.Capacity, domain.LoadOpportunity, domain.LoadOpportunity) {
	t.Helper()
	return crossLoadsOpt(t, true, true)
}

func crossLoadsOpt(t *testing.T, leftCross, rightCross bool) (*world, domain.Capacity, domain.LoadOpportunity, domain.LoadOpportunity) {
	t.Helper()
	w := newWorld(t)
	cap := w.readyCapacity()
	origin, dest := uuid.New(), uuid.New()
	win := span(w.at, w.at.Add(2*time.Hour))
	left := w.flagged(w.shipper, domain.VisMarketplace, nil, origin, dest, false, leftCross, win, win, f64(100), domain.CargoConstraints{})
	right := w.flagged(uuid.New(), domain.VisMarketplace, nil, origin, dest, false, rightCross, win, win, f64(100), domain.CargoConstraints{})
	return w, cap, left, right
}

func windowPair(t *testing.T, pickupA, pickupB, deliveryA, deliveryB domain.TimeWindow) (*world, domain.Capacity) {
	t.Helper()
	w := newWorld(t)
	cap := w.readyCapacity()
	origin, dest := uuid.New(), uuid.New()
	w.flagged(w.shipper, domain.VisMarketplace, nil, origin, dest, true, false, pickupA, deliveryA, f64(10), domain.CargoConstraints{})
	w.flagged(w.shipper, domain.VisMarketplace, nil, origin, dest, true, false, pickupB, deliveryB, f64(10), domain.CargoConstraints{})
	return w, cap
}

func (w *world) readyCapacity() domain.Capacity {
	w.t.Helper()
	cap := w.capacity(domain.CapacityAvailable, domain.SourceManual, 0, 0)
	cap.PayloadRemainingKg = f64(1000)
	cap.Version = 2
	if err := w.store.Within(context.Background(), func(tx repository.Tx) error {
		return tx.UpdateCapacity(context.Background(), cap, 1)
	}); err != nil {
		w.t.Fatal(err)
	}
	return cap
}

func (w *world) flagged(owner uuid.UUID, visibility string, invited []uuid.UUID, origin, dest uuid.UUID, general, cross bool, pickup, delivery domain.TimeWindow, weight *float64, cargo domain.CargoConstraints) domain.LoadOpportunity {
	w.t.Helper()
	return w.insertFlagged(owner, visibility, invited, located(origin, "Origin"), located(dest, "Dest"), general, cross, pickup, delivery, weight, cargo, uuid.New())
}

func (w *world) insertFlagged(owner uuid.UUID, visibility string, invited []uuid.UUID, pickup, delivery domain.Place, general, cross bool, pickupWindow, deliveryWindow domain.TimeWindow, weight *float64, cargo domain.CargoConstraints, source uuid.UUID) domain.LoadOpportunity {
	w.t.Helper()
	load := domain.LoadOpportunity{
		ID: uuid.New(), OwnerTenantID: owner, SourceType: domain.SourceTransportOrder, SourceID: source,
		Pickup: pickup, Delivery: delivery, PickupWindow: pickupWindow, DeliveryWindow: deliveryWindow,
		VisibilityScope: visibility, InvitedCarrierCompanyIDs: invited, Status: domain.LoadPublished, Version: 1,
		ConsolidationAllowed: general, CrossShipperConsolidationAllowed: cross, WeightKg: weight, Cargo: cargo,
		CreatedAt: w.at, UpdatedAt: w.at,
	}
	if err := w.store.Within(context.Background(), func(tx repository.Tx) error {
		return tx.InsertLoad(context.Background(), load)
	}); err != nil {
		w.t.Fatal(err)
	}
	return load
}

func (w *world) consolidate(actor Actor, capacityID uuid.UUID, limit *int) ConsolidationResponse {
	w.t.Helper()
	result, err := w.svc.SearchConsolidation(context.Background(), actor, ConsolidationCommand{CapacityID: capacityID, Pattern: PatternSameOriginDestination, CandidateLimit: limit})
	if err != nil {
		w.t.Fatal(err)
	}
	var doc ConsolidationResponse
	if err := json.Unmarshal(result.Body, &doc); err != nil {
		w.t.Fatal(err)
	}
	raw := string(result.Body)
	if strings.Contains(raw, "match_score") || strings.Contains(raw, `"rank"`) {
		w.t.Fatalf("score leaked %s", raw)
	}
	return doc
}

func replaceLoad(t *testing.T, w *world, load domain.LoadOpportunity) {
	t.Helper()
	if err := w.store.Within(context.Background(), func(tx repository.Tx) error {
		current, err := tx.GetLoad(context.Background(), load.ID)
		if err != nil {
			return err
		}
		load.Version = current.Version + 1
		return tx.UpdateLoad(context.Background(), load, current.Version)
	}); err != nil {
		t.Fatal(err)
	}
}

func located(id uuid.UUID, city string) domain.Place {
	return domain.Place{LocationID: &id, City: city, CountryCode: "RU", Label: city}
}

func span(start, end time.Time) domain.TimeWindow {
	return domain.TimeWindow{Start: &start, End: &end}
}

func wAt(t *testing.T) time.Time {
	t.Helper()
	return time.Date(2026, 9, 25, 8, 0, 0, 0, time.UTC)
}

func pairItems(equipment compat.Equipment, left, right compat.Cargo, need compat.AccessNeed, ctx compat.Context) compat.Result {
	return compat.EvaluateGroupageItems(equipment, []compat.GroupageItem{{Cargo: left, AccessNeed: need}, {Cargo: right, AccessNeed: need}}, ctx)
}

func cargo(id string, kg float64) compat.Cargo {
	return compat.Cargo{ID: id, WeightKg: f64(kg)}
}

func reason(got compat.Result, code string) bool {
	for _, item := range append(append([]compat.Reason{}, got.HardRejects...), got.IndeterminateReasons...) {
		if item.ReasonCode == code {
			return true
		}
	}
	for _, item := range got.Conditions {
		if item.ReasonCode == code {
			return true
		}
	}
	return false
}

func has(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}

func mustJSON(v any) string {
	raw, _ := json.Marshal(v)
	return string(raw)
}

type emptyCatalog struct{}

func (emptyCatalog) Evaluation(context.Context, uuid.UUID) (compat.Context, error) {
	return compat.Context{}, nil
}

type tenantDenyCatalog struct{ deny uuid.UUID }

func (c tenantDenyCatalog) Evaluation(_ context.Context, tenant uuid.UUID) (compat.Context, error) {
	if tenant != c.deny {
		return compat.Context{}, nil
	}
	return compat.Context{Rules: []compat.Rule{{
		RuleCode: "DENY", RuleKind: compat.KindCargoCargo, Layer: compat.LayerTenant,
		LeftSelectorType: "ANY", RightSelectorType: "ANY", Decision: compat.DecisionDeny,
		ReasonCode: "TENANT_HARD_DENY", Severity: compat.SeverityHard,
	}}}, nil
}

type traceCatalog struct{ tenant, source string }

func (c traceCatalog) Evaluation(context.Context, uuid.UUID) (compat.Context, error) {
	return compat.Context{
		RuleSets: []compat.RuleSetRef{{ID: uuid.NewString(), Scope: "TENANT", TenantID: &c.tenant, Version: 1}},
		CatalogRefs: []compat.CatalogVersionRef{{
			ID: uuid.NewString(), CatalogKind: "cargo", Scope: "TENANT", TenantID: &c.tenant, Version: 1,
		}},
		Rules: []compat.Rule{{
			RuleCode: "TRACE", RuleKind: compat.KindCargoCargo, Layer: compat.LayerPlatform,
			LeftSelectorType: "ANY", RightSelectorType: "ANY", Decision: compat.DecisionAllow,
			SourceReference: &c.source, RuleSetTenantID: &c.tenant,
		}},
	}, nil
}

type boomStore struct{}

func (boomStore) SaveConsolidation(context.Context, repository.ConsolidationRun, []repository.ConsolidationCandidate) error {
	return errors.New("save failed")
}

func (boomStore) GetConsolidation(context.Context, uuid.UUID, uuid.UUID) (repository.ConsolidationRun, []repository.ConsolidationCandidate, error) {
	return repository.ConsolidationRun{}, nil, repository.ErrNotFound
}
