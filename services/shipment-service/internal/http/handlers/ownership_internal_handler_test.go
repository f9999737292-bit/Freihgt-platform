package handlers

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/freight-platform/shared-go/integrationauth"
	"github.com/freight-platform/shared-go/internalauth"
	apperrors "github.com/freight-platform/shipment-service/internal/platform/errors"
)

type ownershipStub struct {
	ownerTenant uuid.UUID
	ownerID     uuid.UUID
	gotTenant   uuid.UUID
	gotID       uuid.UUID
}

func (s *ownershipStub) ConfirmOwnership(_ context.Context, tenantID, id uuid.UUID) error {
	s.gotTenant = tenantID
	s.gotID = id
	if tenantID != s.ownerTenant || id != s.ownerID {
		return apperrors.NotFound("shipment not found")
	}
	return nil
}

func ownershipRouter(stub *ownershipStub, token string) http.Handler {
	handler := NewOwnershipInternalHandler(stub)
	auth := internalauth.Config{Token: token}
	r := chi.NewRouter()
	r.Route("/internal/v1/shipments", func(r chi.Router) {
		r.With(auth.Middleware).Get("/{shipmentId}/ownership", handler.GetShipment)
	})
	return r
}

func TestShipmentOwnershipProbe(t *testing.T) {
	tenant := uuid.New()
	other := uuid.New()
	id := uuid.New()
	token := "test-internal-token"
	stub := &ownershipStub{ownerTenant: tenant, ownerID: id}
	router := ownershipRouter(stub, token)
	path := "/internal/v1/shipments/" + id.String() + "/ownership"

	noAuth := httptest.NewRequest(http.MethodGet, path, nil)
	noAuth.Header.Set("X-Tenant-ID", tenant.String())
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, noAuth)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("BNO23 status=%d", rec.Code)
	}

	bad := httptest.NewRequest(http.MethodGet, path, nil)
	bad.Header.Set(internalauth.HeaderName, "wrong-token")
	bad.Header.Set("X-Tenant-ID", tenant.String())
	rec = httptest.NewRecorder()
	router.ServeHTTP(rec, bad)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("BNO24 status=%d", rec.Code)
	}

	foreign := httptest.NewRequest(http.MethodGet, path, nil)
	foreign.Header.Set(internalauth.HeaderName, token)
	foreign.Header.Set("X-Tenant-ID", other.String())
	missing := httptest.NewRequest(http.MethodGet, "/internal/v1/shipments/"+uuid.NewString()+"/ownership", nil)
	missing.Header.Set(internalauth.HeaderName, token)
	missing.Header.Set("X-Tenant-ID", tenant.String())
	foreignRec := httptest.NewRecorder()
	missingRec := httptest.NewRecorder()
	router.ServeHTTP(foreignRec, foreign)
	router.ServeHTTP(missingRec, missing)
	if foreignRec.Code != http.StatusNotFound || missingRec.Code != http.StatusNotFound || foreignRec.Body.String() != missingRec.Body.String() {
		t.Fatalf("BNO25 foreign=%d %s missing=%d %s", foreignRec.Code, foreignRec.Body.String(), missingRec.Code, missingRec.Body.String())
	}

	forged := httptest.NewRequest(http.MethodGet, path, nil)
	forged.Header.Set(integrationauth.HeaderIntegrationPrincipalID, uuid.NewString())
	forged.Header.Set(integrationauth.HeaderIntegrationScopes, "rfx:draft:read")
	forged.Header.Set(integrationauth.HeaderAuthScheme, integrationauth.AuthSchemeOAuth)
	forged.Header.Set(integrationauth.HeaderActorKind, integrationauth.ActorKindIntegration)
	forged.Header.Set("X-Tenant-ID", tenant.String())
	forgedRec := httptest.NewRecorder()
	router.ServeHTTP(forgedRec, forged)
	if forgedRec.Code != http.StatusUnauthorized {
		t.Fatalf("BNO26 forged service identity status=%d", forgedRec.Code)
	}

	okReq := httptest.NewRequest(http.MethodGet, path, nil)
	okReq.Header.Set(internalauth.HeaderName, token)
	okReq.Header.Set("X-Tenant-ID", tenant.String())
	okRec := httptest.NewRecorder()
	router.ServeHTTP(okRec, okReq)
	if okRec.Code != http.StatusOK {
		t.Fatalf("BNO27 status=%d body=%s", okRec.Code, okRec.Body.String())
	}
	var body map[string]any
	if err := json.Unmarshal(okRec.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if body["id"] != id.String() || body["tenant_id"] != tenant.String() || len(body) != 2 {
		t.Fatalf("BNO27 body=%s", okRec.Body.String())
	}
	if stub.gotTenant != tenant || stub.gotID != id {
		t.Fatalf("predicate tenant=%s id=%s", stub.gotTenant, stub.gotID)
	}
}
