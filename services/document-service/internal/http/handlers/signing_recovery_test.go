package handlers

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/freight-platform/document-service/internal/domain"
	apperrors "github.com/freight-platform/document-service/internal/platform/errors"
	"github.com/freight-platform/document-service/internal/service"
)

type recoveryHTTPStore struct {
	recoveries int
	creates    int
	document   *domain.Document
	session    *domain.SigningSession
}

func (s *recoveryHTTPStore) CompanyExists(context.Context, uuid.UUID, uuid.UUID) (bool, error) {
	return true, nil
}
func (s *recoveryHTTPStore) UserExists(context.Context, uuid.UUID, uuid.UUID) (bool, error) {
	return true, nil
}
func (s *recoveryHTTPStore) GetByIDAndTenant(context.Context, uuid.UUID, uuid.UUID) (*domain.Document, error) {
	return s.document, nil
}
func (s *recoveryHTTPStore) CreateSession(context.Context, uuid.UUID, domain.CreateSigningSessionInput) (*domain.SigningSession, error) {
	s.creates++
	return nil, apperrors.Conflict("create should not run during recovery", nil)
}
func (s *recoveryHTTPStore) GetSessionByID(context.Context, uuid.UUID) (*domain.SigningSession, error) {
	return nil, apperrors.NotFound("signing session not found")
}
func (s *recoveryHTTPStore) GetSessionByIDAndTenant(context.Context, uuid.UUID, uuid.UUID) (*domain.SigningSession, error) {
	return nil, apperrors.NotFound("signing session not found")
}
func (s *recoveryHTTPStore) MarkSessionExpired(context.Context, uuid.UUID, uuid.UUID) (*domain.SigningSession, error) {
	return nil, apperrors.NotFound("signing session not found")
}
func (s *recoveryHTTPStore) AddSignature(context.Context, *domain.SigningSession, domain.AddSignatureInput) (*domain.Signature, *domain.SigningSession, *domain.Document, error) {
	return nil, nil, nil, apperrors.Conflict("signature should not run during recovery", nil)
}
func (s *recoveryHTTPStore) RecoverExpiredSigning(_ context.Context, documentID uuid.UUID, in domain.RecoverSigningInput) (*domain.Document, *domain.SigningSession, error) {
	s.recoveries++
	if documentID != s.document.ID || in.TenantID != s.document.TenantID || in.SigningSessionID != s.session.ID {
		return nil, nil, apperrors.NotFound("signing session not found")
	}
	doc := *s.document
	doc.DocumentStatus = domain.DocumentStatusReadyForSigning
	doc.Version++
	session := *s.session
	return &doc, &session, nil
}

// HANDLER_UNIT_TEST_ONLY exercises RecoverExpiredSigning through a router
// created in this test. It does not prove that document-service.NewRouter
// exposes or authorizes signing recovery.
func TestRecoverExpiredSigningHTTP(t *testing.T) {
	tenantID := uuid.New()
	documentID := uuid.New()
	sessionID := uuid.New()
	store := &recoveryHTTPStore{
		document: &domain.Document{
			ID: documentID, TenantID: tenantID, DocumentNumber: "REC-1", DocumentType: "ETRN",
			DocumentStatus: domain.DocumentStatusSigningInProgress, OwnerCompanyID: uuid.New(),
			LegalLanguage: "ru-RU", Version: 4, CreatedAt: time.Now().UTC(), UpdatedAt: time.Now().UTC(),
		},
		session: &domain.SigningSession{
			ID: sessionID, TenantID: tenantID, DocumentID: documentID,
			Status: domain.SigningSessionStatusExpired, RequiredSignersCount: 2, CompletedSignersCount: 1,
			CreatedAt: time.Now().UTC(),
		},
	}
	router := chi.NewRouter()
	router.Post("/v1/documents/{id}/signing-recovery", NewSigningHandler(service.NewSigningService(store, store)).RecoverExpiredSigning)

	t.Run("missing trusted tenant", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/v1/documents/"+documentID.String()+"/signing-recovery", strings.NewReader(`{"signing_session_id":"`+sessionID.String()+`","expected_version":4}`))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)
		if rec.Code != http.StatusUnauthorized {
			t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
		}
	})

	t.Run("spoofed body tenant", func(t *testing.T) {
		body := `{"tenant_id":"` + uuid.New().String() + `","signing_session_id":"` + sessionID.String() + `","expected_version":4}`
		req := httptest.NewRequest(http.MethodPost, "/v1/documents/"+documentID.String()+"/signing-recovery", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("X-Tenant-ID", tenantID.String())
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)
		if rec.Code != http.StatusUnauthorized {
			t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
		}
	})

	before := store.recoveries
	body := `{"signing_session_id":"` + sessionID.String() + `","expected_version":4}`
	req := httptest.NewRequest(http.MethodPost, "/v1/documents/"+documentID.String()+"/signing-recovery", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Tenant-ID", tenantID.String())
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	var payload struct {
		Document struct {
			Status  string `json:"document_status"`
			Version int    `json:"version"`
		} `json:"document"`
		Session struct {
			ID        string `json:"id"`
			Status    string `json:"status"`
			Completed int    `json:"completed_signers_count"`
		} `json:"signing_session"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatal(err)
	}
	if payload.Document.Status != domain.DocumentStatusReadyForSigning || payload.Document.Version != 5 {
		t.Fatalf("document=%+v", payload.Document)
	}
	if payload.Session.ID != sessionID.String() || payload.Session.Status != domain.SigningSessionStatusExpired || payload.Session.Completed != 1 {
		t.Fatalf("session=%+v", payload.Session)
	}
	if store.creates != 0 || store.recoveries != before+1 {
		t.Fatalf("creates=%d recoveries=%d", store.creates, store.recoveries)
	}

	getReq := httptest.NewRequest(http.MethodGet, "/v1/documents/"+documentID.String()+"/signing-recovery", nil)
	getRec := httptest.NewRecorder()
	router.ServeHTTP(getRec, getReq)
	if getRec.Code != http.StatusMethodNotAllowed && getRec.Code != http.StatusNotFound {
		t.Fatalf("GET status=%d", getRec.Code)
	}
}
