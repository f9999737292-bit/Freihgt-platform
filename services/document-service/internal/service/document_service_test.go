package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/freight-platform/document-service/internal/domain"
	apperrors "github.com/freight-platform/document-service/internal/platform/errors"
	"github.com/freight-platform/document-service/internal/repository"
)

type mockDocumentStore struct {
	createFn           func(ctx context.Context, in domain.CreateDocumentInput) (*domain.Document, *domain.DocumentVersion, error)
	getDetailFn        func(ctx context.Context, id, tenantID uuid.UUID) (*repository.DocumentDetail, error)
	getByIDAndTenantFn func(ctx context.Context, id, tenantID uuid.UUID) (*domain.Document, error)
	hasVersionsFn      func(ctx context.Context, documentID uuid.UUID) (bool, error)
	createVersionFn    func(ctx context.Context, documentID uuid.UUID, in domain.CreateDocumentVersionInput) (*domain.DocumentVersion, error)
	updateStatusFn     func(ctx context.Context, id, tenantID uuid.UUID, status string, expectedVersion int) (*domain.Document, error)
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
func (m *mockDocumentStore) GetDetail(ctx context.Context, id, tenantID uuid.UUID) (*repository.DocumentDetail, error) {
	if m.getDetailFn == nil {
		return nil, nil
	}
	return m.getDetailFn(ctx, id, tenantID)
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
	markExpiredFn   func(ctx context.Context, id, tenantID uuid.UUID) (*domain.SigningSession, error)
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
func (m *mockSigningStore) MarkSessionExpired(ctx context.Context, id, tenantID uuid.UUID) (*domain.SigningSession, error) {
	if m.markExpiredFn != nil {
		return m.markExpiredFn(ctx, id, tenantID)
	}
	return nil, apperrors.NotFound("signing session not found")
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

func TestSigningServiceAddSignatureRejectsOverdueSession(t *testing.T) {
	t.Parallel()
	tenantID := uuid.New()
	sessionID := uuid.New()
	documentID := uuid.New()
	expiredAt := time.Now().UTC().Add(-time.Minute)
	var marked bool
	var signed bool
	docRepo := &mockDocumentStore{
		getByIDAndTenantFn: func(context.Context, uuid.UUID, uuid.UUID) (*domain.Document, error) {
			t.Fatal("overdue session must not load the document for signing")
			return nil, nil
		},
	}
	svc := NewSigningService(&mockSigningStore{
		getSessionFn: func(context.Context, uuid.UUID, uuid.UUID) (*domain.SigningSession, error) {
			return &domain.SigningSession{
				ID: sessionID, TenantID: tenantID, DocumentID: documentID,
				Status: domain.SigningSessionStatusCreated, RequiredSignersCount: 1,
				ExpiresAt: &expiredAt,
			}, nil
		},
		markExpiredFn: func(_ context.Context, id, tenant uuid.UUID) (*domain.SigningSession, error) {
			marked = true
			if id != sessionID || tenant != tenantID {
				t.Fatalf("expiry update used id=%s tenant=%s", id, tenant)
			}
			return &domain.SigningSession{
				ID: id, TenantID: tenant, DocumentID: documentID,
				Status: domain.SigningSessionStatusExpired, ExpiresAt: &expiredAt,
			}, nil
		},
		addSignatureFn: func(context.Context, *domain.SigningSession, domain.AddSignatureInput) (*domain.Signature, *domain.SigningSession, *domain.Document, error) {
			signed = true
			t.Fatal("overdue session must not store a signature")
			return nil, nil, nil, nil
		},
	}, docRepo)

	_, session, doc, err := svc.AddSignature(context.Background(), sessionID, domain.AddSignatureInput{
		TenantID: tenantID, SignerUserID: uuid.New(), SignerCompanyID: uuid.New(),
		SignatureType: domain.SignatureTypeSimpleElectronic,
	})
	var appErr *apperrors.AppError
	if !errors.As(err, &appErr) || appErr.Code != apperrors.CodeValidation {
		t.Fatalf("expected validation error, got %v", err)
	}
	if appErr.Details["status"] != domain.SigningSessionStatusExpired {
		t.Fatalf("expected EXPIRED status detail, got %#v", appErr.Details)
	}
	if !marked || signed || session != nil || doc != nil {
		t.Fatalf("marked=%v signed=%v session=%v doc=%v", marked, signed, session, doc)
	}
}

func TestSigningServiceGetSessionReturnsExpiredStatus(t *testing.T) {
	t.Parallel()
	tenantID := uuid.New()
	sessionID := uuid.New()
	expiredAt := time.Now().UTC().Add(-time.Minute)
	svc := NewSigningService(&mockSigningStore{
		getSessionFn: func(context.Context, uuid.UUID, uuid.UUID) (*domain.SigningSession, error) {
			return &domain.SigningSession{
				ID: sessionID, TenantID: tenantID, Status: domain.SigningSessionStatusInProgress,
				ExpiresAt: &expiredAt,
			}, nil
		},
		markExpiredFn: func(_ context.Context, id, tenant uuid.UUID) (*domain.SigningSession, error) {
			return &domain.SigningSession{
				ID: id, TenantID: tenant, Status: domain.SigningSessionStatusExpired, ExpiresAt: &expiredAt,
			}, nil
		},
	}, &mockDocumentStore{})

	session, err := svc.GetSession(context.Background(), sessionID, tenantID)
	if err != nil {
		t.Fatalf("get session: %v", err)
	}
	if session.Status != domain.SigningSessionStatusExpired || session.ExpiresAt == nil || !session.ExpiresAt.Equal(expiredAt) {
		t.Fatalf("expected persisted EXPIRED session, got %+v", session)
	}
}

func TestSigningServiceCreateSessionRejectsPastExpiry(t *testing.T) {
	t.Parallel()
	past := time.Now().UTC().Add(-time.Second)
	svc := NewSigningService(&mockSigningStore{
		createSessionFn: func(context.Context, uuid.UUID, domain.CreateSigningSessionInput) (*domain.SigningSession, error) {
			t.Fatal("past expires_at must not create a session")
			return nil, nil
		},
	}, &mockDocumentStore{
		getByIDAndTenantFn: func(context.Context, uuid.UUID, uuid.UUID) (*domain.Document, error) {
			t.Fatal("past expires_at must not load the document")
			return nil, nil
		},
	})
	_, err := svc.CreateSession(context.Background(), uuid.New(), domain.CreateSigningSessionInput{
		TenantID: uuid.New(), RequiredSignersCount: 1, ExpiresAt: &past,
	})
	var appErr *apperrors.AppError
	if !errors.As(err, &appErr) || appErr.Code != apperrors.CodeValidation || appErr.Details["field"] != "expires_at" {
		t.Fatalf("expected expires_at validation error, got %v", err)
	}
}

func TestDocumentReadPassesTrustedTenantToStore(t *testing.T) {
	t.Parallel()
	docID := uuid.New()
	tenantID := uuid.New()
	var gotID, gotTenant uuid.UUID
	svc := NewDocumentService(&mockDocumentStore{
		getDetailFn: func(_ context.Context, id, tenant uuid.UUID) (*repository.DocumentDetail, error) {
			gotID, gotTenant = id, tenant
			return &repository.DocumentDetail{Document: &domain.Document{ID: id, TenantID: tenant}}, nil
		},
	})
	detail, err := svc.GetByID(context.Background(), docID, tenantID)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if detail.Document.ID != docID || gotID != docID || gotTenant != tenantID {
		t.Fatal("read did not use the trusted tenant")
	}
}

func TestDocumentReadRejectsMissingTenantBeforeLookup(t *testing.T) {
	t.Parallel()
	called := false
	svc := NewDocumentService(&mockDocumentStore{
		getDetailFn: func(context.Context, uuid.UUID, uuid.UUID) (*repository.DocumentDetail, error) {
			called = true
			return nil, nil
		},
	})
	_, err := svc.GetByID(context.Background(), uuid.New(), uuid.Nil)
	var appErr *apperrors.AppError
	if !errors.As(err, &appErr) || appErr.Code != apperrors.CodeUnauthorized || called {
		t.Fatal("missing tenant must fail closed before lookup")
	}
}

func TestSigningSessionReadUsesTenantPredicate(t *testing.T) {
	t.Parallel()
	sessionID := uuid.New()
	tenantID := uuid.New()
	var gotID, gotTenant uuid.UUID
	svc := NewSigningService(&mockSigningStore{
		getSessionFn: func(_ context.Context, id, tenant uuid.UUID) (*domain.SigningSession, error) {
			gotID, gotTenant = id, tenant
			if tenant != tenantID {
				return nil, apperrors.NotFound("signing session not found")
			}
			return &domain.SigningSession{ID: id, TenantID: tenant}, nil
		},
	}, &mockDocumentStore{})
	session, err := svc.GetSession(context.Background(), sessionID, tenantID)
	if err != nil || session.ID != sessionID || gotID != sessionID || gotTenant != tenantID {
		t.Fatal("session read did not use the trusted tenant")
	}
	_, err = svc.GetSession(context.Background(), sessionID, uuid.New())
	var appErr *apperrors.AppError
	if !errors.As(err, &appErr) || appErr.Code != apperrors.CodeNotFound {
		t.Fatal("foreign tenant session must be not found")
	}
}
