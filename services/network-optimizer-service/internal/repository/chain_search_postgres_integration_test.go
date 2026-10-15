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
	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Minute)
	defer cancel()
	pool := startPostgres(t, ctx)
	applyThrough000097(t, ctx, pool)
	store := NewPostgres(pool)

	tenant := uuid.New()
	otherTenant := uuid.New()
	createdHint := time.Now().UTC()
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
		RequestFingerprint:           "req-fp-pending",
		PolicyVersion:                "NLO-0.9C-I0/BOUNDED_TWO_LOAD",
		AlgorithmVersion:             "BOUNDED_TWO_ORDER_COMPARISON",
		CatalogFingerprint:           "catalog-fp",
		RoutingProviderVersion:       "UNKNOWN",
		OwnerToken:                   owner,
		ServiceDurationPolicyID:      &policyID,
		ServiceDurationPolicyVersion: &policyVersion,
		ServiceDurationSnapshot:      knownZero,
		HorizonStart:                 createdHint,
		HorizonEnd:                   createdHint.Add(48 * time.Hour),
		SeedSource:                   ChainSeedManualAvailableCapacity,
		SeedCapacityID:               &capacityID,
		SeedCapacityVersion:          &capacityVersion,
		CandidateCount:               0,
		EvaluatedPairCount:           0,
		ProviderCallCount:            0,
		SearchComplete:               false,
		ReasonCounts:                 map[string]int{},
	}

	var inserted ChainSearchRun
	if err := store.WithinChainSearch(ctx, func(tx ChainSearchTx) error {
		var err error
		inserted, err = tx.InsertChainSearchRun(ctx, pending)
		return err
	}); err != nil {
		t.Fatalf("insert pending: %v", err)
	}
	if !inserted.DeadlineAt.Equal(inserted.CreatedAt.Add(20*time.Second)) || !inserted.ClaimExpiresAt.Equal(inserted.DeadlineAt.Add(5*time.Second)) {
		t.Fatalf("DB claim window mismatch created=%v deadline=%v claim=%v", inserted.CreatedAt, inserted.DeadlineAt, inserted.ClaimExpiresAt)
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

	if err := store.WithinChainSearch(ctx, func(tx ChainSearchTx) error {
		_, err := tx.GetChainSearchRun(ctx, otherTenant, pending.ID)
		if !errors.Is(err, ErrNotFound) {
			return err
		}
		return nil
	}); err != nil {
		t.Fatalf("cross-tenant get: %v", err)
	}

	if err := store.WithinChainSearch(ctx, func(tx ChainSearchTx) error {
		ok, err := tx.MarkChainSearchRunning(ctx, tenant, pending.ID, owner)
		if err != nil {
			return err
		}
		if !ok {
			t.Fatal("mark running missed")
		}
		return nil
	}); err != nil {
		t.Fatalf("mark running: %v", err)
	}

	completed := time.Now().UTC()
	terminal := got
	terminal.Status = ChainSearchCompleted
	terminal.CompletedAt = &completed
	terminal.SearchComplete = true
	terminal.CandidateCount = 1
	terminal.EvaluatedPairCount = 2
	terminal.ProviderCallCount = 3
	terminal.ServiceDurationPolicyID = nil
	terminal.ServiceDurationPolicyVersion = nil
	terminal.ServiceDurationSnapshot = ServiceDurationSnapshot{State: ServiceDurationUnknown, PickupKnown: false, DeliveryKnown: false}
	terminal.ReasonCounts = map[string]int{"STACKING_UNKNOWN": 2}

	if err := store.WithinChainSearch(ctx, func(tx ChainSearchTx) error {
		if err := tx.LockChainSearchRun(ctx, tenant, pending.ID); err != nil {
			return err
		}
		ok, err := tx.OwnerPersistChainSearchRun(ctx, terminal, owner, ChainSearchRunning)
		if err != nil {
			return err
		}
		if !ok {
			t.Fatal("owner persist missed")
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
	if got.ServiceDurationSnapshot.State != ServiceDurationKnown || got.ServiceDurationSnapshot.PickupSeconds == nil || *got.ServiceDurationSnapshot.PickupSeconds != 0 {
		t.Fatalf("claim-time known-zero snapshot must remain immutable: %+v", got.ServiceDurationSnapshot)
	}
	if got.ServiceDurationPolicyID == nil || *got.ServiceDurationPolicyID != policyID || got.ServiceDurationPolicyVersion == nil || *got.ServiceDurationPolicyVersion != policyVersion {
		t.Fatalf("claim-time policy provenance must remain immutable: id=%v ver=%v", got.ServiceDurationPolicyID, got.ServiceDurationPolicyVersion)
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
		CreatedAt:          inserted.CreatedAt,
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

func TestI2A_PG_IMMUTABLE_01_ServiceDurationSnapshot(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Minute)
	defer cancel()
	pool := startPostgres(t, ctx)
	applyThrough000097(t, ctx, pool)
	store := NewPostgres(pool)

	tenant := uuid.New()
	createdHint := time.Now().UTC()
	owner := uuid.New()
	policyID := uuid.New()
	policyVersion := 3
	zero := 0
	capacityID := uuid.New()
	capacityVersion := 1
	original := ChainSearchRun{
		ID:                           uuid.New(),
		TenantID:                     tenant,
		Status:                       ChainSearchPending,
		RequestFingerprint:           "req-fp-immutable",
		PolicyVersion:                "NLO-0.9C-I0/BOUNDED_TWO_LOAD",
		AlgorithmVersion:             "BOUNDED_TWO_ORDER_COMPARISON",
		CatalogFingerprint:           "catalog-fp",
		RoutingProviderVersion:       "UNKNOWN",
		OwnerToken:                   owner,
		ServiceDurationPolicyID:      &policyID,
		ServiceDurationPolicyVersion: &policyVersion,
		ServiceDurationSnapshot: ServiceDurationSnapshot{
			State:           ServiceDurationKnown,
			PolicyID:        &policyID,
			PolicyVersion:   &policyVersion,
			PickupKnown:     true,
			PickupSeconds:   &zero,
			DeliveryKnown:   true,
			DeliverySeconds: &zero,
		},
		HorizonStart:        createdHint,
		HorizonEnd:          createdHint.Add(48 * time.Hour),
		SeedSource:          ChainSeedManualAvailableCapacity,
		SeedCapacityID:      &capacityID,
		SeedCapacityVersion: &capacityVersion,
		ReasonCounts:        map[string]int{},
	}
	if err := store.WithinChainSearch(ctx, func(tx ChainSearchTx) error {
		_, err := tx.InsertChainSearchRun(ctx, original)
		return err
	}); err != nil {
		t.Fatalf("insert: %v", err)
	}
	if err := store.WithinChainSearch(ctx, func(tx ChainSearchTx) error {
		ok, err := tx.MarkChainSearchRunning(ctx, tenant, original.ID, owner)
		if err != nil {
			return err
		}
		if !ok {
			t.Fatal("mark running missed")
		}
		return nil
	}); err != nil {
		t.Fatalf("mark running: %v", err)
	}

	completed := time.Now().UTC()
	poisoned := original
	poisoned.Status = ChainSearchCompleted
	poisoned.CompletedAt = &completed
	poisoned.SearchComplete = true
	poisoned.ServiceDurationPolicyID = nil
	poisoned.ServiceDurationPolicyVersion = nil
	poisoned.ServiceDurationSnapshot = ServiceDurationSnapshot{
		State:         ServiceDurationUnknown,
		PickupKnown:   false,
		DeliveryKnown: false,
	}
	poisoned.ReasonCounts = map[string]int{"X": 1}

	if err := store.WithinChainSearch(ctx, func(tx ChainSearchTx) error {
		ok, err := tx.OwnerPersistChainSearchRun(ctx, poisoned, owner, ChainSearchRunning)
		if err != nil {
			return err
		}
		if !ok {
			t.Fatal("update missed")
		}
		return nil
	}); err != nil {
		t.Fatalf("update: %v", err)
	}

	var got ChainSearchRun
	if err := store.WithinChainSearch(ctx, func(tx ChainSearchTx) error {
		var err error
		got, err = tx.GetChainSearchRun(ctx, tenant, original.ID)
		return err
	}); err != nil {
		t.Fatalf("get: %v", err)
	}
	if got.Status != ChainSearchCompleted {
		t.Fatalf("status=%s", got.Status)
	}
	if got.ServiceDurationPolicyID == nil || *got.ServiceDurationPolicyID != policyID {
		t.Fatalf("policy id mutated: %v", got.ServiceDurationPolicyID)
	}
	if got.ServiceDurationPolicyVersion == nil || *got.ServiceDurationPolicyVersion != policyVersion {
		t.Fatalf("policy version mutated: %v", got.ServiceDurationPolicyVersion)
	}
	if got.ServiceDurationSnapshot.State != ServiceDurationKnown {
		t.Fatalf("snapshot state mutated: %+v", got.ServiceDurationSnapshot)
	}
	if got.ServiceDurationSnapshot.PickupSeconds == nil || *got.ServiceDurationSnapshot.PickupSeconds != 0 {
		t.Fatalf("KNOWN_ZERO pickup lost: %+v", got.ServiceDurationSnapshot)
	}
	if got.ServiceDurationSnapshot.DeliverySeconds == nil || *got.ServiceDurationSnapshot.DeliverySeconds != 0 {
		t.Fatalf("KNOWN_ZERO delivery lost: %+v", got.ServiceDurationSnapshot)
	}
}
