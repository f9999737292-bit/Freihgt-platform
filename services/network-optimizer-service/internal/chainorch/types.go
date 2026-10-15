// Package chainorch implements NLO-0.9C-I2B durable claim/recovery orchestration
// on top of the I2A ChainSearchStore primitives. It does not expose HTTP.
package chainorch

import (
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
)

var (
	ErrComputeDeadlinePassed = errors.New("chain compute deadline passed")
	ErrNotOwner              = errors.New("chain search owner fencing failed")
	ErrInvalidCancelReason   = errors.New("invalid cancel_reason_class")
	ErrDurationSnapshot      = errors.New("service duration snapshot cannot be consumed safely")
	ErrFingerprintConflict   = errors.New("idempotency fingerprint conflict")
	ErrInProgress            = errors.New("chain search in progress")
	errFencedDiscard         = errors.New("owner fenced output discarded")
)

// FingerprintInput is the frozen request material hashed into request_fingerprint.
// Service-duration provenance must already be resolved before hashing.
type FingerprintInput struct {
	PolicyVersion          string
	AlgorithmVersion       string
	CatalogFingerprint     string
	RoutingProviderVersion string
	SeedSource             string
	SeedCapacityID         *uuid.UUID
	SeedCapacityVersion    *int
	SeedShipmentID         *uuid.UUID
	SeedShipmentVersion    *int
	HorizonStart           time.Time
	HorizonEnd             time.Time
	LoadIDsVersions        []LoadRef
	PublicationVisibility  string
	Duration               repository.ServiceDurationSnapshot
}

// LoadRef is a load identity included in the request fingerprint.
type LoadRef struct {
	ID      uuid.UUID
	Version int
}

// ClaimRequest creates a PENDING search + idempotency mapping atomically.
type ClaimRequest struct {
	TenantID       uuid.UUID
	IdempotencyKey string
	SearchID       uuid.UUID // generated when uuid.Nil
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
