package handlers

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/freight-platform/shared-go/internalauth"
	"github.com/freight-platform/transport-order-service/internal/domain"
	apperrors "github.com/freight-platform/transport-order-service/internal/platform/errors"
)

type locationStub struct {
	tenant uuid.UUID
	item   domain.Location
}

func (s locationStub) GetLocation(_ context.Context, tenantID, id uuid.UUID) (*domain.Location, error) {
	if tenantID != s.tenant || id != s.item.ID {
		return nil, apperrors.NotFound("location not found")
	}
	item := s.item
	return &item, nil
}

func TestInternalLocationProjectionHidesAddress(t *testing.T) {
	tenant := uuid.New()
	other := uuid.New()
	id := uuid.New()
	region, city, address := "Sverdlovsk", "Ekaterinburg", "Secret Dock 9"
	lat, lon := 56.8, 60.6
	stub := locationStub{tenant: tenant, item: domain.Location{
		ID: id, TenantID: tenant, Name: "Private Warehouse", CountryCode: "RU",
		Region: &region, City: &city, AddressLine: &address, Lat: &lat, Lon: &lon,
		Timezone: "Asia/Yekaterinburg", Status: "ACTIVE", Version: 3,
	}}
	auth := internalauth.Config{Token: "internal-token", Environment: "test"}
	router := chi.NewRouter()
	router.With(auth.Middleware).Get("/internal/v1/locations/{locationId}", NewLocationInternalHandler(stub).GetProjection)

	req := httptest.NewRequest(http.MethodGet, "/internal/v1/locations/"+id.String(), nil)
	req.Header.Set(internalauth.HeaderName, "internal-token")
	req.Header.Set("X-Tenant-ID", tenant.String())
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d %s", rec.Code, rec.Body.String())
	}
	var doc map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &doc); err != nil {
		t.Fatal(err)
	}
	if doc["city"] != "Ekaterinburg" || doc["country_code"] != "RU" || doc["name"] != nil || doc["address_line"] != nil {
		t.Fatalf("projection %s", rec.Body.String())
	}
	foreign := httptest.NewRequest(http.MethodGet, "/internal/v1/locations/"+id.String(), nil)
	foreign.Header.Set(internalauth.HeaderName, "internal-token")
	foreign.Header.Set("X-Tenant-ID", other.String())
	foreignRec := httptest.NewRecorder()
	router.ServeHTTP(foreignRec, foreign)
	if foreignRec.Code != http.StatusNotFound {
		t.Fatalf("foreign %d", foreignRec.Code)
	}
}
