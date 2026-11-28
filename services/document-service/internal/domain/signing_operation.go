package domain

import (
	"strings"

	"github.com/google/uuid"

	apperrors "github.com/freight-platform/document-service/internal/platform/errors"
)

// Technical states for one signing operation. They do not replace
// SigningSession or Document statuses. PREPARED is not SIGNED, and none of
// these states is cryptographic verification or operator confirmation.
const (
	SigningOperationStatePrepared             = "PREPARED"
	SigningOperationStateResultUnknown        = "RESULT_UNKNOWN"
	SigningOperationStateManualReviewRequired = "MANUAL_REVIEW_REQUIRED"
	SigningOperationStateFailedRetryable      = "FAILED_RETRYABLE"
	SigningOperationStateFailedFinal          = "FAILED_FINAL"

	SigningOperationEvidenceLocalAttempt               = "LOCAL_ATTEMPT_RECORDED"
	SigningOperationEvidenceExternalResponseAbsent     = "EXTERNAL_RESPONSE_ABSENT"
	SigningOperationEvidenceProviderRejectionConfirmed = "PROVIDER_REJECTION_CONFIRMED"
	SigningOperationEvidenceContradiction              = "CONTRADICTION"

	SigningOperationEvidenceSourceServer                = "SERVER"
	SigningOperationEvidenceSourceAuthenticatedProvider = "AUTHENTICATED_PROVIDER"

	signingOperationNotFoundMessage = "signing operation not found"
)

// J-EDO02A-003 remains BLOCKED_POLICY_DECISION.
// This package does not add a state that cancels PREPARED, closes
// MANUAL_REVIEW_REQUIRED, records provider acceptance, or changes the
// tenant-document lock for several signers. EXTERNAL_RESPONSE_ABSENT is not
// cancellation proof. FAILED_RETRYABLE and FAILED_FINAL stay distinct
// terminal states. Whether FAILED_FINAL should hold the document scope is
// part of that policy decision and is not changed here.

// Signing operation lock scope is the pair (tenant_id, document_id).
// PREPARED, RESULT_UNKNOWN, and MANUAL_REVIEW_REQUIRED hold that scope.
// FAILED_RETRYABLE and FAILED_FINAL do not.
// Comparing records in memory is not a PostgreSQL lock and is not a
// concurrency proof.
func SigningOperationStateBlocksDocument(state string) bool {
	switch state {
	case SigningOperationStatePrepared, SigningOperationStateResultUnknown, SigningOperationStateManualReviewRequired:
		return true
	default:
		return false
	}
}

// SigningOperationIdentity is the full identity of one attempt.
// A content hash alone is not an operation id.
type SigningOperationIdentity struct {
	OperationID     uuid.UUID
	TenantID        uuid.UUID
	DocumentID      uuid.UUID
	ExpectedVersion int
	ContentHash     string
	SignerUserID    uuid.UUID
	SignerCompanyID uuid.UUID
}

// DocumentObservation is a server-owned snapshot of the document row.
// A future caller must load it from trusted storage. A client request body
// is not this snapshot. This package does not query PostgreSQL, and a check
// here does not replace a later check under the document row lock.
type DocumentObservation struct {
	TenantID    uuid.UUID
	DocumentID  uuid.UUID
	Version     int
	ContentHash string
}

// LocalSignatureRecord is the one local bookkeeping marker for an attempt.
// It is not DocumentStatusSigned, not a qualified signature, and not a
// verification result.
type LocalSignatureRecord struct {
	Recorded bool
	Attempt  int
}

// SigningOperationEvidenceClaim is the classification supplied with a failure
// decision. Kind by itself is not authenticated provider proof.
type SigningOperationEvidenceClaim struct {
	Kind      string
	Source    string
	Reference string
}

// SigningOperation is an in-memory operation. This package does not persist it,
// does not insert a signature, and does not call a provider.
type SigningOperation struct {
	Identity             SigningOperationIdentity
	State                string
	EvidenceKind         string
	EvidenceSource       string
	EvidenceReference    string
	LocalSignatureRecord LocalSignatureRecord
}

type signingOperationTransition struct {
	From     string
	To       string
	Evidence string
}

// Allowed transitions. A missing external response is evidence only for
// RESULT_UNKNOWN. It is not a row that targets FAILED_RETRYABLE or FAILED_FINAL.
var signingOperationTransitions = []signingOperationTransition{
	{From: SigningOperationStatePrepared, To: SigningOperationStateResultUnknown, Evidence: SigningOperationEvidenceExternalResponseAbsent},
	{From: SigningOperationStateResultUnknown, To: SigningOperationStateManualReviewRequired, Evidence: SigningOperationEvidenceContradiction},
	{From: SigningOperationStateResultUnknown, To: SigningOperationStateFailedRetryable, Evidence: SigningOperationEvidenceProviderRejectionConfirmed},
	{From: SigningOperationStateResultUnknown, To: SigningOperationStateFailedFinal, Evidence: SigningOperationEvidenceProviderRejectionConfirmed},
}

func ValidateSigningOperationIdentity(in SigningOperationIdentity) error {
	if in.OperationID == uuid.Nil {
		return apperrors.Validation("operation_id is required", map[string]any{"field": "operation_id"})
	}
	if in.TenantID == uuid.Nil {
		return apperrors.Validation("tenant_id is required", map[string]any{"field": "tenant_id"})
	}
	if in.DocumentID == uuid.Nil {
		return apperrors.Validation("document_id is required", map[string]any{"field": "document_id"})
	}
	if in.ExpectedVersion < 1 {
		return apperrors.Validation("expected_version is required", map[string]any{"field": "expected_version"})
	}
	if err := validateSigningOperationContentHash(in.ContentHash); err != nil {
		return err
	}
	if in.SignerUserID == uuid.Nil {
		return apperrors.Validation("signer_user_id is required", map[string]any{"field": "signer_user_id"})
	}
	if in.SignerCompanyID == uuid.Nil {
		return apperrors.Validation("signer_company_id is required", map[string]any{"field": "signer_company_id"})
	}
	return nil
}

func validateSigningOperationContentHash(value string) error {
	if len(value) != 64 {
		return apperrors.Validation("content_hash must be a sha-256 hex digest", map[string]any{"field": "content_hash"})
	}
	for _, c := range value {
		if (c < '0' || c > '9') && (c < 'a' || c > 'f') {
			return apperrors.Validation("content_hash must be a sha-256 hex digest", map[string]any{"field": "content_hash"})
		}
	}
	return nil
}

func validateDocumentObservation(obs DocumentObservation) error {
	if obs.TenantID == uuid.Nil {
		return apperrors.Validation("tenant_id is required", map[string]any{"field": "tenant_id"})
	}
	if obs.DocumentID == uuid.Nil {
		return apperrors.Validation("document_id is required", map[string]any{"field": "document_id"})
	}
	if obs.Version < 1 {
		return apperrors.Validation("expected_version is required", map[string]any{"field": "expected_version"})
	}
	return validateSigningOperationContentHash(obs.ContentHash)
}

// ReadSigningOperation returns the stored operation for the caller tenant.
// A foreign tenant and a missing operation produce the same not-found error.
// The function does not write.
func ReadSigningOperation(callerTenant, operationID uuid.UUID, stored *SigningOperation) (SigningOperation, error) {
	if callerTenant == uuid.Nil {
		return SigningOperation{}, apperrors.Validation("tenant_id is required", map[string]any{"field": "tenant_id"})
	}
	if operationID == uuid.Nil {
		return SigningOperation{}, apperrors.Validation("operation_id is required", map[string]any{"field": "operation_id"})
	}
	if stored == nil || stored.Identity.OperationID != operationID || stored.Identity.TenantID != callerTenant {
		return SigningOperation{}, apperrors.NotFound(signingOperationNotFoundMessage)
	}
	return stored.clone(), nil
}

// PlanPrepare resolves one operation for the caller's tenant.
// A stored operation with the same identity is returned with no write, including
// when the live document version or content hash has changed. That replay does
// not create a signature, call a provider, or update the stored operation.
// Observation is checked only when this tenant has no stored operation.
func PlanPrepare(obs DocumentObservation, in SigningOperationIdentity, existing []SigningOperation) (SigningOperation, bool, error) {
	if err := ValidateSigningOperationIdentity(in); err != nil {
		return SigningOperation{}, false, err
	}
	found, ok, err := lookupTenantScopedOperation(existing, in)
	if err != nil {
		return SigningOperation{}, false, err
	}
	if ok {
		if err := matchSigningOperationIdentity(found.Identity, in); err != nil {
			return SigningOperation{}, false, err
		}
		return found.clone(), false, nil
	}
	if err := validateDocumentObservation(obs); err != nil {
		return SigningOperation{}, false, err
	}
	if err := observationMatches(obs, in); err != nil {
		return SigningOperation{}, false, err
	}
	for i := range existing {
		held := existing[i]
		if held.Identity.TenantID == in.TenantID && held.Identity.DocumentID == in.DocumentID && SigningOperationStateBlocksDocument(held.State) {
			return SigningOperation{}, false, apperrors.Conflict("active signing operation blocks a new operation", map[string]any{
				"field": "document_id",
			})
		}
	}
	return SigningOperation{Identity: in, State: SigningOperationStatePrepared}, true, nil
}

// PlanLocalSignatureRecord stores one local attempt marker on PREPARED.
// obs must already be the trusted server-owned document snapshot. A mismatch
// of tenant, document, version, or content hash returns the unchanged operation
// and asks for no write. A marker that is already recorded is read-only when
// that snapshot still matches the operation identity. This is not a signature
// insert, not verification_status VALID, and not document status SIGNED.
func PlanLocalSignatureRecord(stored SigningOperation, presented SigningOperationIdentity, obs DocumentObservation) (SigningOperation, bool, error) {
	if err := matchSigningOperationIdentity(stored.Identity, presented); err != nil {
		return SigningOperation{}, false, err
	}
	if stored.State != SigningOperationStatePrepared {
		return stored.clone(), false, apperrors.Conflict("local signature record is only stored on PREPARED", map[string]any{
			"field": "state",
			"state": stored.State,
		})
	}
	if err := observationMatches(obs, stored.Identity); err != nil {
		return stored.clone(), false, err
	}
	if stored.LocalSignatureRecord.Recorded {
		return stored.clone(), false, nil
	}
	next := stored.clone()
	next.LocalSignatureRecord = LocalSignatureRecord{Recorded: true, Attempt: 1}
	next.EvidenceKind = SigningOperationEvidenceLocalAttempt
	next.EvidenceSource = SigningOperationEvidenceSourceServer
	return next, true, nil
}

// PlanExternalResultUnknown moves PREPARED to RESULT_UNKNOWN because the
// external outcome was not stored. Absence of that response is not a provider
// rejection. Replay writes nothing. This function does not call a provider.
func PlanExternalResultUnknown(stored SigningOperation, presented SigningOperationIdentity) (SigningOperation, bool, error) {
	if err := matchSigningOperationIdentity(stored.Identity, presented); err != nil {
		return SigningOperation{}, false, err
	}
	next, write, err := applySigningOperationTransition(stored, SigningOperationStateResultUnknown, SigningOperationEvidenceExternalResponseAbsent)
	if err != nil || !write {
		return next, write, err
	}
	next.EvidenceSource = SigningOperationEvidenceSourceServer
	next.EvidenceReference = ""
	return next, true, nil
}

// PlanManualReview moves RESULT_UNKNOWN to MANUAL_REVIEW_REQUIRED only for
// contradictory evidence. A missing external response stays RESULT_UNKNOWN.
func PlanManualReview(stored SigningOperation, presented SigningOperationIdentity, evidence string) (SigningOperation, bool, error) {
	if err := matchSigningOperationIdentity(stored.Identity, presented); err != nil {
		return SigningOperation{}, false, err
	}
	return applySigningOperationTransition(stored, SigningOperationStateManualReviewRequired, evidence)
}

// PlanFailure moves RESULT_UNKNOWN to FAILED_RETRYABLE or FAILED_FINAL only
// for an authenticated provider rejection. The evidence kind string is not
// that proof. EXTERNAL_RESPONSE_ABSENT does not cancel PREPARED and does not
// select either failure state. FAILED_RETRYABLE and FAILED_FINAL are not
// interchangeable: a row already in one of them does not move to the other.
func PlanFailure(stored SigningOperation, presented SigningOperationIdentity, target string, claim SigningOperationEvidenceClaim) (SigningOperation, bool, error) {
	if target != SigningOperationStateFailedRetryable && target != SigningOperationStateFailedFinal {
		return SigningOperation{}, false, apperrors.Validation("failure target is not a signing operation failure state", map[string]any{
			"field": "state",
			"state": target,
		})
	}
	if err := matchSigningOperationIdentity(stored.Identity, presented); err != nil {
		return SigningOperation{}, false, err
	}
	if stored.State == target && failureClaimMatches(stored, claim) {
		return stored.clone(), false, nil
	}
	if claim.Kind == "" || claim.Kind == SigningOperationEvidenceExternalResponseAbsent {
		return stored.clone(), false, apperrors.Conflict("external response absence is not a provider rejection", map[string]any{
			"field": "evidence",
		})
	}
	if !authenticatedProviderRejection(claim) {
		return stored.clone(), false, apperrors.Conflict("provider rejection evidence is not authenticated", map[string]any{
			"field": "evidence",
		})
	}
	if !signingOperationTransitionAllowed(stored.State, target, claim.Kind) {
		return stored.clone(), false, apperrors.Conflict("signing operation transition is not allowed", map[string]any{
			"field": "state",
			"from":  stored.State,
			"to":    target,
		})
	}
	next := stored.clone()
	next.State = target
	next.EvidenceKind = claim.Kind
	next.EvidenceSource = claim.Source
	next.EvidenceReference = strings.TrimSpace(claim.Reference)
	return next, true, nil
}

func authenticatedProviderRejection(claim SigningOperationEvidenceClaim) bool {
	return claim.Kind == SigningOperationEvidenceProviderRejectionConfirmed &&
		claim.Source == SigningOperationEvidenceSourceAuthenticatedProvider &&
		strings.TrimSpace(claim.Reference) != ""
}

func failureClaimMatches(stored SigningOperation, claim SigningOperationEvidenceClaim) bool {
	return authenticatedProviderRejection(claim) &&
		stored.EvidenceKind == claim.Kind &&
		stored.EvidenceSource == claim.Source &&
		stored.EvidenceReference == strings.TrimSpace(claim.Reference)
}

func applySigningOperationTransition(stored SigningOperation, to, evidence string) (SigningOperation, bool, error) {
	if stored.State == to && stored.EvidenceKind == evidence {
		return stored.clone(), false, nil
	}
	if !signingOperationTransitionAllowed(stored.State, to, evidence) {
		if signingOperationFailureState(to) && (evidence == "" || evidence == SigningOperationEvidenceExternalResponseAbsent) {
			return stored.clone(), false, apperrors.Conflict("external response absence is not a provider rejection", map[string]any{
				"field": "evidence",
			})
		}
		return stored.clone(), false, apperrors.Conflict("signing operation transition is not allowed", map[string]any{
			"field": "state",
			"from":  stored.State,
			"to":    to,
		})
	}
	next := stored.clone()
	next.State = to
	next.EvidenceKind = evidence
	return next, true, nil
}

func signingOperationTransitionAllowed(from, to, evidence string) bool {
	for _, rule := range signingOperationTransitions {
		if rule.From == from && rule.To == to && rule.Evidence == evidence {
			return true
		}
	}
	return false
}

func signingOperationFailureState(state string) bool {
	return state == SigningOperationStateFailedRetryable || state == SigningOperationStateFailedFinal
}

func observationMatches(obs DocumentObservation, in SigningOperationIdentity) error {
	field := ""
	switch {
	case obs.TenantID != in.TenantID:
		field = "tenant_id"
	case obs.DocumentID != in.DocumentID:
		field = "document_id"
	case obs.Version != in.ExpectedVersion:
		field = "expected_version"
	case obs.ContentHash != in.ContentHash:
		field = "content_hash"
	default:
		return nil
	}
	return apperrors.Conflict("document observation does not match the signing operation", map[string]any{"field": field})
}

func lookupTenantScopedOperation(existing []SigningOperation, in SigningOperationIdentity) (SigningOperation, bool, error) {
	var found *SigningOperation
	sawOtherTenant := false
	for i := range existing {
		if existing[i].Identity.OperationID != in.OperationID {
			continue
		}
		if existing[i].Identity.TenantID != in.TenantID {
			sawOtherTenant = true
			continue
		}
		copy := existing[i]
		found = &copy
	}
	if found != nil {
		return *found, true, nil
	}
	if sawOtherTenant {
		return SigningOperation{}, false, apperrors.NotFound(signingOperationNotFoundMessage)
	}
	return SigningOperation{}, false, nil
}

func matchSigningOperationIdentity(stored, presented SigningOperationIdentity) error {
	if presented.OperationID == uuid.Nil {
		return apperrors.Validation("operation_id is required", map[string]any{"field": "operation_id"})
	}
	if stored.OperationID != presented.OperationID || stored.TenantID != presented.TenantID {
		return apperrors.NotFound(signingOperationNotFoundMessage)
	}
	field := ""
	switch {
	case stored.DocumentID != presented.DocumentID:
		field = "document_id"
	case stored.ExpectedVersion != presented.ExpectedVersion:
		field = "expected_version"
	case stored.ContentHash != presented.ContentHash:
		field = "content_hash"
	case stored.SignerUserID != presented.SignerUserID:
		field = "signer_user_id"
	case stored.SignerCompanyID != presented.SignerCompanyID:
		field = "signer_company_id"
	default:
		return nil
	}
	return apperrors.Conflict("signing operation identity does not match", map[string]any{"field": field})
}

func (op SigningOperation) clone() SigningOperation {
	return op
}
