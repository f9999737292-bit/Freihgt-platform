package handlers

import (
	"context"
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

type signingAuthorityStore struct {
	writes int
	calls  int
	seen   uuid.UUID
	docID  uuid.UUID
}

func (s *signingAuthorityStore) touch(tenantID uuid.UUID) {
	s.calls++
	s.seen = tenantID
}

func (s *signingAuthorityStore) CompanyExists(context.Context, uuid.UUID, uuid.UUID) (bool, error) {
	return true, nil
}
func (s *signingAuthorityStore) UserExists(context.Context, uuid.UUID, uuid.UUID) (bool, error) {
	return true, nil
}
func (s *signingAuthorityStore) GetByIDAndTenant(_ context.Context, id, tenantID uuid.UUID) (*domain.Document, error) {
	s.touch(tenantID)
	return &domain.Document{
		ID: id, TenantID: tenantID, DocumentNumber: "SGN", DocumentType: "ETRN",
		DocumentStatus: domain.DocumentStatusReadyForSigning, OwnerCompanyID: uuid.New(),
		LegalLanguage: "ru-RU", CreatedAt: time.Now(), UpdatedAt: time.Now(),
	}, nil
}
func (s *signingAuthorityStore) CreateSession(_ context.Context, documentID uuid.UUID, in domain.CreateSigningSessionInput) (*domain.SigningSession, error) {
	s.writes++
	s.touch(in.TenantID)
	s.docID = documentID
	return &domain.SigningSession{
		ID: uuid.New(), TenantID: in.TenantID, DocumentID: documentID,
		Status: domain.SigningSessionStatusCreated, RequiredSignersCount: in.RequiredSignersCount,
		CreatedAt: time.Now(),
	}, nil
}
func (s *signingAuthorityStore) GetSessionByID(context.Context, uuid.UUID) (*domain.SigningSession, error) {
	s.calls++
	return nil, nil
}
func (s *signingAuthorityStore) GetSessionByIDAndTenant(_ context.Context, id, tenantID uuid.UUID) (*domain.SigningSession, error) {
	s.touch(tenantID)
	return &domain.SigningSession{
		ID: id, TenantID: tenantID, DocumentID: s.docID,
		Status: domain.SigningSessionStatusCreated, RequiredSignersCount: 1, CreatedAt: time.Now(),
	}, nil
}
func (s *signingAuthorityStore) AddSignature(_ context.Context, session *domain.SigningSession, in domain.AddSignatureInput) (*domain.Signature, *domain.SigningSession, *domain.Document, error) {
	s.writes++
	s.touch(in.TenantID)
	signature := &domain.Signature{
		ID: uuid.New(), TenantID: in.TenantID, SigningSessionID: session.ID, DocumentID: session.DocumentID,
		SignerUserID: &in.SignerUserID, SignerCompanyID: &in.SignerCompanyID,
		SignatureType: in.SignatureType, VerificationStatus: domain.VerificationUnverified,
		CreatedAt: time.Now(),
	}
	doc := &domain.Document{
		ID: session.DocumentID, TenantID: in.TenantID, DocumentNumber: "SGN", DocumentType: "ETRN",
		DocumentStatus: domain.DocumentStatusSigningInProgress, OwnerCompanyID: uuid.New(),
		LegalLanguage: "ru-RU", CreatedAt: time.Now(), UpdatedAt: time.Now(),
	}
	return signature, session, doc, nil
}

func TestSigningTenantAuthority(t *testing.T) {
	tenantA := uuid.New()
	tenantB := uuid.New()
	docID := uuid.New()
	sessionID := uuid.New()
	signer := uuid.New()
	company := uuid.New()
	store := &signingAuthorityStore{docID: docID}
	handler := NewSigningHandler(service.NewSigningService(store, store))
	router := chi.NewRouter()
	router.Post("/v1/documents/{id}/signing-sessions", handler.CreateSession)
	router.Post("/v1/signing-sessions/{id}/signatures", handler.AddSignature)

	sessionBody := func(tenant string) string {
		body := `{"required_signers_count":1`
		if tenant != "" {
			body += `,"tenant_id":"` + tenant + `"`
		}
		return body + `}`
	}
	signatureBody := func(tenant string) string {
		body := `{"signer_user_id":"` + signer.String() + `","signer_company_id":"` + company.String() + `","signature_type":"SIMPLE_ELECTRONIC"`
		if tenant != "" {
			body += `,"tenant_id":"` + tenant + `"`
		}
		return body + `}`
	}
	post := func(path, body, tenant string) *httptest.ResponseRecorder {
		req := httptest.NewRequest(http.MethodPost, path, strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		if tenant != "" {
			req.Header.Set("X-Tenant-ID", tenant)
		}
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)
		return rec
	}
	reset := func() {
		store.writes = 0
		store.calls = 0
		store.seen = uuid.Nil
	}
	assertDenied := func(name string, rec *httptest.ResponseRecorder) {
		t.Helper()
		if rec.Code != http.StatusUnauthorized || store.writes != 0 || store.calls != 0 || strings.Contains(rec.Body.String(), tenantB.String()) {
			t.Fatalf("%s status=%d writes=%d calls=%d body=%s", name, rec.Code, store.writes, store.calls, rec.Body.String())
		}
	}
	assertTenantA := func(name string, rec *httptest.ResponseRecorder) {
		t.Helper()
		if rec.Code != http.StatusCreated || store.seen != tenantA || !strings.Contains(rec.Body.String(), tenantA.String()) {
			t.Fatalf("%s status=%d seen=%s body=%s", name, rec.Code, store.seen, rec.Body.String())
		}
	}

	// SGN-TEN-01
	reset()
	assertDenied("SGN-TEN-01", post("/v1/documents/"+docID.String()+"/signing-sessions", sessionBody(tenantB.String()), tenantA.String()))

	// SGN-TEN-02
	reset()
	assertTenantA("SGN-TEN-02", post("/v1/documents/"+docID.String()+"/signing-sessions", sessionBody(""), tenantA.String()))

	// SGN-TEN-03
	reset()
	assertTenantA("SGN-TEN-03", post("/v1/documents/"+docID.String()+"/signing-sessions", sessionBody(tenantA.String()), tenantA.String()))

	// SGN-TEN-04
	reset()
	assertDenied("SGN-TEN-04", post("/v1/signing-sessions/"+sessionID.String()+"/signatures", signatureBody(tenantB.String()), tenantA.String()))

	// SGN-TEN-05
	reset()
	assertTenantA("SGN-TEN-05", post("/v1/signing-sessions/"+sessionID.String()+"/signatures", signatureBody(""), tenantA.String()))

	// SGN-TEN-06
	reset()
	assertTenantA("SGN-TEN-06", post("/v1/signing-sessions/"+sessionID.String()+"/signatures", signatureBody(tenantA.String()), tenantA.String()))

	// SGN-TEN-07
	for _, tc := range []struct {
		name, path, body, header string
	}{
		{"missing session header", "/v1/documents/" + docID.String() + "/signing-sessions", sessionBody(""), ""},
		{"malformed session header", "/v1/documents/" + docID.String() + "/signing-sessions", sessionBody(""), "not-a-uuid"},
		{"nil session header", "/v1/documents/" + docID.String() + "/signing-sessions", sessionBody(""), uuid.Nil.String()},
		{"missing signature header", "/v1/signing-sessions/" + sessionID.String() + "/signatures", signatureBody(""), ""},
		{"malformed signature header", "/v1/signing-sessions/" + sessionID.String() + "/signatures", signatureBody(""), "not-a-uuid"},
	} {
		reset()
		assertDenied("SGN-TEN-07 "+tc.name, post(tc.path, tc.body, tc.header))
	}
}
