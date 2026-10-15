package chainorch

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"

	"github.com/freight-platform/network-optimizer-service/internal/repository"
)

// Orchestrator performs I2B claim, fencing, cancellation, and recovery.
// It never renews leases, takes over ownership, or auto-recomputes.
type Orchestrator struct {
	Store            repository.ChainSearchStore
	DurationResolver DurationResolver
	Now              func() time.Time
	CleanupTimeout   time.Duration
}

func (o *Orchestrator) now() time.Time {
	if o.Now != nil {
		return o.Now().UTC()
	}
	return time.Now().UTC()
}

func (o *Orchestrator) cleanupTimeout() time.Duration {
	if o.CleanupTimeout > 0 {
		return o.CleanupTimeout
	}
	return DefaultCleanupTimeout
}

// Claim atomically inserts PENDING search + idempotency, or resolves collisions.
// No routing/provider/chain computation runs inside this method.
// Duration provenance is always server-resolved; caller Fingerprint.Duration is ignored.
// Search IDs are always generated internally.
func (o *Orchestrator) Claim(ctx context.Context, req ClaimRequest) (ClaimResult, error) {
	if req.TenantID == uuid.Nil || req.IdempotencyKey == "" {
		return ClaimResult{}, errors.New("tenant_id and idempotency_key required")
	}
	snap, err := o.resolveDuration(ctx, req.TenantID)
	if err != nil {
		return ClaimResult{}, err
	}
	fpIn := req.Fingerprint
	fpIn.TenantID = req.TenantID
	fpIn.Duration = snap
	if fpIn.AlgorithmVersion == "" {
		fpIn.AlgorithmVersion = AlgorithmVersion
	}
	if fpIn.HorizonAnchorPolicy == "" && fpIn.AvailableFrom == nil && fpIn.AvailableUntil == nil {
		fpIn.HorizonAnchorPolicy = HorizonAnchorSearchClock
	}
	fp := ComputeRequestFingerprint(fpIn)
	searchID := uuid.New()

	horizonStart := req.HorizonStart
	horizonEnd := req.HorizonEnd
	if horizonStart.IsZero() || horizonEnd.IsZero() {
		horizonStart, horizonEnd = derivePersistedHorizon(fpIn, o.now())
	}

	run := repository.ChainSearchRun{
		ID:                           searchID,
		TenantID:                     req.TenantID,
		RequestFingerprint:           fp,
		PolicyVersion:                fpIn.PolicyVersion,
		AlgorithmVersion:             fpIn.AlgorithmVersion,
		CatalogFingerprint:           fpIn.CatalogFingerprint,
		RoutingProviderVersion:       fpIn.RoutingProviderVersion,
		ServiceDurationPolicyID:      snap.PolicyID,
		ServiceDurationPolicyVersion: snap.PolicyVersion,
		ServiceDurationSnapshot:      snap,
		HorizonStart:                 horizonStart,
		HorizonEnd:                   horizonEnd,
		SeedSource:                   fpIn.SeedSource,
		SeedCapacityID:               fpIn.SeedCapacityID,
		SeedCapacityVersion:          fpIn.SeedCapacityVersion,
		SeedShipmentID:               fpIn.SeedShipmentID,
		SeedShipmentVersion:          fpIn.SeedShipmentVersion,
		ReasonCounts:                 map[string]int{},
	}

	var created repository.ChainSearchRun
	err = o.Store.WithinChainSearch(ctx, func(tx repository.ChainSearchTx) error {
		inserted, err := tx.InsertChainSearchRun(ctx, run)
		if err != nil {
			return err
		}
		if err := tx.InsertChainSearchIdempotency(ctx, repository.ChainSearchIdempotency{
			TenantID:           req.TenantID,
			IdempotencyKey:     req.IdempotencyKey,
			RequestFingerprint: fp,
			SearchID:           inserted.ID,
			CreatedAt:          inserted.CreatedAt,
		}); err != nil {
			return err
		}
		created = inserted
		return nil
	})
	if err == nil {
		return ClaimResult{
			Outcome:    OutcomeCreated,
			Search:     created,
			OwnerToken: created.OwnerToken,
			Created:    true,
		}, nil
	}
	// Unique collision on idempotency key: losing attempt left neither row (TX rolled back).
	if !errors.Is(err, repository.ErrIdempotencyRace) && !errors.Is(err, repository.ErrConflict) {
		return ClaimResult{}, err
	}
	return o.resolveIdempotencyCollision(ctx, req.TenantID, req.IdempotencyKey, fp)
}

func (o *Orchestrator) resolveDuration(ctx context.Context, tenant uuid.UUID) (repository.ServiceDurationSnapshot, error) {
	if o.DurationResolver == nil {
		return UnknownDurationSnapshot(), nil
	}
	return o.DurationResolver.ResolveServiceDuration(ctx, tenant)
}

func derivePersistedHorizon(in FingerprintInput, clock time.Time) (time.Time, time.Time) {
	start := clock.UTC()
	if in.AvailableFrom != nil && !in.AvailableFrom.IsZero() {
		start = in.AvailableFrom.UTC()
	}
	end := start.Add(48 * time.Hour)
	if in.AvailableUntil != nil && !in.AvailableUntil.IsZero() && in.AvailableUntil.Before(end) {
		end = in.AvailableUntil.UTC()
	}
	return start, end
}

func (o *Orchestrator) resolveIdempotencyCollision(ctx context.Context, tenant uuid.UUID, key, fp string) (ClaimResult, error) {
	var mapping repository.ChainSearchIdempotency
	var search repository.ChainSearchRun
	err := o.Store.WithinChainSearch(ctx, func(tx repository.ChainSearchTx) error {
		var err error
		mapping, err = tx.GetChainSearchIdempotency(ctx, tenant, key)
		if err != nil {
			return err
		}
		if mapping.RequestFingerprint != fp {
			return ErrFingerprintConflict
		}
		search, err = tx.GetChainSearchRun(ctx, tenant, mapping.SearchID)
		return err
	})
	if errors.Is(err, ErrFingerprintConflict) {
		return ClaimResult{Outcome: OutcomeFingerprintConflict, Search: search}, ErrFingerprintConflict
	}
	if err != nil {
		return ClaimResult{}, err
	}

	if IsTerminal(search.Status) {
		return ClaimResult{Outcome: OutcomeReplay, Search: search}, nil
	}

	// Nonterminal: recover if expired, else in-progress.
	recovered, err := o.RecoverExpired(ctx, tenant, search.ID)
	if err != nil {
		return ClaimResult{}, err
	}
	if recovered {
		var terminal repository.ChainSearchRun
		if err := o.Store.WithinChainSearch(ctx, func(tx repository.ChainSearchTx) error {
			var getErr error
			terminal, getErr = tx.GetChainSearchRun(ctx, tenant, search.ID)
			return getErr
		}); err != nil {
			return ClaimResult{}, err
		}
		return ClaimResult{Outcome: OutcomeInterruptedRecovered, Search: terminal}, nil
	}

	// Re-read in case concurrent recovery/completion won between checks.
	if err := o.Store.WithinChainSearch(ctx, func(tx repository.ChainSearchTx) error {
		var getErr error
		search, getErr = tx.GetChainSearchRun(ctx, tenant, search.ID)
		return getErr
	}); err != nil {
		return ClaimResult{}, err
	}
	if IsTerminal(search.Status) {
		return ClaimResult{Outcome: OutcomeReplay, Search: search}, nil
	}
	return ClaimResult{Outcome: OutcomeInProgress, Search: search}, ErrInProgress
}

// BeginRunning transitions PENDING -> RUNNING via the I2A owner/deadline primitive.
func (o *Orchestrator) BeginRunning(ctx context.Context, tenant, searchID, ownerToken uuid.UUID) (repository.ChainSearchRun, error) {
	var out repository.ChainSearchRun
	err := o.Store.WithinChainSearch(ctx, func(tx repository.ChainSearchTx) error {
		if err := tx.LockChainSearchRun(ctx, tenant, searchID); err != nil {
			return err
		}
		ok, err := tx.MarkChainSearchRunning(ctx, tenant, searchID, ownerToken)
		if err != nil {
			return err
		}
		if !ok {
			return ErrNotOwner
		}
		var getErr error
		out, getErr = tx.GetChainSearchRun(ctx, tenant, searchID)
		return getErr
	})
	return out, err
}

// CompleteSearch persists candidates and COMPLETED in one fenced transaction.
func (o *Orchestrator) CompleteSearch(ctx context.Context, ownerToken uuid.UUID, expectedStatus string, write TerminalWrite) (repository.ChainSearchRun, bool, error) {
	if write.Run.Status != repository.ChainSearchCompleted {
		return repository.ChainSearchRun{}, false, errors.New("CompleteSearch requires COMPLETED status")
	}
	write.Run.SearchComplete = true
	write.Run.GlobalOptimum = false
	write.Run.ExecutionSupported = false
	write.Run.FailureCode = nil
	write.Run.CancelReasonClass = nil

	var out repository.ChainSearchRun
	err := o.Store.WithinChainSearch(ctx, func(tx repository.ChainSearchTx) error {
		if err := tx.LockChainSearchRun(ctx, write.Run.TenantID, write.Run.ID); err != nil {
			return err
		}
		for i := range write.Candidates {
			c := write.Candidates[i]
			c.SearchID = write.Run.ID
			c.TenantID = write.Run.TenantID
			if c.CandidateID == uuid.Nil {
				c.CandidateID = uuid.New()
			}
			if err := tx.OwnerInsertChainCandidate(ctx, c, ownerToken, expectedStatus); err != nil {
				if errors.Is(err, repository.ErrConflict) {
					return errFencedDiscard
				}
				return err
			}
		}
		ok, err := tx.OwnerPersistChainSearchRun(ctx, write.Run, ownerToken, expectedStatus)
		if err != nil {
			return err
		}
		if !ok {
			return errFencedDiscard
		}
		var getErr error
		out, getErr = tx.GetChainSearchRun(ctx, write.Run.TenantID, write.Run.ID)
		return getErr
	})
	if errors.Is(err, errFencedDiscard) {
		return repository.ChainSearchRun{}, false, nil
	}
	if err != nil {
		return repository.ChainSearchRun{}, false, err
	}
	return out, true, nil
}

// CancelSearch persists CANCELLED using a bounded cleanup context (not the cancelled request ctx).
func (o *Orchestrator) CancelSearch(parent context.Context, tenant, searchID, ownerToken uuid.UUID, expectedStatus, reasonClass string) (repository.ChainSearchRun, bool, error) {
	if !ValidCancelReason(reasonClass) {
		return repository.ChainSearchRun{}, false, ErrInvalidCancelReason
	}
	cleanup, cancel := context.WithTimeout(context.WithoutCancel(parent), o.cleanupTimeout())
	defer cancel()

	reason := reasonClass
	run := repository.ChainSearchRun{
		ID:                searchID,
		TenantID:          tenant,
		Status:            repository.ChainSearchCancelled,
		SearchComplete:    false,
		CancelReasonClass: &reason,
		ReasonCounts:      map[string]int{},
	}
	var out repository.ChainSearchRun
	var applied bool
	err := o.Store.WithinChainSearch(cleanup, func(tx repository.ChainSearchTx) error {
		if err := tx.LockChainSearchRun(cleanup, tenant, searchID); err != nil {
			return err
		}
		ok, err := tx.OwnerPersistChainSearchRun(cleanup, run, ownerToken, expectedStatus)
		if err != nil {
			return err
		}
		applied = ok
		if !ok {
			return nil
		}
		var getErr error
		out, getErr = tx.GetChainSearchRun(cleanup, tenant, searchID)
		return getErr
	})
	return out, applied, err
}

// FailSearch persists FAILED under owner fencing (non-recovery path).
func (o *Orchestrator) FailSearch(ctx context.Context, tenant, searchID, ownerToken uuid.UUID, expectedStatus, failureCode string, reasonCounts map[string]int) (repository.ChainSearchRun, bool, error) {
	code := failureCode
	run := repository.ChainSearchRun{
		ID:             searchID,
		TenantID:       tenant,
		Status:         repository.ChainSearchFailed,
		SearchComplete: false,
		FailureCode:    &code,
		ReasonCounts:   reasonCounts,
	}
	if run.ReasonCounts == nil {
		run.ReasonCounts = map[string]int{}
	}
	var out repository.ChainSearchRun
	var applied bool
	err := o.Store.WithinChainSearch(ctx, func(tx repository.ChainSearchTx) error {
		if err := tx.LockChainSearchRun(ctx, tenant, searchID); err != nil {
			return err
		}
		ok, err := tx.OwnerPersistChainSearchRun(ctx, run, ownerToken, expectedStatus)
		if err != nil {
			return err
		}
		applied = ok
		if !ok {
			return nil
		}
		var getErr error
		out, getErr = tx.GetChainSearchRun(ctx, tenant, searchID)
		return getErr
	})
	return out, applied, err
}

// RecoverExpired terminalizes one expired PENDING/RUNNING claim (lookup-triggered).
// Idempotent: already-terminal or unexpired returns applied=false without error.
func (o *Orchestrator) RecoverExpired(ctx context.Context, tenant, searchID uuid.UUID) (bool, error) {
	var applied bool
	err := o.Store.WithinChainSearch(ctx, func(tx repository.ChainSearchTx) error {
		if err := tx.LockChainSearchRun(ctx, tenant, searchID); err != nil {
			return err
		}
		ok, err := tx.RecoverExpiredChainSearchClaim(ctx, tenant, searchID)
		applied = ok
		return err
	})
	return applied, err
}

// RecoverExpiredBatch recovers a provided id list (startup/periodic helper).
func (o *Orchestrator) RecoverExpiredBatch(ctx context.Context, ids []repository.ExpiredChainSearchClaim) (recovered int, err error) {
	for _, id := range ids {
		ok, recErr := o.RecoverExpired(ctx, id.TenantID, id.SearchID)
		if recErr != nil {
			return recovered, recErr
		}
		if ok {
			recovered++
		}
	}
	return recovered, nil
}

// SweepExpiredClaims is the bounded startup/periodic sweep entry point.
func (o *Orchestrator) SweepExpiredClaims(ctx context.Context, limit int) (recovered int, err error) {
	var ids []repository.ExpiredChainSearchClaim
	if err := o.Store.WithinChainSearch(ctx, func(tx repository.ChainSearchTx) error {
		var listErr error
		ids, listErr = tx.ListExpiredChainSearchClaims(ctx, limit)
		return listErr
	}); err != nil {
		return 0, err
	}
	return o.RecoverExpiredBatch(ctx, ids)
}

// PersistCandidatesUnderOwner inserts candidates under parent ownership/expiry fencing.
func (o *Orchestrator) PersistCandidatesUnderOwner(ctx context.Context, tenant, searchID, ownerToken uuid.UUID, expectedStatus string, candidates []repository.ChainCandidate) (bool, error) {
	var applied bool
	err := o.Store.WithinChainSearch(ctx, func(tx repository.ChainSearchTx) error {
		if err := tx.LockChainSearchRun(ctx, tenant, searchID); err != nil {
			return err
		}
		for i := range candidates {
			c := candidates[i]
			c.SearchID = searchID
			c.TenantID = tenant
			if c.CandidateID == uuid.Nil {
				c.CandidateID = uuid.New()
			}
			if err := tx.OwnerInsertChainCandidate(ctx, c, ownerToken, expectedStatus); err != nil {
				if errors.Is(err, repository.ErrConflict) {
					return errFencedDiscard
				}
				return err
			}
		}
		applied = true
		return nil
	})
	if errors.Is(err, errFencedDiscard) {
		return false, nil
	}
	return applied, err
}

// ListCandidates returns durable candidates for a tenant-owned search.
func (o *Orchestrator) ListCandidates(ctx context.Context, tenant, searchID uuid.UUID) ([]repository.ChainCandidate, error) {
	var out []repository.ChainCandidate
	err := o.Store.WithinChainSearch(ctx, func(tx repository.ChainSearchTx) error {
		var getErr error
		out, getErr = tx.ListChainCandidates(ctx, tenant, searchID)
		return getErr
	})
	return out, err
}

// GetSearch returns a tenant-scoped search row.
func (o *Orchestrator) GetSearch(ctx context.Context, tenant, searchID uuid.UUID) (repository.ChainSearchRun, error) {
	var out repository.ChainSearchRun
	err := o.Store.WithinChainSearch(ctx, func(tx repository.ChainSearchTx) error {
		var getErr error
		out, getErr = tx.GetChainSearchRun(ctx, tenant, searchID)
		return getErr
	})
	return out, err
}

// LookupTriggeredRecover loads a search and recovers it when expired nonterminal.
func (o *Orchestrator) LookupTriggeredRecover(ctx context.Context, tenant, searchID uuid.UUID) (repository.ChainSearchRun, error) {
	run, err := o.GetSearch(ctx, tenant, searchID)
	if err != nil {
		return repository.ChainSearchRun{}, err
	}
	if IsTerminal(run.Status) {
		return run, nil
	}
	if _, err := o.RecoverExpired(ctx, tenant, searchID); err != nil {
		return repository.ChainSearchRun{}, err
	}
	return o.GetSearch(ctx, tenant, searchID)
}
