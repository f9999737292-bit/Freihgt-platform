package http

import (
	"context"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"

	"github.com/google/uuid"

	"github.com/freight-platform/document-service/internal/domain"
	apperrors "github.com/freight-platform/document-service/internal/platform/errors"
	"github.com/freight-platform/document-service/internal/service"
	"github.com/freight-platform/shared-go/internalauth"
)

type recoveryRouteStore struct {
	calls      int
	version    int
	status     string
	signatures int
}

func (s *recoveryRouteStore) CompanyExists(context.Context, uuid.UUID, uuid.UUID) (bool, error) {
	return true, nil
}
func (s *recoveryRouteStore) UserExists(context.Context, uuid.UUID, uuid.UUID) (bool, error) {
	return true, nil
}
func (s *recoveryRouteStore) GetByIDAndTenant(context.Context, uuid.UUID, uuid.UUID) (*domain.Document, error) {
	return nil, apperrors.NotFound("document not found")
}
func (s *recoveryRouteStore) CreateSession(context.Context, uuid.UUID, domain.CreateSigningSessionInput) (*domain.SigningSession, error) {
	return nil, apperrors.Conflict("create should not run", nil)
}
func (s *recoveryRouteStore) GetSessionByID(context.Context, uuid.UUID) (*domain.SigningSession, error) {
	return nil, apperrors.NotFound("signing session not found")
}
func (s *recoveryRouteStore) GetSessionByIDAndTenant(context.Context, uuid.UUID, uuid.UUID) (*domain.SigningSession, error) {
	return nil, apperrors.NotFound("signing session not found")
}
func (s *recoveryRouteStore) MarkSessionExpired(context.Context, uuid.UUID, uuid.UUID) (*domain.SigningSession, error) {
	return nil, apperrors.NotFound("signing session not found")
}
func (s *recoveryRouteStore) AddSignature(context.Context, *domain.SigningSession, domain.AddSignatureInput) (*domain.Signature, *domain.SigningSession, *domain.Document, error) {
	return nil, nil, nil, apperrors.Conflict("signature should not run", nil)
}
func (s *recoveryRouteStore) RecoverExpiredSigning(context.Context, uuid.UUID, domain.RecoverSigningInput) (*domain.Document, *domain.SigningSession, error) {
	s.calls++
	s.version++
	s.status = domain.DocumentStatusReadyForSigning
	s.signatures = 0
	return nil, nil, apperrors.Conflict("recovery store was called", nil)
}

func TestDocumentServiceRouterDeniesSigningRecovery(t *testing.T) {
	store := &recoveryRouteStore{version: 4, status: domain.SigningSessionStatusExpired, signatures: 1}
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	router := NewRouter(
		log, nil, nil, service.NewSigningService(store, store), nil, nil, nil, nil, nil,
		internalauth.Config{Token: "dedicated-document-token", Environment: "test"},
	)
	documentID := uuid.New()
	sessionID := uuid.New()
	tenantID := uuid.New()
	otherTenant := uuid.New()
	exact := "/v1/documents/" + documentID.String() + "/signing-recovery"

	requests := []struct {
		name   string
		method string
		path   string
		header map[string]string
		body   string
	}{
		{name: "no headers", method: http.MethodPost, path: exact, body: recoveryBody(sessionID, 4, "")},
		{name: "tenant header", method: http.MethodPost, path: exact, header: map[string]string{"X-Tenant-ID": tenantID.String()}, body: recoveryBody(sessionID, 4, "")},
		{name: "foreign tenant header", method: http.MethodPost, path: exact, header: map[string]string{"X-Tenant-ID": otherTenant.String()}, body: recoveryBody(sessionID, 1, "")},
		{name: "bearer", method: http.MethodPost, path: exact, header: map[string]string{"Authorization": "Bearer eyJhbGciOiJub25lIn0.e30.sig", "X-Tenant-ID": tenantID.String()}, body: recoveryBody(sessionID, 4, "")},
		{name: "arbitrary service token", method: http.MethodPost, path: exact, header: map[string]string{"X-Internal-Service-Token": "not-a-real-token"}, body: recoveryBody(sessionID, 9, "")},
		{name: "configured service token", method: http.MethodPost, path: exact, header: map[string]string{"X-Internal-Service-Token": "dedicated-document-token", "X-Internal-Service-Name": "document-service"}, body: recoveryBody(sessionID, 4, "")},
		{name: "version zero", method: http.MethodPost, path: exact, header: map[string]string{"X-Tenant-ID": tenantID.String()}, body: recoveryBody(sessionID, 0, tenantID.String())},
		{name: "version mismatch body", method: http.MethodPost, path: exact, header: map[string]string{"X-Tenant-ID": tenantID.String()}, body: recoveryBody(sessionID, 99, tenantID.String())},
		{name: "matching body tenant", method: http.MethodPost, path: exact, header: map[string]string{"X-Tenant-ID": tenantID.String()}, body: recoveryBody(sessionID, 4, tenantID.String())},
		{name: "different body tenant", method: http.MethodPost, path: exact, header: map[string]string{"X-Tenant-ID": tenantID.String()}, body: recoveryBody(sessionID, 4, otherTenant.String())},
	}
	variants := []struct {
		name   string
		method string
		path   string
		header map[string]string
		body   string
	}{
		{name: "trailing slash", method: http.MethodPost, path: exact + "/", header: map[string]string{"X-Tenant-ID": tenantID.String()}, body: recoveryBody(sessionID, 4, tenantID.String())},
		{name: "extra segment", method: http.MethodPost, path: exact + "/now", header: map[string]string{"Authorization": "Bearer token"}, body: recoveryBody(sessionID, 4, "")},
		{name: "different case", method: http.MethodPost, path: "/v1/documents/" + documentID.String() + "/Signing-Recovery", header: map[string]string{"X-Tenant-ID": tenantID.String()}, body: recoveryBody(sessionID, 4, "")},
		{name: "encoded hyphen", method: http.MethodPost, path: "/v1/documents/" + documentID.String() + "/signing%2Drecovery", header: map[string]string{"X-Tenant-ID": tenantID.String()}, body: recoveryBody(sessionID, 4, tenantID.String())},
		{name: "encoded slash", method: http.MethodPost, path: "/v1/documents/" + documentID.String() + "/signing%2Frecovery", body: recoveryBody(sessionID, 4, "")},
		{name: "get", method: http.MethodGet, path: exact, header: map[string]string{"X-Tenant-ID": tenantID.String()}},
	}

	for _, tc := range requests {
		t.Run(tc.name, func(t *testing.T) {
			rec := serveRecovery(router, tc.method, tc.path, tc.header, tc.body)
			if rec.Code != http.StatusForbidden {
				t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
			}
		})
	}
	for _, tc := range variants {
		t.Run(tc.name, func(t *testing.T) {
			rec := serveRecovery(router, tc.method, tc.path, tc.header, tc.body)
			if rec.Code >= 200 && rec.Code < 300 {
				t.Fatalf("HTTP_SUCCESS status=%d body=%s", rec.Code, rec.Body.String())
			}
		})
	}
	if store.calls != 0 || store.version != 4 || store.status != domain.SigningSessionStatusExpired || store.signatures != 1 {
		t.Fatalf("calls=%d version=%d status=%s signatures=%d", store.calls, store.version, store.status, store.signatures)
	}
}

func serveRecovery(router http.Handler, method, path string, header map[string]string, body string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	for key, value := range header {
		req.Header.Set(key, value)
	}
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	return rec
}

func recoveryBody(sessionID uuid.UUID, version int, tenantID string) string {
	body := `{"signing_session_id":"` + sessionID.String() + `","expected_version":` + strconv.Itoa(version)
	if tenantID != "" {
		body += `,"tenant_id":"` + tenantID + `"`
	}
	return body + `}`
}
