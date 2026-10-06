package service

import (
	"bytes"
	"context"
	"errors"
	"io"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/freight-platform/document-service/internal/domain"
	apperrors "github.com/freight-platform/document-service/internal/platform/errors"
	"github.com/freight-platform/document-service/internal/platform/storage"
	"github.com/freight-platform/document-service/internal/repository"
)

type recordingStore struct {
	existsCalls int
	putCalls    int
	putErr      error
}

func (s *recordingStore) Put(context.Context, string, io.Reader, int64) (int64, error) {
	s.putCalls++
	if s.putErr != nil {
		return 0, s.putErr
	}
	return 1, nil
}

func (s *recordingStore) Get(context.Context, string) (io.ReadCloser, error) {
	return io.NopCloser(bytes.NewReader(nil)), nil
}

func (s *recordingStore) Exists(context.Context, string) (bool, error) {
	s.existsCalls++
	return false, nil
}

func (s *recordingStore) Metadata(context.Context, string) (storage.ObjectMetadata, error) {
	return storage.ObjectMetadata{}, nil
}

func (s *recordingStore) Delete(context.Context, string) error { return nil }

type stubDocs struct{}

func (stubDocs) CompanyExists(context.Context, uuid.UUID, uuid.UUID) (bool, error) {
	return true, nil
}
func (stubDocs) ShipmentExists(context.Context, uuid.UUID, uuid.UUID) (bool, error) {
	return true, nil
}
func (stubDocs) CreateDocument(context.Context, domain.CreateDocumentInput) (*domain.Document, *domain.DocumentVersion, error) {
	return nil, nil, nil
}
func (stubDocs) GetDetail(context.Context, uuid.UUID, uuid.UUID) (*repository.DocumentDetail, error) {
	return nil, nil
}
func (stubDocs) List(context.Context, domain.ListDocumentsFilter) ([]domain.Document, int, error) {
	return nil, 0, nil
}
func (stubDocs) GetByIDAndTenant(_ context.Context, id, tenantID uuid.UUID) (*domain.Document, error) {
	return &domain.Document{ID: id, TenantID: tenantID, DocumentStatus: domain.DocumentStatusDraft}, nil
}
func (stubDocs) HasVersions(context.Context, uuid.UUID) (bool, error) { return false, nil }
func (stubDocs) CreateVersion(context.Context, uuid.UUID, domain.CreateDocumentVersionInput) (*domain.DocumentVersion, error) {
	return nil, nil
}
func (stubDocs) VersionBelongsToDocument(context.Context, uuid.UUID, uuid.UUID) (bool, error) {
	return true, nil
}
func (stubDocs) AddFile(context.Context, uuid.UUID, domain.CreateDocumentFileInput) (*domain.DocumentFile, error) {
	return nil, nil
}
func (stubDocs) UpdateDocumentStatus(context.Context, uuid.UUID, uuid.UUID, string, int) (*domain.Document, error) {
	return nil, nil
}

type stubRepo struct {
	inserts int
	item    *domain.Attachment
}

func (r *stubRepo) FindByIdempotency(context.Context, uuid.UUID, string) (*domain.Attachment, error) {
	return nil, nil
}
func (r *stubRepo) Insert(_ context.Context, item domain.Attachment) error {
	r.inserts++
	copied := item
	r.item = &copied
	return nil
}
func (r *stubRepo) Get(context.Context, uuid.UUID, uuid.UUID, uuid.UUID) (*domain.Attachment, error) {
	return r.item, nil
}
func (r *stubRepo) Finalize(context.Context, uuid.UUID, uuid.UUID, uuid.UUID, time.Time) (*domain.Attachment, error) {
	return nil, nil
}
func (r *stubRepo) FindSignatureByIdempotency(context.Context, uuid.UUID, uuid.UUID, string) (*domain.AttachmentSignature, error) {
	return nil, nil
}
func (r *stubRepo) InsertSignature(context.Context, domain.AttachmentSignature) error { return nil }
func (r *stubRepo) DeleteDraft(context.Context, uuid.UUID, uuid.UUID, uuid.UUID) error {
	return nil
}
func (r *stubRepo) GetSignature(context.Context, uuid.UUID, uuid.UUID, uuid.UUID) (*domain.AttachmentSignature, error) {
	return nil, nil
}
func (r *stubRepo) InsertAudit(context.Context, uuid.UUID, uuid.UUID, *uuid.UUID, *uuid.UUID, *uuid.UUID, string, string) error {
	return nil
}

func TestCreateDoesNotCallExists(t *testing.T) {
	store := &recordingStore{}
	repo := &stubRepo{}
	svc := &AttachmentService{docs: stubDocs{}, store: store, repo: repo, maxByte: domain.MaxAttachmentBytes}
	item, err := svc.Create(context.Background(), uuid.New(), CreateAttachmentInput{
		TenantID:  uuid.New(),
		FileName:  "act.pdf",
		MediaType: "application/pdf",
		Body:      bytes.NewReader([]byte("%PDF-1.7 create-if-absent")),
	})
	if err != nil {
		t.Fatal(err)
	}
	if item == nil || item.StorageKey == "" {
		t.Fatal("attachment was not created")
	}
	if store.existsCalls != 0 {
		t.Fatalf("CREATE_CALLS_EXISTS=%d", store.existsCalls)
	}
	if store.putCalls != 1 || repo.inserts != 1 {
		t.Fatalf("puts=%d inserts=%d", store.putCalls, repo.inserts)
	}
}

func TestCreateMapsPutCollision(t *testing.T) {
	store := &recordingStore{putErr: storage.ErrObjectExists}
	repo := &stubRepo{}
	svc := &AttachmentService{docs: stubDocs{}, store: store, repo: repo, maxByte: domain.MaxAttachmentBytes}
	_, err := svc.Create(context.Background(), uuid.New(), CreateAttachmentInput{
		TenantID:  uuid.New(),
		FileName:  "act.pdf",
		MediaType: "application/pdf",
		Body:      bytes.NewReader([]byte("%PDF-1.7 collision")),
	})
	var appErr *apperrors.AppError
	if !errors.As(err, &appErr) || appErr.Code != apperrors.CodeConflict || appErr.Details["error_code"] != "OBJECT_UPLOAD_FAILED" {
		t.Fatalf("collision err=%v", err)
	}
	if store.existsCalls != 0 || repo.inserts != 0 {
		t.Fatalf("exists=%d inserts=%d", store.existsCalls, repo.inserts)
	}
}
