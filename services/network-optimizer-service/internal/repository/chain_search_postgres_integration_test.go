//go:build integration

package repository

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
)

func TestI2A_PG_RepositoryRoundTrip(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 4*time.Minute)
	defer cancel()
	pool := startPostgres(t, ctx)
	applyThrough000097(t, ctx, pool)
	store := NewPostgres(pool)

	tenant := uuid.New()
	otherTenant := uuid.New()
	created := time.Now().UTC().Truncate(time.Millisecond)
	deadline, claim := DeadlineAndClaimExpiry(created)
	owner := uuid.New()
	policyID := uuid.New()
	policyVersion := 2
	zero := 0
	knownZero := ServiceDurationSnapshot{
		State:           ServiceDurationKnown,
		PolicyID:        &policyID,
		PolicyVersion:   &policyVersion,
		PickupKnown:     true,
		PickupSeconds:   &zero,
		DeliveryKnown:   true,
		DeliverySeconds: &zero,
	}
	capacityID := uuid.New()
	capacityVersion := 1
	pending := ChainSearchRun{
		ID:                           uuid.New(),
		TenantID:                     tenant,
		Status:                       ChainSearchPending,
		CreatedAt:                    created,
		RequestFingerprint:           "req-fp-pending",
		PolicyVersion:                "NLO-0.9C-I0/BOUNDED_TWO_LOAD",
		AlgorithmVersion:             "BOUNDED_TWO_ORDER_COMPARISON",
		CatalogFingerprint:           "catalog-fp",
		RoutingProviderVersion:       "UNKNOWN",
		DeadlineAt:                   deadline,
		OwnerToken:                   owner,
		ClaimExpiresAt:               claim,
		ServiceDurationPolicyID:      &policyID,
		ServiceDurationPolicyVersion: &policyVersion,
		ServiceDurationSnapshot:      knownZero,
		HorizonStart:                 created,
		HorizonEnd:                   created.Add(48 * time.Hour),
		SeedSource:                   ChainSeedManualAvailableCapacity,
		SeedCapacityID:               &capacityID,
		SeedCapacityVersion:          &capacityVersion,
		CandidateCount:               0,
		EvaluatedPairCount:           0,
		ProviderCallCount:            0,
		SearchComplete:               false,
		GlobalOptimum:                false,
		ExecutionSupported:           false,
		ReasonCounts:                 map[string]int{},
	}

	if err := store.WithinChainSearch(ctx, func(tx ChainSearchTx) error {
		return tx.InsertChainSearchRun(ctx, pending)
	}); err != nil {
		t.Fatalf("insert pending: %v", err)
	}

	var got ChainSearchRun
	if err := store.WithinChainSearch(ctx, func(tx ChainSearchTx) error {
		var err error
		got, err = tx.GetChainSearchRun(ctx, tenant, pending.ID)
		return err
	}); err != nil {
		t.Fatalf("get pending: %v", err)
	}
	if got.Status != ChainSearchPending || got.OwnerToken != owner {
		t.Fatalf("pending round-trip status=%s owner=%s", got.Status, got.OwnerToken)
	}
	if got.ServiceDurationSnapshot.State != ServiceDurationKnown || got.ServiceDurationSnapshot.PickupSeconds == nil || *got.ServiceDurationSnapshot.PickupSeconds != 0 {
		t.Fatalf("known zero not preserved: %+v", got.ServiceDurationSnapshot)
	}
	if !got.DeadlineAt.Equal(deadline) || !got.ClaimExpiresAt.Equal(claim) {
		t.Fatalf("deadline/claim mismatch got %v %v", got.DeadlineAt, got.ClaimExpiresAt)
	}

	if err := store.WithinChainSearch(ctx, func(tx ChainSearchTx) error {
		_, err := tx.GetChainSearchRun(ctx, otherTenant, pending.ID)
		if !errors.Is(err, ErrNotFound) {
			return err
		}
		return nil
	}); err != nil {
		t.Fatalf("cross-tenant get: %v", err)
	}

	started := created.Add(time.Second)
	completed := created.Add(10 * time.Second)
	unknownSnapshot := ServiceDurationSnapshot{State: ServiceDurationUnknown, PickupKnown: false, DeliveryKnown: false}
	terminal := got
	terminal.Status = ChainSearchCompleted
	terminal.StartedAt = &started
	terminal.CompletedAt = &completed
	terminal.SearchComplete = true
	terminal.CandidateCount = 1
	terminal.EvaluatedPairCount = 2
	terminal.ProviderCallCount = 3
	terminal.ServiceDurationPolicyID = nil
	terminal.ServiceDurationPolicyVersion = nil
	terminal.ServiceDurationSnapshot = unknownSnapshot
	terminal.ReasonCounts = map[string]int{"STACKING_UNKNOWN": 2}

	if err := store.WithinChainSearch(ctx, func(tx ChainSearchTx) error {
		if err := tx.LockChainSearchRun(ctx, tenant, pending.ID); err != nil {
			return err
		}
		ok, err := tx.UpdateChainSearchRunConditional(ctx, terminal, ChainSearchPending, &owner, true)
		if err != nil {
			return err
		}
		if !ok {
			t.Fatal("conditional update missed")
		}
		return nil
	}); err != nil {
		t.Fatalf("terminal update: %v", err)
	}

	if err := store.WithinChainSearch(ctx, func(tx ChainSearchTx) error {
		var err error
		got, err = tx.GetChainSearchRun(ctx, tenant, pending.ID)
		return err
	}); err != nil {
		t.Fatalf("get terminal: %v", err)
	}
	if got.Status != ChainSearchCompleted || !got.SearchComplete || got.StartedAt == nil || got.CompletedAt == nil {
		t.Fatalf("terminal fields incomplete: %+v", got)
	}
	if got.ServiceDurationSnapshot.State != ServiceDurationUnknown || got.ServiceDurationSnapshot.PickupKnown {
		t.Fatalf("unknown snapshot not preserved: %+v", got.ServiceDurationSnapshot)
	}
	if got.ReasonCounts["STACKING_UNKNOWN"] != 2 {
		t.Fatalf("reason counts: %+v", got.ReasonCounts)
	}

	planID := uuid.New()
	insertMinimalRoutePlan(t, ctx, pool, tenant, planID)
	order := ChainOrderL1BeforeL2
	road := 1200.5
	candidate := ChainCandidate{
		CandidateID:          uuid.New(),
		SearchID:             pending.ID,
		TenantID:             tenant,
		Ordinal:              1,
		FirstLoadID:          uuid.MustParse("00000000-0000-4000-8000-000000000101"),
		FirstLoadVersion:     1,
		SecondLoadID:         uuid.MustParse("00000000-0000-4000-8000-000000000202"),
		SecondLoadVersion:    2,
		FeasibilityStatus:    ChainFeasible,
		SelectedOrder:        &order,
		EmptyRoadDistanceM:   &road,
		TotalDurationSeconds: nil,
		Utilization:          nil,
		OmittedComponents:    []string{"utilization", "revenue"},
		ReasonCodes:          []string{"PAIR_FEASIBLE"},
		RoutePlanID:          &planID,
		CandidateFingerprint: "cand-fp",
		CreatedAt:            completed,
	}
	if err := store.WithinChainSearch(ctx, func(tx ChainSearchTx) error {
		return tx.InsertChainCandidate(ctx, candidate)
	}); err != nil {
		t.Fatalf("insert candidate: %v", err)
	}
	var candidates []ChainCandidate
	if err := store.WithinChainSearch(ctx, func(tx ChainSearchTx) error {
		var err error
		candidates, err = tx.ListChainCandidates(ctx, tenant, pending.ID)
		return err
	}); err != nil {
		t.Fatalf("list candidates: %v", err)
	}
	if len(candidates) != 1 || candidates[0].RoutePlanID == nil || *candidates[0].RoutePlanID != planID {
		t.Fatalf("candidate round-trip: %+v", candidates)
	}
	if candidates[0].TotalDurationSeconds != nil || candidates[0].EmptyRoadDistanceM == nil {
		t.Fatalf("nullable metrics: %+v", candidates[0])
	}
	if err := store.WithinChainSearch(ctx, func(tx ChainSearchTx) error {
		list, err := tx.ListChainCandidates(ctx, otherTenant, pending.ID)
		if err != nil {
			return err
		}
		if len(list) != 0 {
			t.Fatalf("cross-tenant candidates leaked: %d", len(list))
		}
		return nil
	}); err != nil {
		t.Fatalf("cross-tenant list: %v", err)
	}

	idem := ChainSearchIdempotency{
		TenantID:           tenant,
		IdempotencyKey:     "idem-1",
		RequestFingerprint: pending.RequestFingerprint,
		SearchID:           pending.ID,
		CreatedAt:          created,
	}
	if err := store.WithinChainSearch(ctx, func(tx ChainSearchTx) error {
		return tx.InsertChainSearchIdempotency(ctx, idem)
	}); err != nil {
		t.Fatalf("insert idempotency: %v", err)
	}
	var gotIdem ChainSearchIdempotency
	if err := store.WithinChainSearch(ctx, func(tx ChainSearchTx) error {
		var err error
		gotIdem, err = tx.GetChainSearchIdempotency(ctx, tenant, "idem-1")
		return err
	}); err != nil {
		t.Fatalf("get idempotency: %v", err)
	}
	if gotIdem.SearchID != pending.ID {
		t.Fatalf("idempotency round-trip: %+v", gotIdem)
	}
	if err := store.WithinChainSearch(ctx, func(tx ChainSearchTx) error {
		_, err := tx.GetChainSearchIdempotency(ctx, otherTenant, "idem-1")
		if !errors.Is(err, ErrNotFound) {
			return err
		}
		return nil
	}); err != nil {
		t.Fatalf("cross-tenant idempotency: %v", err)
	}
}
