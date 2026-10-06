package service

import (
	"bytes"
	"context"
	"errors"
	"io"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/freight-platform/document-service/internal/domain"
	apperrors "github.com/freight-platform/document-service/internal/platform/errors"
	"github.com/freight-platform/document-service/internal/platform/storage"
	"github.com/freight-platform/document-service/internal/repository"
)

type countingStore struct {
	inner   storage.ObjectStore
	exists  int
	puts    int
	deletes int
	lastKey string
}

func (s *countingStore) Put(ctx context.Context, key string, r io.Reader, max int64) (int64, error) {
	s.puts++
	s.lastKey = key
	return s.inner.Put(ctx, key, r, max)
}
func (s *countingStore) Get(ctx context.Context, key string) (io.ReadCloser, error) {
	return s.inner.Get(ctx, key)
}
func (s *countingStore) Exists(context.Context, string) (bool, error) {
	s.exists++
	return false, nil
}
func (s *countingStore) Metadata(ctx context.Context, key string) (storage.ObjectMetadata, error) {
	return s.inner.Metadata(ctx, key)
}
func (s *countingStore) Delete(ctx context.Context, key string) error {
	s.deletes++
	return s.inner.Delete(ctx, key)
}

type detachedRepo struct {
	attachment           *domain.Attachment
	sig                  *domain.AttachmentSignature
	blobSum              string
	status               string
	reason               string
	inserts              int
	failInsert           bool
	commitUnknown        bool
	persistBeforeUnknown bool
	lastPolicy           string
	audits               []string
}

func (r *detachedRepo) FindByIdempotency(context.Context, uuid.UUID, string) (*domain.Attachment, error) {
	return nil, nil
}
func (r *detachedRepo) Insert(context.Context, domain.Attachment) error { return nil }
func (r *detachedRepo) Get(context.Context, uuid.UUID, uuid.UUID, uuid.UUID) (*domain.Attachment, error) {
	return r.attachment, nil
}
func (r *detachedRepo) Finalize(context.Context, uuid.UUID, uuid.UUID, uuid.UUID, time.Time) (*domain.Attachment, error) {
	return r.attachment, nil
}
func (r *detachedRepo) FindSignatureByIdempotency(context.Context, uuid.UUID, uuid.UUID, string) (*domain.AttachmentSignature, error) {
	return r.sig, nil
}
func (r *detachedRepo) InsertSignature(_ context.Context, item domain.AttachmentSignature) error {
	copied := item
	r.sig = &copied
	return nil
}
func (r *detachedRepo) DeleteDraft(context.Context, uuid.UUID, uuid.UUID, uuid.UUID) error {
	return nil
}
func (r *detachedRepo) GetSignature(context.Context, uuid.UUID, uuid.UUID, uuid.UUID) (*domain.AttachmentSignature, error) {
	if r.sig == nil {
		return nil, apperrors.NotFound("signature not found")
	}
	return r.sig, nil
}
func (r *detachedRepo) InsertAudit(_ context.Context, _ uuid.UUID, _ uuid.UUID, _ *uuid.UUID, _ *uuid.UUID, _ *uuid.UUID, eventType, _ string) error {
	r.audits = append(r.audits, eventType)
	return nil
}
func (r *detachedRepo) SignatureBlobDigest(context.Context, uuid.UUID, uuid.UUID) (string, error) {
	if r.blobSum == "" {
		return "", apperrors.NotFound("signature blob not found")
	}
	return r.blobSum, nil
}
func (r *detachedRepo) LatestEvidence(_ context.Context, _, _ uuid.UUID, policyID, policyVersion string) (string, string, error) {
	r.lastPolicy = policyID + "/" + policyVersion
	if policyID != domain.PolicyQualifiedCAdES || policyVersion != domain.PolicyVersionV1 {
		return "", "", nil
	}
	return r.status, r.reason, nil
}
func (r *detachedRepo) InsertDetachedSignature(_ context.Context, sig domain.AttachmentSignature, objectKey string, size int64, sum, _, _ string, _ time.Time, _, _, _ string, _ uuid.UUID, _ *uuid.UUID) error {
	if r.commitUnknown {
		if r.persistBeforeUnknown {
			r.storeInserted(sig, sum)
		}
		return repository.UnknownCommitOutcome(errors.New("connection reset"))
	}
	if r.failInsert {
		return errors.New("db down")
	}
	if objectKey == "" || size <= 0 {
		return errors.New("missing blob")
	}
	r.storeInserted(sig, sum)
	return nil
}

func (r *detachedRepo) storeInserted(sig domain.AttachmentSignature, sum string) {
	r.inserts++
	copied := sig
	copied.VerificationStatus = domain.VerificationUnverified
	r.sig = &copied
	r.blobSum = sum
	r.status = domain.VerificationPending
	r.reason = domain.ReasonVerifierUnavailable
}

type trustDocs struct {
	stubDocs
	statusWrites int
}

func (d *trustDocs) UpdateDocumentStatus(context.Context, uuid.UUID, uuid.UUID, string, int) (*domain.Document, error) {
	d.statusWrites++
	return nil, errors.New("document status write")
}

type validVerifier struct{}

func (validVerifier) Verify(context.Context, VerificationInput) (VerificationResult, error) {
	return VerificationResult{Status: domain.VerificationValid, ReasonCode: "BAD_SIGNATURE"}, nil
}

func newDetachedFixture(t *testing.T) (*AttachmentService, *countingStore, *detachedRepo, *trustDocs, uuid.UUID, uuid.UUID) {
	t.Helper()
	inner, err := storage.NewLocalObjectStore(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	body := []byte("%PDF-1.7 attachment")
	tenant := uuid.New()
	documentID := uuid.New()
	attachmentID := uuid.New()
	key := "tenants/" + tenant.String() + "/documents/" + documentID.String() + "/attachments/" + attachmentID.String()
	if _, err := inner.Put(context.Background(), key, bytes.NewReader(body), domain.MaxAttachmentBytes); err != nil {
		t.Fatal(err)
	}
	store := &countingStore{inner: inner}
	repo := &detachedRepo{attachment: &domain.Attachment{
		ID: attachmentID, DocumentID: documentID, TenantID: tenant,
		Status: domain.AttachmentStatusFinalized, StorageKey: key,
		SHA256: domain.SHA256Hex(body), SizeBytes: int64(len(body)),
	}}
	docs := &trustDocs{}
	svc := &AttachmentService{docs: docs, store: store, repo: repo, verifier: UnavailableSignatureVerifier{}, maxByte: domain.MaxAttachmentBytes}
	return svc, store, repo, docs, documentID, tenant
}

func TestDetachedSignatureFoundation(t *testing.T) {
	svc, store, repo, docs, documentID, tenant := newDetachedFixture(t)
	attachmentID := repo.attachment.ID
	payload := []byte("detached-cades-bytes")
	got, err := svc.AttachDetachedSignature(context.Background(), tenant, documentID, attachmentID, DetachedSignatureInput{
		Body: bytes.NewReader(payload), IdempotencyKey: "sig-1",
	})
	if err != nil {
		t.Fatal(err)
	}
	if got.EffectiveStatus != domain.VerificationPending || got.ReasonCode != domain.ReasonVerifierUnavailable {
		t.Fatalf("status=%s reason=%s", got.EffectiveStatus, got.ReasonCode)
	}
	if got.VerificationStatus != domain.VerificationUnverified {
		t.Fatalf("persisted status=%s", got.VerificationStatus)
	}
	if store.exists != 0 || store.puts != 1 {
		t.Fatalf("exists=%d puts=%d", store.exists, store.puts)
	}
	if store.lastKey != domain.SignatureObjectKey(tenant, documentID, attachmentID, got.ID) {
		t.Fatalf("object key %s", store.lastKey)
	}
	if got.SignatureReference != domain.OpaqueSignatureReference(got.ID) || bytes.Contains([]byte(got.SignatureReference), []byte(store.lastKey)) {
		t.Fatalf("reference exposed storage key: %s", got.SignatureReference)
	}
	if repo.blobSum != domain.SHA256Hex(payload) || repo.inserts != 1 {
		t.Fatalf("sum=%s inserts=%d", repo.blobSum, repo.inserts)
	}
	for _, event := range repo.audits {
		if event == domain.EventVerificationFailed || event == domain.EventSignatureVerified {
			t.Fatalf("unexpected audit %s", event)
		}
	}
	if repo.lastPolicy != domain.PolicyQualifiedCAdES+"/"+domain.PolicyVersionV1 {
		t.Fatalf("effective policy %s", repo.lastPolicy)
	}
	replay, err := svc.AttachDetachedSignature(context.Background(), tenant, documentID, attachmentID, DetachedSignatureInput{
		Body: bytes.NewReader(payload), IdempotencyKey: "sig-1",
	})
	if err != nil || replay.ID != got.ID || repo.inserts != 1 || store.puts != 1 {
		t.Fatalf("replay id=%v inserts=%d puts=%d err=%v", replay, repo.inserts, store.puts, err)
	}
	_, err = svc.AttachDetachedSignature(context.Background(), tenant, documentID, attachmentID, DetachedSignatureInput{
		Body: bytes.NewReader([]byte("other-bytes")), IdempotencyKey: "sig-1",
	})
	var appErr *apperrors.AppError
	if !errors.As(err, &appErr) || appErr.Code != apperrors.CodeConflict {
		t.Fatalf("conflict err=%v", err)
	}
	if docs.statusWrites != 0 {
		t.Fatal("document status was written")
	}
	session, docStatus := domain.ResolveSigningAfterSignature(1, 1)
	if session != domain.SigningSessionStatusCompleted || docStatus != domain.DocumentStatusSigned {
		t.Fatal("legacy signing-session result changed")
	}
}

func TestDetachedSignatureRejectsOversizedDraftAndValidResult(t *testing.T) {
	svc, store, repo, _, documentID, tenant := newDetachedFixture(t)
	_, err := svc.AttachDetachedSignature(context.Background(), tenant, documentID, repo.attachment.ID, DetachedSignatureInput{
		Body: bytes.NewReader(bytes.Repeat([]byte("a"), domain.MaxSignatureBytes+1)), IdempotencyKey: "oversize",
	})
	if err == nil {
		t.Fatal("oversize signature was accepted")
	}
	repo.attachment.Status = domain.AttachmentStatusDraft
	_, err = svc.AttachDetachedSignature(context.Background(), tenant, documentID, repo.attachment.ID, DetachedSignatureInput{
		Body: bytes.NewReader([]byte("sig")), IdempotencyKey: "draft",
	})
	if err == nil {
		t.Fatal("draft signature was accepted")
	}
	repo.attachment.Status = domain.AttachmentStatusFinalized
	svc.verifier = validVerifier{}
	_, err = svc.AttachDetachedSignature(context.Background(), tenant, documentID, repo.attachment.ID, DetachedSignatureInput{
		Body: bytes.NewReader([]byte("sig")), IdempotencyKey: "valid-result",
	})
	if err == nil || repo.inserts != 0 {
		t.Fatalf("VALID was stored err=%v inserts=%d", err, repo.inserts)
	}
	if store.deletes == 0 {
		t.Fatal("object was not cleaned up after rejected verifier result")
	}
}

func TestDetachedSignatureCleansUpAfterDatabaseFailure(t *testing.T) {
	svc, store, repo, _, documentID, tenant := newDetachedFixture(t)
	repo.failInsert = true
	_, err := svc.AttachDetachedSignature(context.Background(), tenant, documentID, repo.attachment.ID, DetachedSignatureInput{
		Body: bytes.NewReader([]byte("sig")), IdempotencyKey: "db-down",
	})
	if err == nil || store.deletes == 0 || repo.inserts != 0 {
		t.Fatalf("err=%v deletes=%d inserts=%d", err, store.deletes, repo.inserts)
	}
}

func TestUnavailableVerifierDoesNotInventEvidence(t *testing.T) {
	result, err := UnavailableSignatureVerifier{}.Verify(context.Background(), VerificationInput{
		AttachmentSHA256: "abc", Signature: []byte("sig"), Format: "CAdES",
		PolicyID: domain.PolicyQualifiedCAdES, PolicyVersion: domain.PolicyVersionV1,
	})
	if err != nil || result.Status != domain.VerificationPending || result.ReasonCode != domain.ReasonVerifierUnavailable {
		t.Fatalf("%+v %v", result, err)
	}
}

func TestBinaryIdempotencyKeyRequired(t *testing.T) {
	svc, store, _, _, documentID, tenant := newDetachedFixture(t)
	attachmentID := uuid.New()
	for _, key := range []string{"", "   ", strings.Repeat("k", 129)} {
		_, err := svc.AttachDetachedSignature(context.Background(), tenant, documentID, attachmentID, DetachedSignatureInput{
			Body: bytes.NewReader([]byte("sig")), IdempotencyKey: key,
		})
		var appErr *apperrors.AppError
		if !errors.As(err, &appErr) || appErr.Code != apperrors.CodeValidation || store.puts != 0 {
			t.Fatalf("key %q err=%v puts=%d", key, err, store.puts)
		}
	}
}

func TestUnknownCommitDoesNotDeleteObject(t *testing.T) {
	svc, store, repo, _, documentID, tenant := newDetachedFixture(t)
	repo.commitUnknown = true
	repo.persistBeforeUnknown = true
	_, err := svc.AttachDetachedSignature(context.Background(), tenant, documentID, repo.attachment.ID, DetachedSignatureInput{
		Body: bytes.NewReader([]byte("sig")), IdempotencyKey: "ambiguous",
	})
	var appErr *apperrors.AppError
	if !errors.As(err, &appErr) || appErr.Code != apperrors.CodeInternal || store.deletes != 0 || repo.inserts != 1 {
		t.Fatalf("err=%v deletes=%d inserts=%d", err, store.deletes, repo.inserts)
	}
	body, err := store.Get(context.Background(), store.lastKey)
	if err != nil {
		t.Fatal(err)
	}
	_ = body.Close()
	repo.commitUnknown = false
	replay, err := svc.AttachDetachedSignature(context.Background(), tenant, documentID, repo.attachment.ID, DetachedSignatureInput{
		Body: bytes.NewReader([]byte("sig")), IdempotencyKey: "ambiguous",
	})
	if err != nil || replay.ID != repo.sig.ID || store.puts != 1 {
		t.Fatalf("retry id=%v puts=%d err=%v", replay, store.puts, err)
	}
}
