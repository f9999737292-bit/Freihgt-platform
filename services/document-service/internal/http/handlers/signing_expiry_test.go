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
	"github.com/freight-platform/document-service/internal/service"
)

type expiryStore struct {
	session *domain.SigningSession
	signed  int
}

func (s *expiryStore) CompanyExists(context.Context, uuid.UUID, uuid.UUID) (bool, error) {
	return true, nil
}
func (s *expiryStore) UserExists(context.Context, uuid.UUID, uuid.UUID) (bool, error) {
	return true, nil
}
func (s *expiryStore) GetByIDAndTenant(_ context.Context, id, tenantID uuid.UUID) (*domain.Document, error) {
	return &domain.Document{
		ID: id, TenantID: tenantID, DocumentStatus: domain.DocumentStatusSigningInProgress,
	}, nil
}
func (s *expiryStore) CreateSession(context.Context, uuid.UUID, domain.CreateSigningSessionInput) (*domain.SigningSession, error) {
	return nil, nil
}
func (s *expiryStore) GetSessionByID(context.Context, uuid.UUID) (*domain.SigningSession, error) {
	return nil, nil
}
func (s *expiryStore) GetSessionByIDAndTenant(context.Context, uuid.UUID, uuid.UUID) (*domain.SigningSession, error) {
	copySession := *s.session
	return &copySession, nil
}
func (s *expiryStore) MarkSessionExpired(context.Context, uuid.UUID, uuid.UUID) (*domain.SigningSession, error) {
	s.session.Status = domain.SigningSessionStatusExpired
	copySession := *s.session
	return &copySession, nil
}
func (s *expiryStore) AddSignature(context.Context, *domain.SigningSession, domain.AddSignatureInput) (*domain.Signature, *domain.SigningSession, *domain.Document, error) {
	s.signed++
	return nil, nil, nil, nil
}

func TestAddSignatureRejectsOverdueSessionAndGetReturnsExpired(t *testing.T) {
	tenantID := uuid.New()
	sessionID := uuid.New()
	expiredAt := time.Now().UTC().Add(-time.Hour)
	store := &expiryStore{session: &domain.SigningSession{
		ID: sessionID, TenantID: tenantID, DocumentID: uuid.New(),
		Status: domain.SigningSessionStatusCreated, RequiredSignersCount: 1,
		CreatedAt: time.Now().UTC(), ExpiresAt: &expiredAt,
	}}
	handler := NewSigningHandler(service.NewSigningService(store, store))
	router := chi.NewRouter()
	router.Get("/v1/signing-sessions/{id}", handler.GetSession)
	router.Post("/v1/signing-sessions/{id}/signatures", handler.AddSignature)

	body := `{"signer_user_id":"` + uuid.New().String() + `","signer_company_id":"` + uuid.New().String() + `","signature_type":"SIMPLE_ELECTRONIC"}`
	req := httptest.NewRequest(http.MethodPost, "/v1/signing-sessions/"+sessionID.String()+"/signatures", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Tenant-ID", tenantID.String())
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("signature status=%d body=%s", rec.Code, rec.Body.String())
	}
	var payload struct {
		Error struct {
			Code    string         `json:"code"`
			Details map[string]any `json:"details"`
		} `json:"error"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatal(err)
	}
	if payload.Error.Code != "VALIDATION_ERROR" || payload.Error.Details["status"] != domain.SigningSessionStatusExpired {
		t.Fatalf("unexpected error payload: %+v", payload.Error)
	}
	if store.signed != 0 {
		t.Fatalf("signature writes=%d", store.signed)
	}

	getReq := httptest.NewRequest(http.MethodGet, "/v1/signing-sessions/"+sessionID.String(), nil)
	getReq.Header.Set("X-Tenant-ID", tenantID.String())
	getRec := httptest.NewRecorder()
	router.ServeHTTP(getRec, getReq)
	if getRec.Code != http.StatusOK {
		t.Fatalf("get status=%d body=%s", getRec.Code, getRec.Body.String())
	}
	var session struct {
		Status string `json:"status"`
	}
	if err := json.Unmarshal(getRec.Body.Bytes(), &session); err != nil {
		t.Fatal(err)
	}
	if session.Status != domain.SigningSessionStatusExpired {
		t.Fatalf("api status=%s", session.Status)
	}
}
