package domain

import (
	"errors"
	"strings"
	"testing"

	"github.com/google/uuid"

	apperrors "github.com/freight-platform/document-service/internal/platform/errors"
)

// fakeSigningOperationStore is an in-memory stand-in. It has no database and
// no network client. A refused plan must be committed with write=false so the
// write counter stays unchanged.
type fakeSigningOperationStore struct {
	rows   map[uuid.UUID]SigningOperation
	writes int
}

func (s *fakeSigningOperationStore) commit(op SigningOperation, write bool, crashBeforeWrite bool) {
	if !write || crashBeforeWrite {
		return
	}
	if s.rows == nil {
		s.rows = map[uuid.UUID]SigningOperation{}
	}
	s.rows[op.Identity.OperationID] = op
	s.writes++
}

func (s *fakeSigningOperationStore) get(id uuid.UUID) (SigningOperation, bool) {
	op, ok := s.rows[id]
	return op, ok
}

func signingOperationCode(err error) apperrors.Code {
	var app *apperrors.AppError
	if !errors.As(err, &app) || app == nil {
		return ""
	}
	return app.Code
}

func testSigningOperationIdentity() (SigningOperationIdentity, DocumentObservation) {
	in := SigningOperationIdentity{
		OperationID:     uuid.New(),
		TenantID:        uuid.New(),
		DocumentID:      uuid.New(),
		ExpectedVersion: 3,
		ContentHash:     strings.Repeat("ab", 32),
		SignerUserID:    uuid.New(),
		SignerCompanyID: uuid.New(),
	}
	obs := DocumentObservation{
		TenantID:    in.TenantID,
		DocumentID:  in.DocumentID,
		Version:     in.ExpectedVersion,
		ContentHash: in.ContentHash,
	}
	return in, obs
}

func requireNoWrite(t *testing.T, store *fakeSigningOperationStore, write bool) {
	t.Helper()
	if write {
		t.Fatal("expected write=false")
	}
	store.commit(SigningOperation{}, write, false)
	if store.writes != 0 {
		t.Fatalf("STORE_WRITES=%d, want 0", store.writes)
	}
}

func TestSigningOperationF01CrashBeforePersist(t *testing.T) {
	in, obs := testSigningOperationIdentity()
	store := &fakeSigningOperationStore{}

	op, write, err := PlanPrepare(obs, in, nil)
	if err != nil {
		t.Fatal(err)
	}
	if !write || op.State != SigningOperationStatePrepared {
		t.Fatalf("plan = %+v write=%v", op, write)
	}
	store.commit(op, write, true)
	if store.writes != 0 {
		t.Fatalf("STORE_WRITES=%d, want 0", store.writes)
	}
	if _, ok := store.get(in.OperationID); ok {
		t.Fatal("operation was stored after a crash before persist")
	}

	in.ContentHash = ""
	_, write, err = PlanPrepare(obs, in, nil)
	if signingOperationCode(err) != apperrors.CodeValidation {
		t.Fatalf("code=%s err=%v", signingOperationCode(err), err)
	}
	requireNoWrite(t, store, write)
}

func TestSigningOperationF02LocalAttemptRetry(t *testing.T) {
	in, obs := testSigningOperationIdentity()
	store := &fakeSigningOperationStore{}
	prepared, write, err := PlanPrepare(obs, in, nil)
	if err != nil || !write {
		t.Fatalf("prepare err=%v write=%v", err, write)
	}
	store.commit(prepared, write, false)
	if store.writes != 1 {
		t.Fatalf("STORE_WRITES=%d, want 1", store.writes)
	}

	stored, _ := store.get(in.OperationID)
	recorded, write, err := PlanLocalSignatureRecord(stored, in)
	if err != nil {
		t.Fatal(err)
	}
	if !write || !recorded.LocalSignatureRecord.Recorded || recorded.LocalSignatureRecord.Attempt != 1 {
		t.Fatalf("record = %+v write=%v", recorded.LocalSignatureRecord, write)
	}
	if recorded.State != SigningOperationStatePrepared {
		t.Fatalf("state=%s", recorded.State)
	}
	store.commit(recorded, write, false)

	replayed, write, err := PlanLocalSignatureRecord(recorded, in)
	if err != nil {
		t.Fatal(err)
	}
	if write {
		t.Fatal("replay requested a write")
	}
	store.commit(replayed, write, false)
	if store.writes != 2 {
		t.Fatalf("STORE_WRITES=%d, want 2", store.writes)
	}
	if replayed.LocalSignatureRecord.Attempt != 1 || replayed.State != SigningOperationStatePrepared {
		t.Fatalf("replay = %+v state=%s", replayed.LocalSignatureRecord, replayed.State)
	}
}

func TestSigningOperationF06DuplicateOperationID(t *testing.T) {
	in, obs := testSigningOperationIdentity()
	store := &fakeSigningOperationStore{}
	prepared, write, err := PlanPrepare(obs, in, nil)
	if err != nil || !write {
		t.Fatal(err)
	}
	store.commit(prepared, write, false)
	unknown, write, err := PlanExternalResultUnknown(prepared, in)
	if err != nil || !write || unknown.State != SigningOperationStateResultUnknown {
		t.Fatalf("unknown=%s write=%v err=%v", unknown.State, write, err)
	}
	store.commit(unknown, write, false)

	replayed, write, err := PlanPrepare(obs, in, []SigningOperation{unknown})
	if err != nil {
		t.Fatal(err)
	}
	if write || replayed.State != SigningOperationStateResultUnknown {
		t.Fatalf("replay state=%s write=%v", replayed.State, write)
	}
	before := store.writes
	store.commit(replayed, write, false)
	if store.writes != before {
		t.Fatalf("STORE_WRITES=%d, want %d", store.writes, before)
	}

	cases := []struct {
		name string
		edit func(*SigningOperationIdentity)
		code apperrors.Code
	}{
		{name: "tenant", edit: func(id *SigningOperationIdentity) { id.TenantID = uuid.New() }, code: apperrors.CodeNotFound},
		{name: "document", edit: func(id *SigningOperationIdentity) { id.DocumentID = uuid.New() }, code: apperrors.CodeConflict},
		{name: "version", edit: func(id *SigningOperationIdentity) { id.ExpectedVersion = 9 }, code: apperrors.CodeConflict},
		{name: "hash", edit: func(id *SigningOperationIdentity) { id.ContentHash = strings.Repeat("cd", 32) }, code: apperrors.CodeConflict},
		{name: "signer user", edit: func(id *SigningOperationIdentity) { id.SignerUserID = uuid.New() }, code: apperrors.CodeConflict},
		{name: "signer company", edit: func(id *SigningOperationIdentity) { id.SignerCompanyID = uuid.New() }, code: apperrors.CodeConflict},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			presented := in
			tc.edit(&presented)
			got, write, err := PlanLocalSignatureRecord(unknown, presented)
			if signingOperationCode(err) != tc.code {
				t.Fatalf("code=%s err=%v", signingOperationCode(err), err)
			}
			if write || got.Identity.OperationID != uuid.Nil || got.State != "" {
				t.Fatalf("reused stored result: %+v write=%v", got, write)
			}
			store.commit(got, write, false)
			if store.writes != before {
				t.Fatalf("STORE_WRITES=%d, want %d", store.writes, before)
			}
		})
	}
}

func TestSigningOperationF07SecondActiveOperation(t *testing.T) {
	in, obs := testSigningOperationIdentity()
	active := SigningOperation{Identity: in, State: SigningOperationStatePrepared}
	other := in
	other.OperationID = uuid.New()
	other.SignerUserID = uuid.New()
	otherObs := obs
	store := &fakeSigningOperationStore{}

	_, write, err := PlanPrepare(otherObs, other, []SigningOperation{active})
	if signingOperationCode(err) != apperrors.CodeConflict {
		t.Fatalf("code=%s err=%v", signingOperationCode(err), err)
	}
	requireNoWrite(t, store, write)

	// FAILED_RETRYABLE releases the in-memory tenant-document scope.
	// This fake store does not prove PostgreSQL concurrency safety.
	released := active
	released.State = SigningOperationStateFailedRetryable
	created, write, err := PlanPrepare(otherObs, other, []SigningOperation{released})
	if err != nil || !write || created.State != SigningOperationStatePrepared {
		t.Fatalf("created=%s write=%v err=%v", created.State, write, err)
	}
}

func TestSigningOperationF08VersionOrHashMismatch(t *testing.T) {
	in, obs := testSigningOperationIdentity()
	store := &fakeSigningOperationStore{}

	badVersion := obs
	badVersion.Version = 4
	_, write, err := PlanPrepare(badVersion, in, nil)
	if signingOperationCode(err) != apperrors.CodeConflict {
		t.Fatalf("version code=%s err=%v", signingOperationCode(err), err)
	}
	requireNoWrite(t, store, write)

	badHash := obs
	badHash.ContentHash = strings.Repeat("cd", 32)
	_, write, err = PlanPrepare(badHash, in, nil)
	if signingOperationCode(err) != apperrors.CodeConflict {
		t.Fatalf("hash code=%s err=%v", signingOperationCode(err), err)
	}
	requireNoWrite(t, store, write)

	prepared := SigningOperation{Identity: in, State: SigningOperationStatePrepared}
	presented := in
	presented.ExpectedVersion = 8
	got, write, err := PlanLocalSignatureRecord(prepared, presented)
	if signingOperationCode(err) != apperrors.CodeConflict || write || got.Identity.OperationID != uuid.Nil {
		t.Fatalf("got=%+v write=%v err=%v", got, write, err)
	}
	store.commit(got, write, false)
	if store.writes != 0 {
		t.Fatalf("STORE_WRITES=%d, want 0", store.writes)
	}
}

func TestSigningOperationF09CrossTenantNotFound(t *testing.T) {
	in, _ := testSigningOperationIdentity()
	stored := SigningOperation{Identity: in, State: SigningOperationStatePrepared, EvidenceKind: SigningOperationEvidenceLocalAttempt}
	store := &fakeSigningOperationStore{}

	missing, missingErr := ReadSigningOperation(in.TenantID, uuid.New(), nil)
	foreign, foreignErr := ReadSigningOperation(uuid.New(), in.OperationID, &stored)
	if signingOperationCode(missingErr) != apperrors.CodeNotFound || signingOperationCode(foreignErr) != apperrors.CodeNotFound {
		t.Fatalf("missing=%v foreign=%v", missingErr, foreignErr)
	}
	if missingErr.Error() != foreignErr.Error() {
		t.Fatalf("missing %q foreign %q", missingErr.Error(), foreignErr.Error())
	}
	if missing.Identity.OperationID != uuid.Nil || foreign.Identity.OperationID != uuid.Nil || foreign.State != "" {
		t.Fatalf("missing=%+v foreign=%+v", missing, foreign)
	}
	store.commit(foreign, false, false)
	if store.writes != 0 {
		t.Fatalf("STORE_WRITES=%d, want 0", store.writes)
	}
}

func TestSigningOperationUnknownExternalResult(t *testing.T) {
	in, obs := testSigningOperationIdentity()
	prepared, write, err := PlanPrepare(obs, in, nil)
	if err != nil || !write {
		t.Fatal(err)
	}
	unknown, write, err := PlanExternalResultUnknown(prepared, in)
	if err != nil || !write {
		t.Fatalf("err=%v write=%v", err, write)
	}
	if unknown.State != SigningOperationStateResultUnknown || unknown.EvidenceKind != SigningOperationEvidenceExternalResponseAbsent {
		t.Fatalf("state=%s evidence=%s", unknown.State, unknown.EvidenceKind)
	}

	store := &fakeSigningOperationStore{}
	store.commit(unknown, true, false)
	replayed, write, err := PlanExternalResultUnknown(unknown, in)
	if err != nil || write || replayed.State != SigningOperationStateResultUnknown {
		t.Fatalf("replay state=%s write=%v err=%v", replayed.State, write, err)
	}
	store.commit(replayed, write, false)

	for _, target := range []string{SigningOperationStateFailedRetryable, SigningOperationStateFailedFinal} {
		kept, write, err := PlanFailure(unknown, in, target, SigningOperationEvidenceExternalResponseAbsent)
		if signingOperationCode(err) != apperrors.CodeConflict || write || kept.State != SigningOperationStateResultUnknown {
			t.Fatalf("target=%s kept=%s write=%v err=%v", target, kept.State, write, err)
		}
		store.commit(kept, write, false)
		kept, write, err = PlanFailure(unknown, in, target, "")
		if signingOperationCode(err) != apperrors.CodeConflict || write || kept.State != SigningOperationStateResultUnknown {
			t.Fatalf("empty evidence target=%s kept=%s write=%v err=%v", target, kept.State, write, err)
		}
		store.commit(kept, write, false)
	}
	if store.writes != 1 {
		t.Fatalf("STORE_WRITES=%d, want 1", store.writes)
	}

	confirmed, write, err := PlanFailure(unknown, in, SigningOperationStateFailedRetryable, SigningOperationEvidenceProviderRejectionConfirmed)
	if err != nil || !write || confirmed.State != SigningOperationStateFailedRetryable {
		t.Fatalf("confirmed=%s write=%v err=%v", confirmed.State, write, err)
	}
	again, write, err := PlanFailure(confirmed, in, SigningOperationStateFailedRetryable, SigningOperationEvidenceProviderRejectionConfirmed)
	if err != nil || write || again.State != SigningOperationStateFailedRetryable {
		t.Fatalf("failure replay state=%s write=%v err=%v", again.State, write, err)
	}
}

func TestSigningOperationManualReviewRequiresContradiction(t *testing.T) {
	in, _ := testSigningOperationIdentity()
	unknown := SigningOperation{
		Identity:     in,
		State:        SigningOperationStateResultUnknown,
		EvidenceKind: SigningOperationEvidenceExternalResponseAbsent,
	}
	kept, write, err := PlanManualReview(unknown, in, SigningOperationEvidenceExternalResponseAbsent)
	if signingOperationCode(err) != apperrors.CodeConflict || write || kept.State != SigningOperationStateResultUnknown {
		t.Fatalf("kept=%s write=%v err=%v", kept.State, write, err)
	}
	review, write, err := PlanManualReview(unknown, in, SigningOperationEvidenceContradiction)
	if err != nil || !write || review.State != SigningOperationStateManualReviewRequired {
		t.Fatalf("review=%s write=%v err=%v", review.State, write, err)
	}
	replayed, write, err := PlanManualReview(review, in, SigningOperationEvidenceContradiction)
	if err != nil || write || replayed.State != SigningOperationStateManualReviewRequired {
		t.Fatalf("replay=%s write=%v err=%v", replayed.State, write, err)
	}
	if !SigningOperationStateBlocksDocument(review.State) {
		t.Fatal("manual review must hold the document scope")
	}
}
