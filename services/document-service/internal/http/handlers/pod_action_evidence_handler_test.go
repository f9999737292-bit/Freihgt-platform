package handlers

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/freight-platform/document-service/internal/domain"
	apperrors "github.com/freight-platform/document-service/internal/platform/errors"
	"github.com/freight-platform/document-service/internal/service"
	"github.com/freight-platform/shared-go/internalauth"
)

const (
	actionDedicatedToken = "dedicated-shipment-pod-token"
	actionGenericToken   = "generic-internal-service-token"
)

type authStore struct {
	subject    *domain.DeliveryActionSubject
	rows       []domain.PODActionEvidence
	subjectErr error
	listErr    error
}

func (authStore) CompanyExists(context.Context, uuid.UUID, uuid.UUID) (bool, error) {
	return false, nil
}
func (authStore) FindIntentByIdempotency(context.Context, uuid.UUID, string) (*domain.PODActionIntent, error) {
	return nil, nil
}
func (authStore) RotateUploadToken(context.Context, uuid.UUID, string) error { return nil }
func (authStore) LoadIntent(context.Context, uuid.UUID) (*domain.PODActionIntent, error) {
	return nil, apperrors.NotFound("upload intent not found")
}
func (authStore) InsertIntent(context.Context, domain.PODActionIntent) (domain.PODActionIntent, error) {
	return domain.PODActionIntent{}, nil
}
func (authStore) MarkIntentUploaded(context.Context, uuid.UUID, int64) error { return nil }
func (authStore) EvidenceByIntent(context.Context, uuid.UUID) (*domain.PODActionEvidence, error) {
	return nil, apperrors.NotFound("upload intent not found")
}
func (authStore) CompleteIntent(context.Context, uuid.UUID, string, string, *string) (*domain.PODActionEvidence, error) {
	return nil, apperrors.NotFound("upload intent not found")
}
func (s authStore) ListPresentEvidence(context.Context, uuid.UUID, uuid.UUID, uuid.UUID) ([]domain.PODActionEvidence, error) {
	if s.listErr != nil {
		return nil, s.listErr
	}
	return s.rows, nil
}

func actionRouter(store service.PODActionEvidenceStore) http.Handler {
	svc := service.NewPODActionEvidenceService(store, nil, 1024, service.ActionObjectPlacement{StorageProvider: "local"})
	h := NewPODActionEvidenceHandler(svc)
	r := chi.NewRouter()
	r.Route("/internal/v1/pod-action-evidence", func(r chi.Router) {
		r.Use(internalauth.Config{Token: actionDedicatedToken, Environment: "test"}.Middleware)
		r.Use(RequireShipmentServiceCaller)
		r.Post("/upload-intents", h.CreateIntent)
		r.Put("/upload-intents/{uploadId}/content", h.UploadContent)
		r.Post("/upload-intents/{uploadId}/complete", h.Complete)
		r.Get("/{operatingTenantId}/{executionId}/{executionActionId}", h.GetEvidence)
	})
	return r
}

func actionRequest(method, path, body string, token, caller, tenant string) *http.Request {
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	if token != "" {
		req.Header.Set(internalauth.HeaderName, token)
	}
	if caller != "" {
		req.Header.Set(HeaderInternalServiceName, caller)
	}
	if tenant != "" {
		req.Header.Set("X-Tenant-ID", tenant)
	}
	return req
}

func TestPODActionInternalAuthRejectsMissingToken(t *testing.T) {
	rec := httptest.NewRecorder()
	actionRouter(authStore{}).ServeHTTP(rec, actionRequest(http.MethodPost, "/internal/v1/pod-action-evidence/upload-intents", `{}`, "", "shipment-service", uuid.NewString()))
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
}

func TestPODActionInternalAuthRejectsWrongCaller(t *testing.T) {
	rec := httptest.NewRecorder()
	actionRouter(authStore{}).ServeHTTP(rec, actionRequest(http.MethodPost, "/internal/v1/pod-action-evidence/upload-intents", `{}`, actionDedicatedToken, "api-gateway", ""))
	if rec.Code != http.StatusForbidden {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
}

func TestPODActionInternalAuthRejectsTenantHeaderOnly(t *testing.T) {
	rec := httptest.NewRecorder()
	actionRouter(authStore{}).ServeHTTP(rec, actionRequest(http.MethodGet, "/internal/v1/pod-action-evidence/"+uuid.NewString()+"/"+uuid.NewString()+"/"+uuid.NewString(), "", "", "", uuid.NewString()))
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
}

func TestPODActionInternalAuthAcceptsShipmentService(t *testing.T) {
	rec := httptest.NewRecorder()
	actionRouter(authStore{}).ServeHTTP(rec, actionRequest(http.MethodPost, "/internal/v1/pod-action-evidence/upload-intents", `{}`, actionDedicatedToken, AuthorizedPODEvidenceCaller, uuid.NewString()))
	if rec.Code == http.StatusUnauthorized || rec.Code == http.StatusForbidden {
		t.Fatalf("authorized caller rejected: %d %s", rec.Code, rec.Body.String())
	}
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected validation after auth, got %d %s", rec.Code, rec.Body.String())
	}
}

func TestPODActionReadDoesNotLeakDocumentID(t *testing.T) {
	secret := uuid.New()
	rec := httptest.NewRecorder()
	actionRouter(authStore{}).ServeHTTP(rec, actionRequest(http.MethodGet, "/internal/v1/pod-action-evidence/"+uuid.NewString()+"/"+uuid.NewString()+"/"+uuid.NewString(), "", actionDedicatedToken, AuthorizedPODEvidenceCaller, ""))
	if rec.Code != http.StatusOK || bytes.Contains(rec.Body.Bytes(), []byte(secret.String())) || bytes.Contains(rec.Body.Bytes(), []byte("documentId")) {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
}

func TestGenericInternalTokenCannotSpoofShipmentCaller(t *testing.T) {
	rec := httptest.NewRecorder()
	actionRouter(authStore{}).ServeHTTP(rec, actionRequest(http.MethodPost, "/internal/v1/pod-action-evidence/upload-intents", `{}`, actionGenericToken, AuthorizedPODEvidenceCaller, ""))
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
}

func TestPODActionReadStoreOutageIsNotEmptyList(t *testing.T) {
	store := authStore{listErr: apperrors.Internal("repository unavailable", nil)}
	rec := httptest.NewRecorder()
	actionRouter(store).ServeHTTP(rec, actionRequest(http.MethodGet, "/internal/v1/pod-action-evidence/"+uuid.NewString()+"/"+uuid.NewString()+"/"+uuid.NewString(), "", actionDedicatedToken, AuthorizedPODEvidenceCaller, ""))
	body := rec.Body.String()
	if rec.Code != http.StatusInternalServerError || strings.Contains(body, `"evidence"`) {
		t.Fatalf("status=%d body=%s", rec.Code, body)
	}
}
