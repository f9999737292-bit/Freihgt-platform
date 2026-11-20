package domain

import (
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

	signingOperationNotFoundMessage = "signing operation not found"
)

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

// DocumentObservation is the document fact a caller presents before a write.
// It is not loaded from PostgreSQL by this package.
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

// SigningOperation is an in-memory operation. This package does not persist it
// and does not call a provider.
type SigningOperation struct {
	Identity             SigningOperationIdentity
	State                string
	EvidenceKind         string
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

// PlanPrepare inserts one PREPARED operation when the observation matches the
// identity and the tenant-document lock is free.
// Replay of the same operation id and the same identity returns the stored
// operation and asks for no write. A different identity for that id does not
// return the stored operation.
func PlanPrepare(obs DocumentObservation, in SigningOperationIdentity, existing []SigningOperation) (SigningOperation, bool, error) {
	if err := ValidateSigningOperationIdentity(in); err != nil {
		return SigningOperation{}, false, err
	}
	if err := validateDocumentObservation(obs); err != nil {
		return SigningOperation{}, false, err
	}
	if err := observationMatches(obs, in); err != nil {
		return SigningOperation{}, false, err
	}
	for i := range existing {
		if existing[i].Identity.OperationID != in.OperationID {
			continue
		}
		if err := matchSigningOperationIdentity(existing[i].Identity, in); err != nil {
			return SigningOperation{}, false, err
		}
		return existing[i].clone(), false, nil
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

// PlanLocalSignatureRecord stores the single local attempt marker on PREPARED.
// A recorded marker is replayed with no write. This is not a provider call and
// it does not set a document status.
func PlanLocalSignatureRecord(stored SigningOperation, presented SigningOperationIdentity) (SigningOperation, bool, error) {
	if err := matchSigningOperationIdentity(stored.Identity, presented); err != nil {
		return SigningOperation{}, false, err
	}
	if stored.State != SigningOperationStatePrepared {
		return stored.clone(), false, apperrors.Conflict("local signature record is only stored on PREPARED", map[string]any{
			"field": "state",
			"state": stored.State,
		})
	}
	if stored.LocalSignatureRecord.Recorded {
		return stored.clone(), false, nil
	}
	next := stored.clone()
	next.LocalSignatureRecord = LocalSignatureRecord{Recorded: true, Attempt: 1}
	next.EvidenceKind = SigningOperationEvidenceLocalAttempt
	return next, true, nil
}

// PlanExternalResultUnknown moves PREPARED to RESULT_UNKNOWN because the
// external outcome was not stored. Absence of that response is not a provider
// rejection. Replay writes nothing. This function does not call a provider.
func PlanExternalResultUnknown(stored SigningOperation, presented SigningOperationIdentity) (SigningOperation, bool, error) {
	if err := matchSigningOperationIdentity(stored.Identity, presented); err != nil {
		return SigningOperation{}, false, err
	}
	return applySigningOperationTransition(stored, SigningOperationStateResultUnknown, SigningOperationEvidenceExternalResponseAbsent)
}

// PlanManualReview moves RESULT_UNKNOWN to MANUAL_REVIEW_REQUIRED only for
// contradictory evidence. A missing external response stays RESULT_UNKNOWN.
func PlanManualReview(stored SigningOperation, presented SigningOperationIdentity, evidence string) (SigningOperation, bool, error) {
	if err := matchSigningOperationIdentity(stored.Identity, presented); err != nil {
		return SigningOperation{}, false, err
	}
	return applySigningOperationTransition(stored, SigningOperationStateManualReviewRequired, evidence)
}

// PlanFailure moves RESULT_UNKNOWN to a failure state only when the evidence
// is a confirmed provider rejection. EXTERNAL_RESPONSE_ABSENT and an empty
// evidence value leave the state unchanged and ask for no write.
func PlanFailure(stored SigningOperation, presented SigningOperationIdentity, target, evidence string) (SigningOperation, bool, error) {
	if target != SigningOperationStateFailedRetryable && target != SigningOperationStateFailedFinal {
		return SigningOperation{}, false, apperrors.Validation("failure target is not a signing operation failure state", map[string]any{
			"field": "state",
			"state": target,
		})
	}
	if err := matchSigningOperationIdentity(stored.Identity, presented); err != nil {
		return SigningOperation{}, false, err
	}
	return applySigningOperationTransition(stored, target, evidence)
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
