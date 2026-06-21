package service

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"

	"github.com/freight-platform/document-service/internal/domain"
	"github.com/freight-platform/document-service/internal/repository"
	apperrors "github.com/freight-platform/document-service/internal/platform/errors"
)

type mockDocumentStore struct {
	createFn            func(ctx context.Context, in domain.CreateDocumentInput) (*domain.Document, *domain.DocumentVersion, error)
	getByIDAndTenantFn  func(ctx context.Context, id, tenantID uuid.UUID) (*domain.Document, error)
	hasVersionsFn       func(ctx context.Context, documentID uuid.UUID) (bool, error)
	createVersionFn     func(ctx context.Context, documentID uuid.UUID, in domain.CreateDocumentVersionInput) (*domain.DocumentVersion, error)
	updateStatusFn      func(ctx context.Context, id, tenantID uuid.UUID, status string, expectedVersion int) (*domain.Document, error)
}

func (m *mockDocumentStore) CompanyExists(context.Context, uuid.UUID, uuid.UUID) (bool, error) {
	return true, nil
}
func (m *mockDocumentStore) ShipmentExists(context.Context, uuid.UUID, uuid.UUID) (bool, error) {
	return true, nil
}
func (m *mockDocumentStore) CreateDocument(ctx context.Context, in domain.CreateDocumentInput) (*domain.Document, *domain.DocumentVersion, error) {
	return m.createFn(ctx, in)
}
func (m *mockDocumentStore) GetDetail(context.Context, uuid.UUID) (*repository.DocumentDetail, error) {
	return nil, nil
}
func (m *mockDocumentStore) List(context.Context, domain.ListDocumentsFilter) ([]domain.Document, int, error) {
	return nil, 0, nil
}
func (m *mockDocumentStore) GetByIDAndTenant(ctx context.Context, id, tenantID uuid.UUID) (*domain.Document, error) {
	return m.getByIDAndTenantFn(ctx, id, tenantID)
}
func (m *mockDocumentStore) HasVersions(ctx context.Context, documentID uuid.UUID) (bool, error) {
	return m.hasVersionsFn(ctx, documentID)
}
func (m *mockDocumentStore) CreateVersion(ctx context.Context, documentID uuid.UUID, in domain.CreateDocumentVersionInput) (*domain.DocumentVersion, error) {
	return m.createVersionFn(ctx, documentID, in)
}
func (m *mockDocumentStore) VersionBelongsToDocument(context.Context, uuid.UUID, uuid.UUID) (bool, error) {
	return true, nil
}
func (m *mockDocumentStore) AddFile(context.Context, uuid.UUID, domain.CreateDocumentFileInput) (*domain.DocumentFile, error) {
	return nil, nil
}
func (m *mockDocumentStore) UpdateDocumentStatus(ctx context.Context, id, tenantID uuid.UUID, status string, expectedVersion int) (*domain.Document, error) {
	return m.updateStatusFn(ctx, id, tenantID, status, expectedVersion)
}

type mockSigningStore struct {
	createSessionFn func(ctx context.Context, documentID uuid.UUID, in domain.CreateSigningSessionInput) (*domain.SigningSession, error)
	getSessionFn    func(ctx context.Context, id, tenantID uuid.UUID) (*domain.SigningSession, error)
	addSignatureFn  func(ctx context.Context, session *domain.SigningSession, in domain.AddSignatureInput) (*domain.Signature, *domain.SigningSession, *domain.Document, error)
}

func (m *mockSigningStore) CompanyExists(context.Context, uuid.UUID, uuid.UUID) (bool, error) {
	return true, nil
}
func (m *mockSigningStore) UserExists(context.Context, uuid.UUID, uuid.UUID) (bool, error) {
	return true, nil
}
func (m *mockSigningStore) CreateSession(ctx context.Context, documentID uuid.UUID, in domain.CreateSigningSessionInput) (*domain.SigningSession, error) {
	return m.createSessionFn(ctx, documentID, in)
}
func (m *mockSigningStore) GetSessionByID(context.Context, uuid.UUID) (*domain.SigningSession, error) {
	return nil, nil
}
func (m *mockSigningStore) GetSessionByIDAndTenant(ctx context.Context, id, tenantID uuid.UUID) (*domain.SigningSession, error) {
	return m.getSessionFn(ctx, id, tenantID)
}
func (m *mockSigningStore) AddSignature(ctx context.Context, session *domain.SigningSession, in domain.AddSignatureInput) (*domain.Signature, *domain.SigningSession, *domain.Document, error) {
	return m.addSignatureFn(ctx, session, in)
}

func TestDocumentServiceCreateValidation(t *testing.T) {
	t.Parallel()
	svc := NewDocumentService(&mockDocumentStore{})
	_, err := svc.Create(context.Background(), domain.CreateDocumentInput{})
	if err == nil {
		t.Fatalf("expected validation error")
	}
}

func TestDocumentServiceCreateFirstVersion(t *testing.T) {
	t.Parallel()
	svc := NewDocumentService(&mockDocumentStore{
		createFn: func(_ context.Context, in domain.CreateDocumentInput) (*domain.Document, *domain.DocumentVersion, error) {
			return &domain.Document{DocumentStatus: domain.DocumentStatusDraft}, &domain.DocumentVersion{VersionNumber: 1}, nil
		},
	})
	doc, err := svc.Create(context.Background(), domain.CreateDocumentInput{
		TenantID: uuid.New(), DocumentNumber: "DOC-1", DocumentType: "ETRN",
		OwnerCompanyID: uuid.New(), LegalLanguage: "ru-RU",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if doc.DocumentStatus != domain.DocumentStatusDraft {
		t.Fatalf("unexpected status: %s", doc.DocumentStatus)
	}
}

func TestDocumentServiceCreateVersionOnlyDraftOrRejected(t *testing.T) {
	t.Parallel()
	svc := NewDocumentService(&mockDocumentStore{
		getByIDAndTenantFn: func(context.Context, uuid.UUID, uuid.UUID) (*domain.Document, error) {
			return &domain.Document{DocumentStatus: domain.DocumentStatusSigned}, nil
		},
	})
	_, err := svc.CreateVersion(context.Background(), uuid.New(), domain.CreateDocumentVersionInput{TenantID: uuid.New()})
	if err == nil {
		t.Fatalf("expected validation error")
	}
}

func TestDocumentServiceReadyForSigningOnlyFromDraft(t *testing.T) {
	t.Parallel()
	svc := NewDocumentService(&mockDocumentStore{
		getByIDAndTenantFn: func(context.Context, uuid.UUID, uuid.UUID) (*domain.Document, error) {
			return &domain.Document{DocumentStatus: domain.DocumentStatusSigned}, nil
		},
	})
	_, err := svc.ReadyForSigning(context.Background(), uuid.New(), domain.ReadyForSigningInput{TenantID: uuid.New()})
	if err == nil {
		t.Fatalf("expected validation error")
	}
}

func TestDocumentServiceCancelSignedDocument(t *testing.T) {
	t.Parallel()
	svc := NewDocumentService(&mockDocumentStore{
		getByIDAndTenantFn: func(context.Context, uuid.UUID, uuid.UUID) (*domain.Document, error) {
			return &domain.Document{DocumentStatus: domain.DocumentStatusSigned}, nil
		},
	})
	_, err := svc.Cancel(context.Background(), uuid.New(), domain.CancelDocumentInput{TenantID: uuid.New()})
	var appErr *apperrors.AppError
	if !errors.As(err, &appErr) || appErr.Code != apperrors.CodeValidation {
		t.Fatalf("expected validation error, got %v", err)
	}
}

func TestDocumentServiceArchiveOnlySignedOrAccepted(t *testing.T) {
	t.Parallel()
	svc := NewDocumentService(&mockDocumentStore{
		getByIDAndTenantFn: func(context.Context, uuid.UUID, uuid.UUID) (*domain.Document, error) {
			return &domain.Document{DocumentStatus: domain.DocumentStatusDraft}, nil
		},
	})
	_, err := svc.Archive(context.Background(), uuid.New(), domain.ArchiveDocumentInput{TenantID: uuid.New()})
	if err == nil {
		t.Fatalf("expected validation error")
	}
}

func TestSigningServiceCreateSession(t *testing.T) {
	t.Parallel()
	docRepo := &mockDocumentStore{
		getByIDAndTenantFn: func(context.Context, uuid.UUID, uuid.UUID) (*domain.Document, error) {
			return &domain.Document{DocumentStatus: domain.DocumentStatusReadyForSigning}, nil
		},
	}
	svc := NewSigningService(&mockSigningStore{
		createSessionFn: func(context.Context, uuid.UUID, domain.CreateSigningSessionInput) (*domain.SigningSession, error) {
			return &domain.SigningSession{Status: domain.SigningSessionStatusCreated}, nil
		},
	}, docRepo)
	session, err := svc.CreateSession(context.Background(), uuid.New(), domain.CreateSigningSessionInput{
		TenantID: uuid.New(), RequiredSignersCount: 2,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if session.Status != domain.SigningSessionStatusCreated {
		t.Fatalf("unexpected status: %s", session.Status)
	}
}

func TestSigningServiceAddSignatureCompletesDocument(t *testing.T) {
	t.Parallel()
	docRepo := &mockDocumentStore{
		getByIDAndTenantFn: func(context.Context, uuid.UUID, uuid.UUID) (*domain.Document, error) {
			return &domain.Document{DocumentStatus: domain.DocumentStatusSigningInProgress}, nil
		},
	}
	svc := NewSigningService(&mockSigningStore{
		getSessionFn: func(context.Context, uuid.UUID, uuid.UUID) (*domain.SigningSession, error) {
			return &domain.SigningSession{
				Status: domain.SigningSessionStatusInProgress, RequiredSignersCount: 1, CompletedSignersCount: 0,
			}, nil
		},
		addSignatureFn: func(_ context.Context, _ *domain.SigningSession, _ domain.AddSignatureInput) (*domain.Signature, *domain.SigningSession, *domain.Document, error) {
			return &domain.Signature{VerificationStatus: domain.SignatureVerificationValid},
				&domain.SigningSession{Status: domain.SigningSessionStatusCompleted},
				&domain.Document{DocumentStatus: domain.DocumentStatusSigned}, nil
		},
	}, docRepo)
	_, session, doc, err := svc.AddSignature(context.Background(), uuid.New(), domain.AddSignatureInput{
		TenantID: uuid.New(), SignerUserID: uuid.New(), SignerCompanyID: uuid.New(),
		SignatureType: domain.SignatureTypeSimpleElectronic,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if session.Status != domain.SigningSessionStatusCompleted || doc.DocumentStatus != domain.DocumentStatusSigned {
		t.Fatalf("expected completed signing and SIGNED document")
	}
}
