package handlers

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/freight-platform/document-service/internal/domain"
	apperrors "github.com/freight-platform/document-service/internal/platform/errors"
	"github.com/freight-platform/document-service/internal/repository"
	"github.com/freight-platform/document-service/internal/service"
)

type readStore struct {
	detailFn  func(ctx context.Context, id, tenantID uuid.UUID) (*repository.DocumentDetail, error)
	sessionFn func(ctx context.Context, id, tenantID uuid.UUID) (*domain.SigningSession, error)
	writes    int
}

func (s *readStore) CompanyExists(context.Context, uuid.UUID, uuid.UUID) (bool, error) {
	return true, nil
}
func (s *readStore) ShipmentExists(context.Context, uuid.UUID, uuid.UUID) (bool, error) {
	return true, nil
}
func (s *readStore) UserExists(context.Context, uuid.UUID, uuid.UUID) (bool, error) {
	return true, nil
}
func (s *readStore) CreateDocument(context.Context, domain.CreateDocumentInput) (*domain.Document, *domain.DocumentVersion, error) {
	s.writes++
	return nil, nil, nil
}
func (s *readStore) GetDetail(ctx context.Context, id, tenantID uuid.UUID) (*repository.DocumentDetail, error) {
	return s.detailFn(ctx, id, tenantID)
}
func (s *readStore) List(context.Context, domain.ListDocumentsFilter) ([]domain.Document, int, error) {
	return nil, 0, nil
}
func (s *readStore) GetByIDAndTenant(context.Context, uuid.UUID, uuid.UUID) (*domain.Document, error) {
	return nil, apperrors.NotFound("document not found")
}
func (s *readStore) HasVersions(context.Context, uuid.UUID) (bool, error) { return false, nil }
func (s *readStore) CreateVersion(context.Context, uuid.UUID, domain.CreateDocumentVersionInput) (*domain.DocumentVersion, error) {
	s.writes++
	return nil, nil
}
func (s *readStore) VersionBelongsToDocument(context.Context, uuid.UUID, uuid.UUID) (bool, error) {
	return false, nil
}
func (s *readStore) AddFile(context.Context, uuid.UUID, domain.CreateDocumentFileInput) (*domain.DocumentFile, error) {
	s.writes++
	return nil, nil
}
func (s *readStore) UpdateDocumentStatus(context.Context, uuid.UUID, uuid.UUID, string, int) (*domain.Document, error) {
	s.writes++
	return nil, nil
}
func (s *readStore) CreateSession(context.Context, uuid.UUID, domain.CreateSigningSessionInput) (*domain.SigningSession, error) {
	s.writes++
	return nil, nil
}
func (s *readStore) GetSessionByID(context.Context, uuid.UUID) (*domain.SigningSession, error) {
	return nil, apperrors.NotFound("signing session not found")
}
func (s *readStore) GetSessionByIDAndTenant(ctx context.Context, id, tenantID uuid.UUID) (*domain.SigningSession, error) {
	return s.sessionFn(ctx, id, tenantID)
}
func (s *readStore) AddSignature(context.Context, *domain.SigningSession, domain.AddSignatureInput) (*domain.Signature, *domain.SigningSession, *domain.Document, error) {
	s.writes++
	return nil, nil, nil, nil
}

func TestGetDocumentIgnoresQueryTenantAndRepeatedReadDoesNotWrite(t *testing.T) {
	tenantA := uuid.New()
	docA := uuid.New()
	var seenTenant uuid.UUID
	store := &readStore{
		detailFn: func(_ context.Context, id, tenantID uuid.UUID) (*repository.DocumentDetail, error) {
			seenTenant = tenantID
			if id != docA || tenantID != tenantA {
				return nil, apperrors.NotFound("document not found")
			}
			return &repository.DocumentDetail{Document: &domain.Document{
				ID: docA, TenantID: tenantA, DocumentNumber: "OWN-DOC", DocumentStatus: domain.DocumentStatusDraft,
				OwnerCompanyID: uuid.New(), LegalLanguage: "ru-RU",
			}}, nil
		},
	}
	handler := NewDocumentHandler(service.NewDocumentService(store))
	router := chi.NewRouter()
	router.Get("/v1/documents/{id}", handler.GetByID)

	otherTenant := uuid.New()
	req := httptest.NewRequest(http.MethodGet, "/v1/documents/"+docA.String()+"?tenant_id="+otherTenant.String(), nil)
	req.Header.Set("X-Tenant-ID", tenantA.String())
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("own read status=%d", rec.Code)
	}
	if seenTenant != tenantA {
		t.Fatal("query tenant_id changed the trusted tenant")
	}
	if store.writes != 0 {
		t.Fatal("get wrote state")
	}
	rec2 := httptest.NewRecorder()
	router.ServeHTTP(rec2, req)
	if rec2.Code != http.StatusOK || store.writes != 0 {
		t.Fatal("repeat get changed state")
	}
}

func TestGetDocumentForeignAndMissingShareNotFoundEnvelope(t *testing.T) {
	tenantA := uuid.New()
	docA := uuid.New()
	store := &readStore{
		detailFn: func(_ context.Context, id, tenantID uuid.UUID) (*repository.DocumentDetail, error) {
			if id == docA && tenantID == tenantA {
				return &repository.DocumentDetail{Document: &domain.Document{
					ID: docA, TenantID: tenantA, DocumentNumber: "SECRET-NUMBER", DocumentStatus: domain.DocumentStatusDraft,
					OwnerCompanyID: uuid.New(),
				}}, nil
			}
			return nil, apperrors.NotFound("document not found")
		},
	}
	handler := NewDocumentHandler(service.NewDocumentService(store))
	router := chi.NewRouter()
	router.Get("/v1/documents/{id}", handler.GetByID)

	foreign := httptest.NewRequest(http.MethodGet, "/v1/documents/"+docA.String(), nil)
	foreign.Header.Set("X-Tenant-ID", uuid.New().String())
	foreignRec := httptest.NewRecorder()
	router.ServeHTTP(foreignRec, foreign)

	missing := httptest.NewRequest(http.MethodGet, "/v1/documents/"+uuid.New().String(), nil)
	missing.Header.Set("X-Tenant-ID", tenantA.String())
	missingRec := httptest.NewRecorder()
	router.ServeHTTP(missingRec, missing)

	if foreignRec.Code != http.StatusNotFound || missingRec.Code != http.StatusNotFound {
		t.Fatalf("foreign=%d missing=%d", foreignRec.Code, missingRec.Code)
	}
	if foreignRec.Body.String() != missingRec.Body.String() {
		t.Fatal("foreign and missing envelopes differ")
	}
	body := foreignRec.Body.String()
	if body == "" || strings.Contains(body, "SECRET-NUMBER") || strings.Contains(body, docA.String()) || strings.Contains(body, tenantA.String()) {
		t.Fatal("not found body leaks document data")
	}
}

func TestGetDocumentWithoutTrustedTenantFailsClosed(t *testing.T) {
	called := false
	store := &readStore{
		detailFn: func(context.Context, uuid.UUID, uuid.UUID) (*repository.DocumentDetail, error) {
			called = true
			return nil, nil
		},
	}
	handler := NewDocumentHandler(service.NewDocumentService(store))
	router := chi.NewRouter()
	router.Get("/v1/documents/{id}", handler.GetByID)

	for _, header := range []string{"", "not-a-uuid"} {
		req := httptest.NewRequest(http.MethodGet, "/v1/documents/"+uuid.New().String(), nil)
		if header != "" {
			req.Header.Set("X-Tenant-ID", header)
		}
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)
		if rec.Code != http.StatusUnauthorized || called {
			t.Fatalf("header %q status=%d called=%v", header, rec.Code, called)
		}
	}
}

func TestGetSigningSessionForeignMatchesMissing(t *testing.T) {
	tenantA := uuid.New()
	sessionA := uuid.New()
	store := &readStore{
		sessionFn: func(_ context.Context, id, tenantID uuid.UUID) (*domain.SigningSession, error) {
			if id == sessionA && tenantID == tenantA {
				return &domain.SigningSession{ID: sessionA, TenantID: tenantA, DocumentID: uuid.New(), Status: domain.SigningSessionStatusCreated}, nil
			}
			return nil, apperrors.NotFound("signing session not found")
		},
	}
	handler := NewSigningHandler(service.NewSigningService(store, store))
	router := chi.NewRouter()
	router.Get("/v1/signing-sessions/{id}", handler.GetSession)

	own := httptest.NewRequest(http.MethodGet, "/v1/signing-sessions/"+sessionA.String(), nil)
	own.Header.Set("X-Tenant-ID", tenantA.String())
	ownRec := httptest.NewRecorder()
	router.ServeHTTP(ownRec, own)
	if ownRec.Code != http.StatusOK {
		t.Fatalf("own session status=%d", ownRec.Code)
	}

	foreign := httptest.NewRequest(http.MethodGet, "/v1/signing-sessions/"+sessionA.String()+"?tenant_id="+tenantA.String(), nil)
	foreign.Header.Set("X-Tenant-ID", uuid.New().String())
	foreignRec := httptest.NewRecorder()
	router.ServeHTTP(foreignRec, foreign)
	missing := httptest.NewRequest(http.MethodGet, "/v1/signing-sessions/"+uuid.New().String(), nil)
	missing.Header.Set("X-Tenant-ID", tenantA.String())
	missingRec := httptest.NewRecorder()
	router.ServeHTTP(missingRec, missing)
	if foreignRec.Code != http.StatusNotFound || foreignRec.Body.String() != missingRec.Body.String() {
		t.Fatal("foreign signing session is distinguishable from missing")
	}
	var payload map[string]any
	if err := json.Unmarshal(foreignRec.Body.Bytes(), &payload); err != nil {
		t.Fatal(err)
	}
}
