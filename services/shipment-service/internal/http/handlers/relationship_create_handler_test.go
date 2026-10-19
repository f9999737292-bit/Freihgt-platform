package handlers

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/freight-platform/shipment-service/internal/http/userauth"
	"github.com/freight-platform/shipment-service/internal/platform/security"
	"github.com/freight-platform/shipment-service/internal/service"
)

func TestRelationshipCreateRouteRequiresJWTAndReturnsTokenOnce(t *testing.T) {
	shipmentID := uuid.New()
	userID := uuid.New()
	tenantID := uuid.New()
	stub := &stubCreator{result: service.RelationshipCreateResult{
		RequestID: uuid.New(), Status: "PENDING", AcceptanceToken: "once", Created: true,
	}}
	token := signToken(t, "test-secret", userID, tenantID)
	path := "/v1/shipments/" + shipmentID.String() + "/carrier-relationship-requests"

	missing := httptest.NewRequest(http.MethodPost, path, nil)
	missing.Header.Set("Idempotency-Key", "key-1")
	rec := httptest.NewRecorder()
	routeWith(stub, "test-secret").ServeHTTP(rec, missing)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("missing jwt %d", rec.Code)
	}

	trusted := httptest.NewRequest(http.MethodPost, path+"?tenant_id="+uuid.NewString(), strings.NewReader(`{"fingerprint":"client"}`))
	trusted.Header.Set("Authorization", "Bearer "+token)
	trusted.Header.Set("Idempotency-Key", "key-1")
	trusted.Header.Set("X-Tenant-ID", uuid.NewString())
	rec = httptest.NewRecorder()
	routeWith(stub, "test-secret").ServeHTTP(rec, trusted)
	if rec.Code != http.StatusBadRequest || stub.calls != 0 {
		t.Fatalf("trusted client fields %d calls=%d body=%s", rec.Code, stub.calls, rec.Body.String())
	}

	ok := httptest.NewRequest(http.MethodPost, path, strings.NewReader(`{}`))
	ok.Header.Set("Authorization", "Bearer "+token)
	ok.Header.Set("Idempotency-Key", "key-1")
	ok.Header.Set("X-Tenant-ID", uuid.NewString())
	ok.Header.Set("X-Internal-Service-Name", "rfx-service")
	rec = httptest.NewRecorder()
	routeWith(stub, "test-secret").ServeHTTP(rec, ok)
	if rec.Code != http.StatusCreated || !strings.Contains(rec.Body.String(), `"acceptance_token":"once"`) || rec.Header().Get("Cache-Control") != "no-store" {
		t.Fatalf("create %d cache=%s %s", rec.Code, rec.Header().Get("Cache-Control"), rec.Body.String())
	}
	if stub.actor.TenantID != tenantID || stub.actor.UserID != userID || stub.shipmentID != shipmentID || stub.key != "key-1" {
		t.Fatalf("actor %+v shipment %s key %s", stub.actor, stub.shipmentID, stub.key)
	}

	stub.result.Created = false
	stub.result.AcceptanceToken = "hidden"
	replay := httptest.NewRequest(http.MethodPost, path, nil)
	replay.Header.Set("Authorization", "Bearer "+token)
	replay.Header.Set("Idempotency-Key", "key-1")
	rec = httptest.NewRecorder()
	routeWith(stub, "test-secret").ServeHTTP(rec, replay)
	if rec.Code != http.StatusOK || strings.Contains(rec.Body.String(), "acceptance_token") || strings.Contains(rec.Body.String(), "hidden") || rec.Header().Get("Cache-Control") == "no-store" {
		t.Fatalf("replay %d cache=%s %s", rec.Code, rec.Header().Get("Cache-Control"), rec.Body.String())
	}

	closed := httptest.NewRequest(http.MethodPost, path, nil)
	closed.Header.Set("Authorization", "Bearer "+token)
	closed.Header.Set("Idempotency-Key", "key-1")
	rec = httptest.NewRecorder()
	routeWith(stub, "").ServeHTTP(rec, closed)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("empty secret %d", rec.Code)
	}
}

type stubCreator struct {
	result     service.RelationshipCreateResult
	actor      service.RelationshipActor
	shipmentID uuid.UUID
	key        string
	calls      int
}

func (s *stubCreator) Status(context.Context, service.RelationshipActor, uuid.UUID) ([]service.RelationshipStatusItem, error) {
	return []service.RelationshipStatusItem{}, nil
}

func (s *stubCreator) Cancel(context.Context, service.RelationshipActor, uuid.UUID, uuid.UUID, string, string) (service.RelationshipCancelResult, error) {
	return service.RelationshipCancelResult{}, nil
}

func (s *stubCreator) Create(_ context.Context, actor service.RelationshipActor, shipmentID uuid.UUID, key string) (service.RelationshipCreateResult, error) {
	s.calls++
	s.actor = actor
	s.shipmentID = shipmentID
	s.key = key
	return s.result, nil
}

func routeWith(stub *stubCreator, secret string) http.Handler {
	return userauth.RequireJWT(secret)(http.HandlerFunc(NewRelationshipCreateHandler(stub).Create))
}

func signToken(t *testing.T, secret string, userID, tenantID uuid.UUID) string {
	t.Helper()
	token, err := security.SignHS256(secret, userID, tenantID, time.Now().UTC().Add(time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	return token
}
