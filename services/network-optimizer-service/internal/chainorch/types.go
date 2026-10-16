// Package chainorch implements NLO-0.9C-I2B durable claim/recovery orchestration
// on top of the I2A ChainSearchStore primitives. It does not expose HTTP.
package chainorch

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"

	"github.com/freight-platform/network-optimizer-service/internal/repository"
)

// Internal orchestration outcomes (I2C maps these to HTTP later).
const (
	OutcomeCreated              = "CREATED"
	OutcomeReplay               = "REPLAY"
	OutcomeInProgress           = "CHAIN_SEARCH_IN_PROGRESS"
	OutcomeFingerprintConflict  = "FINGERPRINT_CONFLICT"
	OutcomeInterruptedRecovered = "INTERRUPTED_RECOVERED"
)

// Cancel reason classes frozen by I1.
const (
	CancelClientCancelled  = "CLIENT_CANCELLED"
	CancelGatewayCancelled = "GATEWAY_CANCELLED"
	CancelContextCancelled = "CONTEXT_CANCELLED"
)

const (
	DefaultCleanupTimeout = 4 * time.Second
	AlgorithmVersion      = "BOUNDED_TWO_ORDER_COMPARISON"
	// HorizonAnchorSearchClock is the frozen policy marker for search-clock-derived horizons.
	// It must not embed a wall-clock instant so replay stays stable.
	HorizonAnchorSearchClock = "SEARCH_CLOCK+48H"

	// Durable failure codes (FAILED.status + failure_code). Not new search statuses.
	FailureBudgetExhausted         = "CHAIN_BUDGET_EXHAUSTED"
	FailureUnexpectedEngineStatus  = "CHAIN_UNEXPECTED_ENGINE_STATUS"
	FailureEngineError             = "CHAIN_ENGINE_ERROR"
	FailureTerminalizeIncomplete   = "CHAIN_TERMINALIZE_INCOMPLETE"
)

var (
	ErrComputeDeadlinePassed = errors.New("chain compute deadline passed")
	ErrNotOwner              = errors.New("chain search owner fencing failed")
	ErrInvalidCancelReason   = errors.New("invalid cancel_reason_class")
	ErrDurationSnapshot      = errors.New("service duration snapshot cannot be consumed safely")
	ErrFingerprintConflict   = errors.New("idempotency fingerprint conflict")
	ErrInProgress            = errors.New("chain search in progress")
	ErrDurationResolverRequired = errors.New("DurationResolver is required")
	ErrSearchNotRunnable        = errors.New("chain search is not runnable")
	errFencedDiscard            = errors.New("owner fenced output discarded")
)

// DurationResolver is the server-owned active-policy source for claim provenance.
type DurationResolver interface {
	ResolveServiceDuration(ctx context.Context, tenant uuid.UUID) (repository.ServiceDurationSnapshot, error)
}

// FixedDurationResolver injects a pre-resolved snapshot (tests / offline seams only).
type FixedDurationResolver struct {
	Snapshot repository.ServiceDurationSnapshot
}

func (f FixedDurationResolver) ResolveServiceDuration(context.Context, uuid.UUID) (repository.ServiceDurationSnapshot, error) {
	return f.Snapshot, nil
}

// PostgresDurationResolver resolves from the tenant ACTIVE service-duration policy.
type PostgresDurationResolver struct {
	Store *repository.Postgres
}

func (p PostgresDurationResolver) ResolveServiceDuration(ctx context.Context, tenant uuid.UUID) (repository.ServiceDurationSnapshot, error) {
	return ResolveActiveDurationSnapshot(ctx, p.Store, tenant)
}

// FingerprintInput is the frozen request material hashed into request_fingerprint.
// Duration is overwritten by Claim from DurationResolver before hashing.
// Horizon wall-clock instants are not idempotency material; use anchors/policy markers.
type FingerprintInput struct {
	TenantID               uuid.UUID // overwritten from ClaimRequest.TenantID
	PolicyVersion          string
	AlgorithmVersion       string
	CatalogFingerprint     string
	RoutingProviderVersion string
	SeedSource             string
	SeedCapacityID         *uuid.UUID
	SeedCapacityVersion    *int
	SeedShipmentID         *uuid.UUID
	SeedShipmentVersion    *int
	AvailableFrom          *time.Time // manual capacity absolute anchor
	AvailableUntil         *time.Time // manual capacity absolute anchor
	HorizonAnchorPolicy    string     // e.g. HorizonAnchorSearchClock
	PublicationVisibility  string
	FilterFingerprint      string // deterministic server-applied filter material
	Duration               repository.ServiceDurationSnapshot
}

// ClaimRequest creates a PENDING search + idempotency mapping atomically.
// Search IDs are always generated internally; callers cannot choose them.
// Persisted horizon bounds are derived server-side from fingerprint anchors
// (AvailableFrom/Until or SEARCH_CLOCK+48H) — callers cannot inject them.
type ClaimRequest struct {
	TenantID       uuid.UUID
	IdempotencyKey string
	Fingerprint    FingerprintInput
}

// ClaimResult is the internal orchestration result of Claim.
type ClaimResult struct {
	Outcome    string
	Search     repository.ChainSearchRun
	OwnerToken uuid.UUID // non-zero only for OutcomeCreated (this process owns compute)
	Created    bool
}

// TerminalWrite carries candidate shortlist + terminal run fields for one fenced TX.
type TerminalWrite struct {
	Run        repository.ChainSearchRun
	Candidates []repository.ChainCandidate
}

// TerminalStats is the frozen durable counter/reason payload for normal in-process
// terminalization after a real engine result. Crash recovery (CHAIN_INTERRUPTED)
// does not invent these values.
type TerminalStats struct {
	CandidateCount     int
	EvaluatedPairCount int
	ProviderCallCount  int
	ReasonCounts       map[string]int
}

// IsTerminal reports whether status is COMPLETED/CANCELLED/FAILED.
func IsTerminal(status string) bool {
	switch status {
	case repository.ChainSearchCompleted, repository.ChainSearchCancelled, repository.ChainSearchFailed:
		return true
	default:
		return false
	}
}

// ValidCancelReason reports whether class is in the frozen set.
func ValidCancelReason(class string) bool {
	switch class {
	case CancelClientCancelled, CancelGatewayCancelled, CancelContextCancelled:
		return true
	default:
		return false
	}
}
