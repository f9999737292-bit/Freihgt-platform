package service

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/freight-platform/network-optimizer-service/internal/compat"
	"github.com/freight-platform/network-optimizer-service/internal/domain"
	apperrors "github.com/freight-platform/network-optimizer-service/internal/platform/errors"
	"github.com/freight-platform/network-optimizer-service/internal/repository"
)

type saveCounter struct {
	repository.ConsolidationStore
	saves int
}

func (c *saveCounter) SaveConsolidation(ctx context.Context, run repository.ConsolidationRun, candidates []repository.ConsolidationCandidate) error {
	c.saves++
	return c.ConsolidationStore.SaveConsolidation(ctx, run, candidates)
}

func (w *world) countSaves() *saveCounter {
	counter := &saveCounter{ConsolidationStore: w.svc.consolidations}
	w.svc.consolidations = counter
	return counter
}

func (w *world) nMember(capacityID uuid.UUID, limit *int, policy string) (NMemberConsolidationSearchResponse, []byte, error) {
	w.t.Helper()
	result, err := w.svc.SearchConsolidation(context.Background(), w.actor(), ConsolidationCommand{
		CapacityID: capacityID, Pattern: PatternSameOriginDestinationNMember, CandidateLimit: limit, Policy: policy,
	})
	if err != nil {
		return NMemberConsolidationSearchResponse{}, nil, err
	}
	var doc NMemberConsolidationSearchResponse
	if err := json.Unmarshal(result.Body, &doc); err != nil {
		return NMemberConsolidationSearchResponse{}, result.Body, err
	}
	return doc, result.Body, nil
}

func seedOD(w *world, owner uuid.UUID, n int, origin, dest uuid.UUID, general, cross bool, pickup, delivery domain.TimeWindow, weight *float64) []domain.LoadOpportunity {
	w.t.Helper()
	loads := make([]domain.LoadOpportunity, 0, n)
	for i := 0; i < n; i++ {
		loads = append(loads, w.flagged(owner, domain.VisMarketplace, nil, origin, dest, general, cross, pickup, delivery, weight, domain.CargoConstraints{}))
	}
	return loads
}

func TestNLO03EBoundedNMemberSearch(t *testing.T) {
	t.Run("NLO03E_PATTERN_DISPATCH", func(t *testing.T) {
		w, cap := nMemberWorld(t, 2, f64(10))
		doc, raw, err := searchOwned(t, w, cap.ID, nil)
		if err != nil || doc.Pattern != PatternSameOriginDestinationNMember || strings.Contains(string(raw), `"evaluated_pair_count"`) {
			t.Fatalf("err %v raw %s", err, raw)
		}
		if doc.EvaluatedSetCount != 1 || len(doc.Candidates) != 1 || len(doc.Candidates[0].Members) != 2 {
			t.Fatalf("%+v", doc)
		}
	})
	t.Run("NLO03E_UNKNOWN_PATTERN_REJECTED", func(t *testing.T) {
		w := newWorld(t)
		cap := w.readyCapacity()
		for _, pattern := range []string{"NOT_A_PATTERN", "SAME_ORIGIN_SAME_DESTINATION_N"} {
			_, err := w.svc.SearchConsolidation(context.Background(), w.actor(), ConsolidationCommand{CapacityID: cap.ID, Pattern: pattern})
			var app *apperrors.AppError
			if !errors.As(err, &app) || app.Code != apperrors.CodeValidation || app.Details["reason"] != "PATTERN_NOT_IMPLEMENTED" {
				t.Fatalf("%s: %v", pattern, err)
			}
		}
	})
	t.Run("NLO03E_POOL_10_ACCEPTED", func(t *testing.T) {
		w, cap := nMemberWorld(t, 10, f64(10))
		doc, _, err := searchOwned(t, w, cap.ID, nil)
		if err != nil || doc.PoolLoadCount != 10 || doc.EvaluatedSetCount != 165 {
			t.Fatalf("err %v doc %+v", err, doc)
		}
	})
	t.Run("NLO03E_POOL_11_REJECTED_BEFORE_ENUMERATION", func(t *testing.T) {
		w, cap := nMemberWorld(t, 11, f64(10))
		saves := w.countSaves()
		_, _, err := searchOwned(t, w, cap.ID, nil)
		var app *apperrors.AppError
		if !errors.As(err, &app) || app.Code != apperrors.CodeValidation || app.Details["reason"] != ReasonPoolLimitExceeded {
			t.Fatal(err)
		}
		if saves.saves != 0 || w.svc.groupageCalls != 0 || w.routes.routeCalls != 0 {
			t.Fatalf("saves %d groupage %d routes %d", saves.saves, w.svc.groupageCalls, w.routes.routeCalls)
		}
	})
	t.Run("NLO03E_SET_SIZE_2", func(t *testing.T) {
		w, cap := nMemberWorld(t, 2, f64(10))
		doc, _, err := searchOwned(t, w, cap.ID, nil)
		if err != nil || len(doc.Candidates) != 1 || len(doc.Candidates[0].Members) != 2 || doc.Candidates[0].ExecutionSupported {
			t.Fatalf("err %v %+v", err, doc)
		}
		_, rows, err := w.store.GetConsolidation(context.Background(), w.carrier, doc.SearchID)
		if err != nil || rows[0].Pattern != PatternSameOriginDestinationNMember || len(rows[0].Members) != 2 {
			t.Fatal(err)
		}
	})
	t.Run("NLO03E_SET_SIZE_3", func(t *testing.T) {
		w, cap := nMemberWorld(t, 3, f64(10))
		doc, _, err := searchOwned(t, w, cap.ID, nil)
		if err != nil || doc.EvaluatedSetCount != 4 {
			t.Fatalf("err %v %+v", err, doc)
		}
		_, rows, err := w.store.GetConsolidation(context.Background(), w.carrier, doc.SearchID)
		if err != nil {
			t.Fatal(err)
		}
		triples := 0
		for _, row := range rows {
			if len(row.Members) == 3 && row.Members[2].Ordinal == 3 {
				triples++
			}
		}
		if triples != 1 {
			t.Fatalf("triples %d", triples)
		}
	})
	t.Run("NLO03E_NO_SET_SIZE_4", func(t *testing.T) {
		w, cap := nMemberWorld(t, 4, f64(10))
		doc, _, err := searchOwned(t, w, cap.ID, nil)
		if err != nil || doc.EvaluatedSetCount != 10 {
			t.Fatalf("err %v count %d", err, doc.EvaluatedSetCount)
		}
		_, rows, err := w.store.GetConsolidation(context.Background(), w.carrier, doc.SearchID)
		if err != nil {
			t.Fatal(err)
		}
		for _, row := range rows {
			if len(row.Members) < 2 || len(row.Members) > 3 {
				t.Fatalf("members %d", len(row.Members))
			}
		}
	})
	t.Run("NLO03E_MAX_SET_COUNT_165", func(t *testing.T) {
		w, cap := nMemberWorld(t, 10, f64(10))
		doc, _, err := searchOwned(t, w, cap.ID, nil)
		if err != nil || doc.EvaluatedSetCount != 165 || doc.EvaluatedSetCount > 165 {
			t.Fatalf("err %v count %d", err, doc.EvaluatedSetCount)
		}
	})
	t.Run("NLO03E_MAX_GROUPAGE_CALLS_330", func(t *testing.T) {
		w, cap := nMemberWorld(t, 10, f64(10))
		w.svc.UseCatalog(&callCatalog{fallback: compat.Context{}})
		doc, _, err := searchOwned(t, w, cap.ID, nil)
		if err != nil || doc.EvaluatedSetCount != 165 || w.svc.groupageCalls != 330 {
			t.Fatalf("err %v sets %d groupage %d", err, doc.EvaluatedSetCount, w.svc.groupageCalls)
		}
	})
	t.Run("NLO03E_LEXICOGRAPHIC_DETERMINISM", func(t *testing.T) {
		w, cap := nMemberWorld(t, 3, f64(10))
		doc, _, err := searchOwned(t, w, cap.ID, nil)
		if err != nil {
			t.Fatal(err)
		}
		_, rows, err := w.store.GetConsolidation(context.Background(), w.carrier, doc.SearchID)
		if err != nil {
			t.Fatal(err)
		}
		keys := memberIDKeys(rows)
		again, _, err := searchOwned(t, w, cap.ID, nil)
		if err != nil {
			t.Fatal(err)
		}
		_, more, err := w.store.GetConsolidation(context.Background(), w.carrier, again.SearchID)
		if err != nil || memberIDKeys(more) != keys {
			t.Fatalf("keys %s again %s", keys, memberIDKeys(more))
		}
		triple := tripleRow(t, rows)
		ids := []string{triple.Members[0].LoadOpportunityID.String(), triple.Members[1].LoadOpportunityID.String(), triple.Members[2].LoadOpportunityID.String()}
		if !(ids[0] < ids[1] && ids[1] < ids[2]) || len(rows) != 4 {
			t.Fatalf("order %v rows %d", ids, len(rows))
		}
	})
	t.Run("NLO03E_ALL_MEMBERS_SAME_OD", func(t *testing.T) {
		w := newWorld(t)
		cap := w.readyCapacity()
		win := span(w.at, w.at.Add(2*time.Hour))
		leftOrigin, leftDest := uuid.New(), uuid.New()
		rightOrigin, rightDest := uuid.New(), uuid.New()
		seedOD(w, w.shipper, 2, leftOrigin, leftDest, true, false, win, win, f64(10))
		seedOD(w, w.shipper, 2, rightOrigin, rightDest, true, false, win, win, f64(10))
		doc, _, err := searchOwned(t, w, cap.ID, nil)
		if err != nil || doc.EvaluatedSetCount != 2 || len(doc.Candidates) != 2 {
			t.Fatalf("err %v %+v", err, doc)
		}
		for _, candidate := range doc.Candidates {
			pickup := candidate.Members[0].Load.Pickup.LocationID
			delivery := candidate.Members[0].Load.Delivery.LocationID
			for _, member := range candidate.Members[1:] {
				if member.Load.Pickup.LocationID == nil || *member.Load.Pickup.LocationID != *pickup || *member.Load.Delivery.LocationID != *delivery {
					t.Fatalf("%+v", candidate.Members)
				}
			}
		}
	})
	t.Run("NLO03E_PICKUP_WINDOW_N_WAY_INTERSECTION", func(t *testing.T) {
		_, _, rows := windowWorld(t, true)
		triple := tripleRow(t, rows)
		if triple.Status != ConsolidationHardReject || !has(triple.HardRejectReasons, ReasonPickupDisjoint) {
			t.Fatalf("%+v", triple)
		}
	})
	t.Run("NLO03E_DELIVERY_WINDOW_N_WAY_INTERSECTION", func(t *testing.T) {
		_, _, rows := windowWorld(t, false)
		triple := tripleRow(t, rows)
		if triple.Status != ConsolidationHardReject || !has(triple.HardRejectReasons, ReasonDeliveryDisjoint) {
			t.Fatalf("%+v", triple)
		}
	})
	t.Run("NLO03E_GROUPAGE_REUSE", func(t *testing.T) {
		w, cap := nMemberWorld(t, 3, f64(400))
		doc, _, err := searchOwned(t, w, cap.ID, nil)
		if err != nil || doc.FeasibleCandidateCount != 3 || doc.HardRejectCandidateCount != 1 || w.svc.groupageCalls == 0 {
			t.Fatalf("err %v doc %+v calls %d", err, doc, w.svc.groupageCalls)
		}
		_, rows, err := w.store.GetConsolidation(context.Background(), w.carrier, doc.SearchID)
		if err != nil || !has(tripleRow(t, rows).HardRejectReasons, "PAYLOAD_EXCEEDED") {
			t.Fatal(tripleRow(t, rows).HardRejectReasons)
		}
	})
	t.Run("NLO03E_ORDER_INVARIANT_HARD_REJECT", func(t *testing.T) {
		w, cap := nMemberWorld(t, 3, f64(400))
		loads := published(t, w)
		equipment := equipmentFromCapacity(cap, nil)
		forward := w.svc.assessNMemberSet(context.Background(), cap.OwnerTenantID, equipment, loads)
		reverse := append([]domain.LoadOpportunity(nil), loads...)
		for i, j := 0, len(reverse)-1; i < j; i, j = i+1, j-1 {
			reverse[i], reverse[j] = reverse[j], reverse[i]
		}
		backward := w.svc.assessNMemberSet(context.Background(), cap.OwnerTenantID, equipment, reverse)
		if forward.status != ConsolidationHardReject || backward.status != forward.status || strings.Join(forward.hard, ",") != strings.Join(backward.hard, ",") {
			t.Fatalf("%s %v / %s %v", forward.status, forward.hard, backward.status, backward.hard)
		}
	})
	t.Run("NLO03E_REQUIRED_UNKNOWN_INDETERMINATE", func(t *testing.T) {
		w := newWorld(t)
		cap := w.readyCapacity()
		origin, dest := uuid.New(), uuid.New()
		win := span(w.at, w.at.Add(2*time.Hour))
		unknown := win
		unknown.End = nil
		seedOD(w, w.shipper, 2, origin, dest, true, false, win, win, f64(10))
		w.flagged(w.shipper, domain.VisMarketplace, nil, origin, dest, true, false, unknown, win, f64(10), domain.CargoConstraints{})
		doc, _, err := searchOwned(t, w, cap.ID, nil)
		if err != nil || doc.FeasibleCandidateCount == 0 || doc.IndeterminateCandidateCount == 0 {
			t.Fatalf("err %v %+v", err, doc)
		}
		_, rows, err := w.store.GetConsolidation(context.Background(), w.carrier, doc.SearchID)
		if err != nil {
			t.Fatal(err)
		}
		triple := tripleRow(t, rows)
		if triple.Status != ConsolidationIndeterminate || !has(triple.IndeterminateReasonCodes, ReasonPickupUnknown) || triple.Status == ConsolidationFeasible {
			t.Fatalf("%+v", triple)
		}
	})
	t.Run("NLO03E_CROSS_SHIPPER_FAIL_CLOSED", func(t *testing.T) {
		w := newWorld(t)
		cap := w.readyCapacity()
		origin, dest := uuid.New(), uuid.New()
		win := span(w.at, w.at.Add(2*time.Hour))
		other := uuid.New()
		seedOD(w, w.shipper, 2, origin, dest, false, true, win, win, f64(10))
		w.flagged(other, domain.VisMarketplace, nil, origin, dest, false, true, win, win, f64(10), domain.CargoConstraints{})
		doc, raw, err := searchOwned(t, w, cap.ID, nil)
		if err != nil || doc.FeasibleCandidateCount != 0 {
			t.Fatalf("err %v %+v", err, doc)
		}
		_, rows, err := w.store.GetConsolidation(context.Background(), w.carrier, doc.SearchID)
		if err != nil || tripleRow(t, rows).Status != ConsolidationIndeterminate || !has(tripleRow(t, rows).IndeterminateReasonCodes, ReasonMultiPartyUnavailable) {
			t.Fatal(tripleRow(t, rows))
		}
		if strings.Contains(string(raw), ReasonMultiPartyUnavailable) == false {
			t.Fatal(string(raw))
		}
	})
	t.Run("NLO03E_PRIVATE_OVERLAY_NOT_LEAKED", func(t *testing.T) {
		w := newWorld(t)
		cap := w.readyCapacity()
		origin, dest := uuid.New(), uuid.New()
		win := span(w.at, w.at.Add(2*time.Hour))
		other := uuid.New()
		seedOD(w, w.shipper, 1, origin, dest, false, true, win, win, f64(10))
		w.flagged(other, domain.VisMarketplace, nil, origin, dest, false, true, win, win, f64(10), domain.CargoConstraints{})
		sentinel := "PRIVATE_OVERLAY_SENTINEL"
		cat := &callCatalog{byTenant: map[uuid.UUID]compat.Context{w.shipper: {Rules: []compat.Rule{{
			RuleCode: sentinel, RuleKind: compat.KindCargoCargo, Layer: compat.LayerTenant,
			LeftSelectorType: "ANY", RightSelectorType: "ANY", Decision: compat.DecisionDeny,
			ReasonCode: sentinel, Severity: compat.SeverityHard,
		}}}}}
		w.svc.UseCatalog(cat)
		doc, raw, err := searchOwned(t, w, cap.ID, nil)
		if err != nil || doc.FeasibleCandidateCount != 0 || len(cat.calls) != 0 {
			t.Fatalf("err %v calls %v doc %+v", err, cat.calls, doc)
		}
		if strings.Contains(string(raw), sentinel) || strings.Contains(string(raw), w.shipper.String()) || strings.Contains(string(raw), other.String()) {
			t.Fatal(string(raw))
		}
	})
	t.Run("NLO03E_ROUTING_CALLS_ZERO", func(t *testing.T) {
		w, cap := nMemberWorld(t, 3, f64(10))
		if _, _, err := searchOwned(t, w, cap.ID, nil); err != nil {
			t.Fatal(err)
		}
		if w.routes.routeCalls != 0 || w.routes.calls != 0 {
			t.Fatalf("route %d matrix %d", w.routes.routeCalls, w.routes.calls)
		}
	})
	t.Run("NLO03E_CANDIDATE_LIMIT_ZERO_RESPONSE_ONLY", func(t *testing.T) {
		w, cap := nMemberWorld(t, 3, f64(10))
		limit := 0
		doc, _, err := searchOwned(t, w, cap.ID, &limit)
		if err != nil || doc.ReturnedCandidateCount != 0 || len(doc.Candidates) != 0 || doc.EvaluatedSetCount != 4 {
			t.Fatalf("err %v %+v", err, doc)
		}
		_, rows, err := w.store.GetConsolidation(context.Background(), w.carrier, doc.SearchID)
		if err != nil || len(rows) != 4 {
			t.Fatalf("persisted %d err %v", len(rows), err)
		}
	})
	t.Run("NLO03E_CANDIDATE_LIMIT_LARGE_DOES_NOT_EXPAND_WORK", func(t *testing.T) {
		w, cap := nMemberWorld(t, 4, f64(10))
		limit := 10000
		doc, _, err := searchOwned(t, w, cap.ID, &limit)
		if err != nil || doc.PoolLoadCount != 4 || doc.EvaluatedSetCount != 10 || doc.ReturnedCandidateCount > 10 {
			t.Fatalf("err %v %+v", err, doc)
		}
		overflow, overflowCap := nMemberWorld(t, 11, f64(10))
		_, _, err = searchOwned(t, overflow, overflowCap.ID, &limit)
		var app *apperrors.AppError
		if !errors.As(err, &app) || app.Details["reason"] != ReasonPoolLimitExceeded {
			t.Fatal(err)
		}
	})
	t.Run("NLO03E_SEARCH_BUDGET_PRECHECK", func(t *testing.T) {
		w, cap := nMemberWorld(t, 3, f64(10))
		saves := w.countSaves()
		policy := defaultNMemberPolicy()
		policy.MaxSetsEvaluated = 0
		policy.TimeBudget = 0
		w.svc.nMemberPolicy = policy
		_, _, err := searchOwned(t, w, cap.ID, nil)
		var app *apperrors.AppError
		if !errors.As(err, &app) || app.Details["reason"] != ReasonSearchBudgetExceeded || app.Details["budget"] != "sets" {
			t.Fatal(err)
		}
		if saves.saves != 0 {
			t.Fatal(saves.saves)
		}
		w.svc.groupageCalls = 0
		policy = defaultNMemberPolicy()
		policy.MaxGroupageCalls = 1
		w.svc.nMemberPolicy = policy
		w.svc.UseCatalog(&callCatalog{fallback: compat.Context{}})
		_, _, err = searchOwned(t, w, cap.ID, nil)
		if !errors.As(err, &app) || app.Details["budget"] != "groupage" || w.svc.groupageCalls != 0 || saves.saves != 0 {
			t.Fatalf("err %v calls %d saves %d", err, w.svc.groupageCalls, saves.saves)
		}
	})
	t.Run("NLO03E_TIME_BUDGET_FAIL_CLOSED", func(t *testing.T) {
		w, cap := nMemberWorld(t, 3, f64(10))
		saves := w.countSaves()
		base := time.Date(2026, 9, 27, 8, 0, 0, 0, time.UTC)
		ticks := 0
		w.svc.now = func() time.Time {
			ticks++
			if ticks >= 3 {
				return base.Add(6 * time.Second)
			}
			return base
		}
		_, _, err := searchOwned(t, w, cap.ID, nil)
		var app *apperrors.AppError
		if !errors.As(err, &app) || app.Details["reason"] != ReasonSearchBudgetExceeded || app.Details["budget"] != "time" || saves.saves != 0 {
			t.Fatalf("err %v saves %d", err, saves.saves)
		}
	})
	t.Run("NLO03E_BUDGET_ROLLBACK", func(t *testing.T) {
		w, cap := nMemberWorld(t, 3, f64(10))
		saves := w.countSaves()
		policy := defaultNMemberPolicy()
		policy.MaxSetsEvaluated = 1
		w.svc.nMemberPolicy = policy
		_, _, err := searchOwned(t, w, cap.ID, nil)
		var app *apperrors.AppError
		if !errors.As(err, &app) || app.Code != apperrors.CodeValidation || saves.saves != 0 {
			t.Fatal(err)
		}
		if _, _, getErr := w.store.GetConsolidation(context.Background(), w.carrier, uuid.New()); getErr == nil {
			t.Fatal("unexpected run")
		}
	})
	t.Run("NLO03E_FINGERPRINT_STABLE", func(t *testing.T) {
		w, cap := nMemberWorld(t, 2, f64(10))
		first, _, err := searchOwned(t, w, cap.ID, nil)
		if err != nil {
			t.Fatal(err)
		}
		second, _, err := searchOwned(t, w, cap.ID, nil)
		if err != nil {
			t.Fatal(err)
		}
		_, left, err := w.store.GetConsolidation(context.Background(), w.carrier, first.SearchID)
		if err != nil {
			t.Fatal(err)
		}
		_, right, err := w.store.GetConsolidation(context.Background(), w.carrier, second.SearchID)
		if err != nil || left[0].CandidateFingerprint != right[0].CandidateFingerprint {
			t.Fatal(err)
		}
	})
	t.Run("NLO03E_PATTERN_IN_FINGERPRINT", func(t *testing.T) {
		body := fingerprintBody(t, nil)
		if !strings.Contains(body, `"pattern":"SAME_ORIGIN_SAME_DESTINATION_N_MEMBER"`) {
			t.Fatal(body)
		}
		for _, banned := range []string{"shipment_version", "gps", "eta", "routing", "route_sequence"} {
			if strings.Contains(body, banned) {
				t.Fatal(banned)
			}
		}
	})
	t.Run("NLO03E_BUDGET_POLICY_IN_FINGERPRINT", func(t *testing.T) {
		body := fingerprintBody(t, strPtr("REHANDLING_FORBIDDEN"))
		if !strings.Contains(body, `"algorithm_policy_version":"nlo-0.3e-lex-v1"`) || !strings.Contains(body, `"budget_policy_version":"nlo-0.3e-budget-v1"`) || !strings.Contains(body, `"policy_version":"REHANDLING_FORBIDDEN"`) {
			t.Fatal(body)
		}
	})
	t.Run("NLO03E_EXECUTION_SUPPORTED_FALSE", func(t *testing.T) {
		w, cap := nMemberWorld(t, 3, f64(10))
		doc, _, err := searchOwned(t, w, cap.ID, nil)
		if err != nil {
			t.Fatal(err)
		}
		_, rows, err := w.store.GetConsolidation(context.Background(), w.carrier, doc.SearchID)
		if err != nil {
			t.Fatal(err)
		}
		for _, candidate := range doc.Candidates {
			if candidate.ExecutionSupported || candidate.Routing != nil {
				t.Fatalf("%+v", candidate)
			}
		}
		for _, row := range rows {
			if row.ExecutionSupported || row.Pattern != PatternSameOriginDestinationNMember {
				t.Fatalf("%+v", row)
			}
		}
		run, _, err := w.store.GetConsolidation(context.Background(), w.carrier, doc.SearchID)
		if err != nil || !run.PairCountNull || run.EvaluatedSetCount == nil || *run.EvaluatedSetCount != 4 {
			t.Fatalf("%+v", run)
		}
	})
	t.Run("NLO03E_LEGACY_PAIRWISE_NOT_POOL_CAPPED", func(t *testing.T) {
		w := newWorld(t)
		cap := w.readyCapacity()
		origin, dest := uuid.New(), uuid.New()
		win := span(w.at, w.at.Add(2*time.Hour))
		seedOD(w, w.shipper, 11, origin, dest, true, false, win, win, f64(10))
		doc := w.consolidate(w.actor(), cap.ID, nil)
		if doc.Pattern != PatternSameOriginDestination || doc.PoolLoadCount != 11 || doc.EvaluatedPairCount != 55 {
			t.Fatalf("%+v", doc)
		}
		for _, candidate := range doc.Candidates {
			if len(candidate.Members) != 2 {
				t.Fatalf("members %d", len(candidate.Members))
			}
		}
		raw := mustJSON(doc)
		if strings.Contains(raw, `"evaluated_set_count"`) || !strings.Contains(raw, `"evaluated_pair_count"`) {
			t.Fatal(raw)
		}
	})
	t.Run("NLO03E_CURRENT_TRIP_UNCHANGED", func(t *testing.T) {
		w, src, _ := newFill(t)
		doc := searchFill(t, w, src.execution.ShipmentID)
		if doc.Pattern != PatternCurrentTripFill || doc.MaxAdditionalLoads == nil || *doc.MaxAdditionalLoads != 1 || doc.EvaluatedPairCount < 1 {
			t.Fatalf("%+v", doc)
		}
		if len(doc.Candidates) == 0 || len(doc.Candidates[0].Members) != 1 || w.routes.routeCalls == 0 {
			t.Fatalf("members %d routes %d", len(doc.Candidates), w.routes.routeCalls)
		}
		run, _, err := w.store.GetConsolidation(context.Background(), w.carrier, doc.SearchID)
		if err != nil || run.EvaluatedPairCount != doc.EvaluatedPairCount || run.EvaluatedSetCount != nil || run.PairCountNull || run.CapacityID != nil {
			t.Fatalf("%+v %v", run, err)
		}
	})
	t.Run("NLO03E_FOREIGN_CAPACITY_NOT_FOUND", func(t *testing.T) {
		w, cap := nMemberWorld(t, 2, f64(10))
		_, err := w.svc.SearchConsolidation(context.Background(), serviceActor(uuid.New(), nil), ConsolidationCommand{CapacityID: cap.ID, Pattern: PatternSameOriginDestinationNMember})
		var app *apperrors.AppError
		if !errors.As(err, &app) || app.Code != apperrors.CodeNotFound {
			t.Fatal(err)
		}
	})
}

func nMemberWorld(t *testing.T, n int, weight *float64) (*world, domain.Capacity) {
	t.Helper()
	w := newWorld(t)
	cap := w.readyCapacity()
	origin, dest := uuid.New(), uuid.New()
	win := span(w.at, w.at.Add(2*time.Hour))
	seedOD(w, w.shipper, n, origin, dest, true, false, win, win, weight)
	return w, cap
}

func searchOwned(t *testing.T, w *world, capacityID uuid.UUID, limit *int) (NMemberConsolidationSearchResponse, []byte, error) {
	t.Helper()
	return w.nMember(capacityID, limit, "")
}

func windowWorld(t *testing.T, pickup bool) (*world, domain.Capacity, []repository.ConsolidationCandidate) {
	t.Helper()
	w := newWorld(t)
	cap := w.readyCapacity()
	origin, dest := uuid.New(), uuid.New()
	open := span(w.at, w.at.Add(24*time.Hour))
	a := span(w.at, w.at.Add(10*time.Hour))
	b := span(w.at.Add(5*time.Hour), w.at.Add(15*time.Hour))
	c := span(w.at.Add(12*time.Hour), w.at.Add(20*time.Hour))
	windows := []domain.TimeWindow{a, b, c}
	for _, window := range windows {
		pickupWindow, deliveryWindow := open, open
		if pickup {
			pickupWindow = window
		} else {
			deliveryWindow = window
		}
		w.flagged(w.shipper, domain.VisMarketplace, nil, origin, dest, true, false, pickupWindow, deliveryWindow, f64(10), domain.CargoConstraints{})
	}
	doc, _, err := searchOwned(t, w, cap.ID, nil)
	if err != nil {
		t.Fatal(err)
	}
	_, rows, err := w.store.GetConsolidation(context.Background(), w.carrier, doc.SearchID)
	if err != nil {
		t.Fatal(err)
	}
	return w, cap, rows
}

func tripleRow(t *testing.T, rows []repository.ConsolidationCandidate) repository.ConsolidationCandidate {
	t.Helper()
	for _, row := range rows {
		if len(row.Members) == 3 {
			return row
		}
	}
	t.Fatal("missing triple")
	return repository.ConsolidationCandidate{}
}

func published(t *testing.T, w *world) []domain.LoadOpportunity {
	t.Helper()
	var loads []domain.LoadOpportunity
	err := w.store.Within(context.Background(), func(tx repository.Tx) error {
		rows, err := tx.ListPublicConsolidationPool(context.Background(), w.carrier, nil)
		loads = rows
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
	return loads
}

func memberIDKeys(rows []repository.ConsolidationCandidate) string {
	var b strings.Builder
	for _, row := range rows {
		for _, member := range row.Members {
			b.WriteString(member.LoadOpportunityID.String())
			b.WriteByte(',')
		}
		b.WriteByte(';')
	}
	return b.String()
}

func fingerprintBody(t *testing.T, policy *string) string {
	t.Helper()
	w, cap := nMemberWorld(t, 2, f64(10))
	value := ""
	if policy != nil {
		value = *policy
	}
	loads := published(t, w)
	set := w.svc.assessNMemberSet(context.Background(), cap.OwnerTenantID, equipmentFromCapacity(cap, nil), loads)
	return string(set.fingerprintDocument(cap, value, w.svc.nMemberBounds()))
}
