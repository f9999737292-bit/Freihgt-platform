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

	"github.com/freight-platform/transport-order-service/internal/domain"
	apperrors "github.com/freight-platform/transport-order-service/internal/platform/errors"
	"github.com/freight-platform/transport-order-service/internal/service"
)

type cargoProfileStore struct{ cargo *domain.Cargo }

func (s cargoProfileStore) Create(context.Context, domain.CreateCargoInput) (*domain.Cargo, error) {
	return nil, apperrors.NotFound("unused")
}
func (s cargoProfileStore) GetByIDAndTenant(_ context.Context, id, tenantID uuid.UUID) (*domain.Cargo, error) {
	if s.cargo == nil || id != s.cargo.ID || tenantID != s.cargo.TenantID {
		return nil, apperrors.NotFound("cargo not found")
	}
	return s.cargo, nil
}
func (s cargoProfileStore) ExistsInTenant(context.Context, uuid.UUID, uuid.UUID) (bool, error) {
	return false, nil
}

func TestNLO03C_CargoPlanningProfile(t *testing.T) {
	hazard := "3"
	cargo := &domain.Cargo{
		ID: uuid.New(), TenantID: uuid.New(), Version: 7,
		Items: []domain.CargoItem{{Name: "item", HazardClass: &hazard}},
	}
	payload := cargoPlanningProfilePayload(cargo)
	t.Run("NLO03C_025_CARGO_PLANNING_PROFILE_RETURNS_VERSION", func(t *testing.T) {
		if payload["version"] != 7 {
			t.Fatal(payload["version"])
		}
	})
	t.Run("NLO03C_028_NULL_CARGO_FACTS_REMAIN_NULL", func(t *testing.T) {
		raw, err := json.Marshal(payload)
		if err != nil {
			t.Fatal(err)
		}
		var decoded map[string]any
		if err := json.Unmarshal(raw, &decoded); err != nil {
			t.Fatal(err)
		}
		for _, key := range []string{"weight_kg", "volume_m3", "pallet_count", "linear_meters", "stackable", "fragile"} {
			if _, present := decoded[key]; !present || decoded[key] != nil {
				t.Fatalf("%s = %#v", key, decoded[key])
			}
		}
	})
	t.Run("NLO03C_029_HAZARD_CLASSES_PRESERVED", func(t *testing.T) {
		classes, _ := payload["hazard_classes"].([]string)
		if len(classes) != 1 || classes[0] != "3" {
			t.Fatalf("%#v", payload["hazard_classes"])
		}
	})

	svc := service.NewTransportOrderService(nil, cargoProfileStore{cargo: cargo}, nil, nil)
	handler := NewHandler(svc)
	t.Run("NLO03C_026_CARGO_PROFILE_TENANT_SCOPED", func(t *testing.T) {
		rec := serveProfile(handler, cargo.TenantID, cargo.ID)
		if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"version":7`) {
			t.Fatalf("%d %s", rec.Code, rec.Body.String())
		}
	})
	t.Run("NLO03C_027_FOREIGN_CARGO_PROFILE_NOT_FOUND", func(t *testing.T) {
		rec := serveProfile(handler, uuid.New(), cargo.ID)
		if rec.Code != http.StatusNotFound {
			t.Fatalf("%d %s", rec.Code, rec.Body.String())
		}
	})
}

func serveProfile(handler *Handler, tenant, id uuid.UUID) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodGet, "/internal/v1/cargoes/"+id.String()+"/planning-profile", nil)
	req.Header.Set("X-Tenant-ID", tenant.String())
	route := chi.NewRouteContext()
	route.URLParams.Add("id", id.String())
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, route))
	rec := httptest.NewRecorder()
	handler.GetCargoPlanningProfile(rec, req)
	return rec
}
