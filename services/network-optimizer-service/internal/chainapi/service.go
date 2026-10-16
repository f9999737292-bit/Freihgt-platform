package chainapi

import (
	"context"
	"errors"
	"sync/atomic"

	"github.com/google/uuid"

	"github.com/freight-platform/network-optimizer-service/internal/chain"
	"github.com/freight-platform/network-optimizer-service/internal/chainorch"
	"github.com/freight-platform/network-optimizer-service/internal/compat"
	"github.com/freight-platform/network-optimizer-service/internal/reference"
	"github.com/freight-platform/network-optimizer-service/internal/repository"
	"github.com/freight-platform/network-optimizer-service/internal/routing"
	"github.com/freight-platform/network-optimizer-service/internal/service"
)

// API translates public chain-search HTTP into I2B claim and execution.
// It does not reimplement deadline, fencing, cancellation, or recovery.
type API struct {
	Orch                   *chainorch.Orchestrator
	Seeds                  SeedResolver
	Pool                   LoadPool
	Catalog                reference.Catalog
	Route                  routing.Provider
	RoutingProviderVersion string
	computeInvocations     atomic.Int64
}

func (a *API) ComputeInvocations() int64 {
	if a == nil {
		return 0
	}
	return a.computeInvocations.Load()
}

type PostResult struct {
	Status int
	Body   SearchResource
}

func (a *API) Post(ctx context.Context, actor service.Actor, body []byte, idempotencyHeader string, idempotencySupplied bool) (PostResult, error) {
	if actor.TenantID == uuid.Nil || actor.UserID == uuid.Nil {
		return PostResult{}, &StatusError{Status: 401, Code: "UNAUTHORIZED", Message: "verified tenant context is required", Details: map[string]any{}}
	}
	if actor.CompanyID == nil {
		return PostResult{}, badRequest("X-Company-ID is required")
	}
	seedReq, err := parsePublicSeed(body)
	if err != nil {
		return PostResult{}, err
	}
	if err := a.postReady(); err != nil {
		return PostResult{}, err
	}
	key, _, err := parseIdempotencyKey(idempotencyHeader, idempotencySupplied)
	if err != nil {
		return PostResult{}, err
	}
	resolved, err := a.resolve(ctx, actor, seedReq)
	if err != nil {
		return PostResult{}, err
	}
	loads, err := a.loads(ctx, actor)
	if err != nil {
		return PostResult{}, unavailable(err)
	}
	fp, err := a.Fingerprint(ctx, actor, resolved)
	if err != nil {
		return PostResult{}, unavailable(err)
	}
	claim, err := a.Orch.Claim(ctx, chainorch.ClaimRequest{TenantID: actor.TenantID, IdempotencyKey: key, Fingerprint: fp})
	if err != nil && claim.Outcome != chainorch.OutcomeFingerprintConflict && claim.Outcome != chainorch.OutcomeInProgress {
		if errors.Is(err, chainorch.ErrFingerprintConflict) || errors.Is(err, chainorch.ErrInProgress) {
			// Claim still returns the outcome alongside the error.
		} else {
			return PostResult{}, unavailable(err)
		}
	}
	switch claim.Outcome {
	case chainorch.OutcomeFingerprintConflict:
		return PostResult{}, &StatusError{Status: 409, Code: "FINGERPRINT_CONFLICT", Message: "idempotency key was used with a different request", Details: map[string]any{}}
	case chainorch.OutcomeInProgress:
		return PostResult{}, &StatusError{
			Status: 409, Code: "CHAIN_SEARCH_IN_PROGRESS", Message: "chain search is already in progress",
			Details: map[string]any{"search_id": claim.Search.ID.String(), "chain_search_id": claim.Search.ID.String()},
			Headers: map[string]string{"Retry-After": "1"},
		}
	case chainorch.OutcomeReplay, chainorch.OutcomeInterruptedRecovered:
		rows, err := a.candidates(ctx, actor.TenantID, claim.Search.ID)
		if err != nil {
			return PostResult{}, unavailable(err)
		}
		return PostResult{Status: 200, Body: projectSearch(claim.Search, rows, true)}, nil
	case chainorch.OutcomeCreated:
		a.computeInvocations.Add(1)
		base := chain.Request{
			TenantID: actor.TenantID, Seed: resolved.ChainSeed, Loads: loads,
			VehicleProfile: resolved.Vehicle, Route: a.Route,
			Groupage: a.groupage(ctx, actor.TenantID),
		}
		out, err := a.Orch.ExecuteOwnedDurableChainSearch(ctx, actor.TenantID, claim.Search.ID, claim.OwnerToken, chain.Engine{}, base, chainorch.ExecuteOwnedOptions{})
		if err != nil {
			return PostResult{}, unavailable(err)
		}
		return PostResult{Status: 201, Body: projectSearch(out.Search, out.Candidates, true)}, nil
	default:
		return PostResult{}, unavailable(errors.New("unexpected claim outcome"))
	}
}

func (a *API) Get(ctx context.Context, actor service.Actor, searchID uuid.UUID) (SearchResource, error) {
	run, err := a.read(ctx, actor, searchID)
	if err != nil {
		return SearchResource{}, err
	}
	return projectSearch(run, nil, false), nil
}

func (a *API) Candidates(ctx context.Context, actor service.Actor, searchID uuid.UUID) (CandidateList, error) {
	run, err := a.read(ctx, actor, searchID)
	if err != nil {
		return CandidateList{}, err
	}
	if run.Status != repository.ChainSearchCompleted {
		return CandidateList{Candidates: []PublicCandidate{}}, nil
	}
	rows, err := a.candidates(ctx, actor.TenantID, searchID)
	if err != nil {
		return CandidateList{}, unavailable(err)
	}
	return CandidateList{Candidates: projectCandidates(rows)}, nil
}

func (a *API) read(ctx context.Context, actor service.Actor, searchID uuid.UUID) (repository.ChainSearchRun, error) {
	if a == nil || a.Orch == nil || a.Orch.Store == nil || actor.TenantID == uuid.Nil {
		return repository.ChainSearchRun{}, unavailable(errors.New("chain orchestrator is not configured"))
	}
	if _, err := a.Orch.RecoverExpired(ctx, actor.TenantID, searchID); err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return repository.ChainSearchRun{}, notFound()
		}
		return repository.ChainSearchRun{}, unavailable(err)
	}
	var run repository.ChainSearchRun
	err := a.Orch.Store.WithinChainSearch(ctx, func(tx repository.ChainSearchTx) error {
		var getErr error
		run, getErr = tx.GetChainSearchRun(ctx, actor.TenantID, searchID)
		return getErr
	})
	if errors.Is(err, repository.ErrNotFound) {
		return repository.ChainSearchRun{}, notFound()
	}
	if err != nil {
		return repository.ChainSearchRun{}, unavailable(err)
	}
	return run, nil
}

func (a *API) resolve(ctx context.Context, actor service.Actor, seed publicSeed) (ResolvedSeed, error) {
	if a.Seeds == nil {
		return ResolvedSeed{}, unavailable(errors.New("seed resolver is not configured"))
	}
	var (
		resolved ResolvedSeed
		err      error
	)
	switch seed.SeedType {
	case PublicSeedManualCapacity:
		resolved, err = a.Seeds.ResolveManual(ctx, actor, *seed.CapacityID)
	case PublicSeedCurrentTrip:
		resolved, err = a.Seeds.ResolveTrip(ctx, actor, *seed.ShipmentID)
	default:
		return ResolvedSeed{}, badRequest("seed_type is not supported")
	}
	if errors.Is(err, ErrPredictedSeed) {
		return ResolvedSeed{}, unprocessable("predicted capacity is not a chain seed", map[string]any{"field": "seed_type"})
	}
	if errors.Is(err, ErrSeedIneligible) {
		return ResolvedSeed{}, unprocessable("seed is not eligible", map[string]any{"field": "seed_type"})
	}
	if err != nil {
		return ResolvedSeed{}, unavailable(err)
	}
	return resolved, nil
}

func (a *API) postReady() error {
	if a == nil || a.Orch == nil || a.Orch.Store == nil || a.Orch.DurationResolver == nil {
		return unavailable(errors.New("chain orchestrator is not configured"))
	}
	if a.Seeds == nil || a.Pool == nil || a.Catalog == nil || a.Route == nil {
		return unavailable(errors.New("chain search dependency is not configured"))
	}
	return nil
}

func (a *API) loads(ctx context.Context, actor service.Actor) ([]chain.VisibleLoad, error) {
	if a.Pool == nil {
		return nil, errors.New("load pool is not configured")
	}
	return a.Pool.Loads(ctx, actor)
}

func (a *API) Fingerprint(ctx context.Context, actor service.Actor, resolved ResolvedSeed) (chainorch.FingerprintInput, error) {
	catalogFP, err := catalogFingerprint(ctx, a.Catalog, actor.TenantID)
	if err != nil {
		return chainorch.FingerprintInput{}, err
	}
	routingVersion := a.RoutingProviderVersion
	if routingVersion == "" {
		routingVersion = "UNKNOWN"
	}
	policy := resolved.HorizonPolicy
	if resolved.InternalSource == repository.ChainSeedCurrentTripResidual {
		policy = chainorch.HorizonAnchorSearchClock
	}
	return chainorch.FingerprintInput{
		TenantID: actor.TenantID, PolicyVersion: chain.PolicyVersion, AlgorithmVersion: chainorch.AlgorithmVersion,
		CatalogFingerprint: catalogFP, RoutingProviderVersion: routingVersion,
		SeedSource: resolved.InternalSource, SeedCapacityID: resolved.CapacityID, SeedCapacityVersion: resolved.CapacityVersion,
		SeedShipmentID: resolved.ShipmentID, SeedShipmentVersion: resolved.ShipmentVersion,
		AvailableFrom: resolved.AvailableFrom, AvailableUntil: resolved.AvailableUntil, HorizonAnchorPolicy: policy,
		PublicationVisibility: publicationFingerprint(actor), FilterFingerprint: filterFingerprint,
	}, nil
}

func (a *API) candidates(ctx context.Context, tenant, searchID uuid.UUID) ([]repository.ChainCandidate, error) {
	var rows []repository.ChainCandidate
	err := a.Orch.Store.WithinChainSearch(ctx, func(tx repository.ChainSearchTx) error {
		var listErr error
		rows, listErr = tx.ListChainCandidates(ctx, tenant, searchID)
		return listErr
	})
	return rows, err
}

func (a *API) groupage(ctx context.Context, tenant uuid.UUID) func([]compat.GroupageItem) compat.Result {
	return func(items []compat.GroupageItem) compat.Result {
		if a.Catalog == nil {
			return compat.Result{Status: compat.StatusCompatible}
		}
		eval, err := a.Catalog.Evaluation(ctx, tenant)
		if err != nil {
			return compat.Result{Status: compat.StatusIndeterminate}
		}
		return compat.EvaluateGroupageItems(compat.Equipment{}, items, eval)
	}
}
