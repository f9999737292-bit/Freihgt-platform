package handlers

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/freight-platform/network-optimizer-service/internal/domain"
	"github.com/freight-platform/network-optimizer-service/internal/repository"
	"github.com/freight-platform/network-optimizer-service/internal/service"
	"github.com/freight-platform/shared-go/lowcode"
)

func TestNLO03B_011_SEARCHER_CANNOT_OVERRIDE_OPT_IN(t *testing.T) {
	store := repository.NewMemory()
	svc := service.New(store, nil)
	carrier := uuid.New()
	shipper := uuid.New()
	now := time.Date(2026, 9, 26, 8, 0, 0, 0, time.UTC)
	origin, dest := uuid.New(), uuid.New()
	cap := domain.Capacity{
		ID: uuid.New(), OwnerTenantID: carrier, AvailableFrom: now, AvailableUntil: now.Add(time.Hour),
		Source: domain.SourceManual, VisibilityScope: domain.CapVisPrivate, Status: domain.CapacityAvailable, Version: 1,
		PayloadRemainingKg: floatPtr(1000),
	}
	win := domain.TimeWindow{Start: &now, End: timePtr(now.Add(time.Hour))}
	insert := func(opt bool) {
		t.Helper()
		load := domain.LoadOpportunity{
			ID: uuid.New(), OwnerTenantID: shipper, SourceType: domain.SourceTransportOrder, SourceID: uuid.New(),
			Pickup: domain.Place{LocationID: &origin, City: "A", CountryCode: "RU"}, Delivery: domain.Place{LocationID: &dest, City: "B", CountryCode: "RU"},
			PickupWindow: win, DeliveryWindow: win, VisibilityScope: domain.VisMarketplace, Status: domain.LoadPublished,
			Version: 1, WeightKg: floatPtr(10), CreatedAt: now, UpdatedAt: now,
		}
		if err := store.Within(context.Background(), func(tx repository.Tx) error {
			return tx.InsertLoad(context.Background(), load)
		}); err != nil {
			t.Fatal(err)
		}
		_ = opt
	}
	if err := store.Within(context.Background(), func(tx repository.Tx) error {
		return tx.InsertCapacity(context.Background(), cap)
	}); err != nil {
		t.Fatal(err)
	}
	insert(false)
	insert(false)
	h := New(nil, svc)
	req := httptest.NewRequest(http.MethodPost, "/v1/network/consolidation/search", strings.NewReader(`{"capacity_id":"`+cap.ID.String()+`","pattern":"SAME_ORIGIN_SAME_DESTINATION","consolidation_allowed":true,"cross_shipper_consolidation_allowed":true}`))
	req.Header.Set(lowcode.HeaderTenantID, carrier.String())
	req.Header.Set(lowcode.HeaderUserID, uuid.NewString())
	rec := httptest.NewRecorder()
	h.SearchConsolidation(rec, req)
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"evaluated_pair_count":0`) || strings.Contains(rec.Body.String(), `"status":"FEASIBLE"`) {
		t.Fatalf("status %d body %s", rec.Code, rec.Body.String())
	}
}

func floatPtr(v float64) *float64    { return &v }
func timePtr(v time.Time) *time.Time { return &v }
