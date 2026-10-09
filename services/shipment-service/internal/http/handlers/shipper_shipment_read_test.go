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

	"github.com/freight-platform/shipment-service/internal/domain"
	apperrors "github.com/freight-platform/shipment-service/internal/platform/errors"
	"github.com/freight-platform/shipment-service/internal/service"
)

func TestShipperShipmentReadsAreCompanyScoped(t *testing.T) {
	tenantA := uuid.MustParse("11111111-1111-1111-1111-111111111111")
	tenantB := uuid.MustParse("22222222-2222-2222-2222-222222222222")
	shipperA := uuid.MustParse("aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa")
	shipperB := uuid.MustParse("bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb")
	now := time.Date(2026, 10, 9, 12, 0, 0, 0, time.UTC)
	row := func(id, tenant, shipper uuid.UUID, number string, consignee uuid.UUID, forwarder *uuid.UUID) domain.Shipment {
		return domain.Shipment{
			ID: id, TenantID: tenant, ShipperCompanyID: shipper, ConsigneeCompanyID: consignee,
			ForwarderCompanyID: forwarder, ShipmentNumber: number, Status: domain.ShipmentStatusInTransit,
			CreatedAt: now, UpdatedAt: now,
		}
	}
	ownedA := uuid.MustParse("00000000-0000-0000-0000-0000000000a1")
	ownedB := uuid.MustParse("00000000-0000-0000-0000-0000000000b1")
	otherTenant := uuid.MustParse("00000000-0000-0000-0000-0000000000c1")
	forwarderRow := uuid.MustParse("00000000-0000-0000-0000-0000000000d1")
	consigneeRow := uuid.MustParse("00000000-0000-0000-0000-0000000000e1")
	rows := []domain.Shipment{
		row(ownedA, tenantA, shipperA, "SHP-A", shipperB, nil),
		row(ownedB, tenantA, shipperB, "SHP-B", shipperA, nil),
		row(otherTenant, tenantB, shipperA, "SHP-TENANT-B", shipperB, nil),
		row(forwarderRow, tenantA, shipperB, "SHP-FORWARDER", shipperB, &shipperA),
		row(consigneeRow, tenantA, shipperB, "SHP-CONSIGNEE", shipperA, nil),
	}
	store := &tenantScopedShipmentService{
		listByShipperFn: func(_ context.Context, filter domain.ShipperShipmentListFilter) ([]domain.Shipment, int, error) {
			matched := make([]domain.Shipment, 0)
			for _, item := range rows {
				if item.TenantID != filter.TenantID || item.ShipperCompanyID != filter.ShipperCompanyID {
					continue
				}
				if filter.Status != nil && item.Status != *filter.Status {
					continue
				}
				matched = append(matched, item)
			}
			total := len(matched)
			if filter.Offset > len(matched) {
				matched = nil
			} else {
				matched = matched[filter.Offset:]
			}
			if filter.Limit < len(matched) {
				matched = matched[:filter.Limit]
			}
			return matched, total, nil
		},
		getByShipperFn: func(_ context.Context, id, tenantID, shipperCompanyID uuid.UUID) (*domain.Shipment, error) {
			for _, item := range rows {
				if item.ID == id && item.TenantID == tenantID && item.ShipperCompanyID == shipperCompanyID {
					copy := item
					return &copy, nil
				}
			}
			return nil, apperrors.NotFound("shipment not found")
		},
	}
	handler := NewShipmentHandler(service.NewShipmentService(store, nil, nil))
	router := chi.NewRouter()
	router.Get("/v1/shipper/shipments", handler.ListForShipper)
	router.Get("/v1/shipper/shipments/{id}", handler.GetForShipper)

	call := func(method, path, tenant string) *httptest.ResponseRecorder {
		req := httptest.NewRequest(method, path, nil)
		if tenant != "" {
			req.Header.Set("X-Tenant-ID", tenant)
		}
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)
		return rec
	}
	numbers := func(body string) []string {
		var payload struct {
			Items []struct {
				Number string `json:"shipment_number"`
			} `json:"items"`
		}
		if err := json.Unmarshal([]byte(body), &payload); err != nil {
			t.Fatal(err)
		}
		out := make([]string, 0, len(payload.Items))
		for _, item := range payload.Items {
			out = append(out, item.Number)
		}
		return out
	}

	t.Run("A shipper A list", func(t *testing.T) {
		rec := call(http.MethodGet, "/v1/shipper/shipments?shipper_company_id="+shipperA.String()+"&consignee_company_id="+shipperB.String(), tenantA.String())
		if rec.Code != http.StatusOK {
			t.Fatalf("%d %s", rec.Code, rec.Body.String())
		}
		got := numbers(rec.Body.String())
		if len(got) != 1 || got[0] != "SHP-A" {
			t.Fatalf("%v", got)
		}
	})
	t.Run("B shipper B list", func(t *testing.T) {
		rec := call(http.MethodGet, "/v1/shipper/shipments?shipper_company_id="+shipperB.String()+"&limit=1&offset=0", tenantA.String())
		if rec.Code != http.StatusOK {
			t.Fatalf("%d %s", rec.Code, rec.Body.String())
		}
		got := numbers(rec.Body.String())
		if len(got) != 1 || got[0] != "SHP-B" || !strings.Contains(rec.Body.String(), `"total":3`) {
			t.Fatalf("%s", rec.Body.String())
		}
	})
	t.Run("C owned detail", func(t *testing.T) {
		rec := call(http.MethodGet, "/v1/shipper/shipments/"+ownedA.String()+"?shipper_company_id="+shipperA.String(), tenantA.String())
		if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), "SHP-A") {
			t.Fatalf("%d %s", rec.Code, rec.Body.String())
		}
	})
	t.Run("D cross shipper detail", func(t *testing.T) {
		rec := call(http.MethodGet, "/v1/shipper/shipments/"+ownedB.String()+"?shipper_company_id="+shipperA.String(), tenantA.String())
		if rec.Code != http.StatusNotFound || strings.Contains(rec.Body.String(), "SHP-B") {
			t.Fatalf("%d %s", rec.Code, rec.Body.String())
		}
	})
	t.Run("E cross tenant detail", func(t *testing.T) {
		rec := call(http.MethodGet, "/v1/shipper/shipments/"+ownedA.String()+"?shipper_company_id="+shipperA.String(), tenantB.String())
		if rec.Code != http.StatusNotFound || strings.Contains(rec.Body.String(), "SHP-A") {
			t.Fatalf("%d %s", rec.Code, rec.Body.String())
		}
	})
	t.Run("F missing list company", func(t *testing.T) {
		rec := call(http.MethodGet, "/v1/shipper/shipments", tenantA.String())
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("%d %s", rec.Code, rec.Body.String())
		}
	})
	t.Run("G missing detail company", func(t *testing.T) {
		rec := call(http.MethodGet, "/v1/shipper/shipments/"+ownedA.String(), tenantA.String())
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("%d %s", rec.Code, rec.Body.String())
		}
	})
	t.Run("H malformed company", func(t *testing.T) {
		rec := call(http.MethodGet, "/v1/shipper/shipments?shipper_company_id=not-a-uuid", tenantA.String())
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("%d %s", rec.Code, rec.Body.String())
		}
	})
	t.Run("I forwarder is not shipper", func(t *testing.T) {
		rec := call(http.MethodGet, "/v1/shipper/shipments/"+forwarderRow.String()+"?shipper_company_id="+shipperA.String(), tenantA.String())
		if rec.Code != http.StatusNotFound || strings.Contains(rec.Body.String(), "SHP-FORWARDER") {
			t.Fatalf("%d %s", rec.Code, rec.Body.String())
		}
	})
	t.Run("J consignee is not shipper", func(t *testing.T) {
		rec := call(http.MethodGet, "/v1/shipper/shipments/"+consigneeRow.String()+"?shipper_company_id="+shipperA.String(), tenantA.String())
		if rec.Code != http.StatusNotFound || strings.Contains(rec.Body.String(), "SHP-CONSIGNEE") {
			t.Fatalf("%d %s", rec.Code, rec.Body.String())
		}
	})
}
