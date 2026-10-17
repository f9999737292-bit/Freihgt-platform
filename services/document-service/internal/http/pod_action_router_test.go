package http

import (
	"context"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/google/uuid"

	"github.com/freight-platform/document-service/internal/domain"
	apperrors "github.com/freight-platform/document-service/internal/platform/errors"
	"github.com/freight-platform/document-service/internal/service"
	"github.com/freight-platform/shared-go/internalauth"
)

func TestActionEvidenceRouteRequiresInternalAuth(t *testing.T) {
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	svc := service.NewPODActionEvidenceService(routerActionStore{}, nil, 1024, service.ActionObjectPlacement{StorageProvider: "local"})
	router := NewRouter(log, nil, nil, nil, nil, nil, nil, nil, svc, internalauth.Config{Token: "dedicated-shipment-token", Environment: "test"})
	req := httptest.NewRequest(http.MethodGet, "/internal/v1/pod-action-evidence/"+uuid.NewString()+"/"+uuid.NewString()+"/"+uuid.NewString(), nil)
	req.Header.Set("X-Tenant-ID", uuid.NewString())
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("tenant header alone status=%d body=%s", rec.Code, rec.Body.String())
	}
}

func TestActionEvidenceRouteFailClosedWithoutDedicatedToken(t *testing.T) {
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	svc := service.NewPODActionEvidenceService(routerActionStore{}, nil, 1024, service.ActionObjectPlacement{StorageProvider: "local"})
	router := NewRouter(log, nil, nil, nil, nil, nil, nil, nil, svc, internalauth.Config{Environment: "test"})
	req := httptest.NewRequest(http.MethodPost, "/internal/v1/pod-action-evidence/upload-intents", nil)
	req.Header.Set(internalauth.HeaderName, "generic-internal-service-token")
	req.Header.Set("X-Internal-Service-Name", "shipment-service")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("empty dedicated token status=%d body=%s", rec.Code, rec.Body.String())
	}
}

func TestActionEvidenceRouteBindsDedicatedCredentialToCallerName(t *testing.T) {
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	svc := service.NewPODActionEvidenceService(routerActionStore{}, nil, 1024, service.ActionObjectPlacement{StorageProvider: "local"})
	router := NewRouter(log, nil, nil, nil, nil, nil, nil, nil, svc, internalauth.Config{Token: "dedicated-shipment-token", Environment: "test"})

	spoof := httptest.NewRequest(http.MethodPost, "/internal/v1/pod-action-evidence/upload-intents", nil)
	spoof.Header.Set(internalauth.HeaderName, "generic-internal-service-token")
	spoof.Header.Set("X-Internal-Service-Name", "shipment-service")
	spoofRec := httptest.NewRecorder()
	router.ServeHTTP(spoofRec, spoof)
	if spoofRec.Code != http.StatusUnauthorized {
		t.Fatalf("generic token status=%d body=%s", spoofRec.Code, spoofRec.Body.String())
	}

	wrong := httptest.NewRequest(http.MethodPost, "/internal/v1/pod-action-evidence/upload-intents", nil)
	wrong.Header.Set(internalauth.HeaderName, "dedicated-shipment-token")
	wrong.Header.Set("X-Internal-Service-Name", "api-gateway")
	wrongRec := httptest.NewRecorder()
	router.ServeHTTP(wrongRec, wrong)
	if wrongRec.Code != http.StatusForbidden {
		t.Fatalf("wrong caller status=%d body=%s", wrongRec.Code, wrongRec.Body.String())
	}
}

type routerActionStore struct{}

func (routerActionStore) CompanyExists(context.Context, uuid.UUID, uuid.UUID) (bool, error) {
	return false, nil
}
func (routerActionStore) FindIntentByIdempotency(context.Context, uuid.UUID, string) (*domain.PODActionIntent, error) {
	return nil, nil
}
func (routerActionStore) RotateUploadToken(context.Context, uuid.UUID, string) error { return nil }
func (routerActionStore) LoadIntent(context.Context, uuid.UUID) (*domain.PODActionIntent, error) {
	return nil, apperrors.NotFound("upload intent not found")
}
func (routerActionStore) InsertIntent(context.Context, domain.PODActionIntent) (domain.PODActionIntent, error) {
	return domain.PODActionIntent{}, nil
}
func (routerActionStore) MarkIntentUploaded(context.Context, uuid.UUID, int64) error { return nil }
func (routerActionStore) EvidenceByIntent(context.Context, uuid.UUID) (*domain.PODActionEvidence, error) {
	return nil, apperrors.NotFound("upload intent not found")
}
func (routerActionStore) CompleteIntent(context.Context, uuid.UUID, string, string, *string) (*domain.PODActionEvidence, error) {
	return nil, apperrors.NotFound("upload intent not found")
}
func (routerActionStore) ListPresentEvidence(context.Context, uuid.UUID, uuid.UUID, uuid.UUID) ([]domain.PODActionEvidence, error) {
	return nil, nil
}
