package handlers

import (
	"context"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	apperrors "github.com/freight-platform/network-optimizer-service/internal/platform/errors"
	"github.com/freight-platform/network-optimizer-service/internal/repository"
	"github.com/freight-platform/network-optimizer-service/internal/service"
)

func TestNLO04BHandlerRejectsCallerStateAndMissingKey(t *testing.T) {
	svc := service.New(repository.NewMemory(), nil)
	h := New(slog.New(slog.DiscardHandler), svc)
	shipment := uuid.New()
	load := uuid.New()
	body := `{"planning_mode":"CURRENT_TRIP","shipment_id":"` + shipment.String() + `","candidate_load_ids":["` + load.String() + `"],"start":{"latitude":1}}`
	req := httptest.NewRequest(http.MethodPost, "/v1/network/route-plans/evaluate", strings.NewReader(body))
	req.Header.Set("X-Tenant-ID", uuid.NewString())
	req.Header.Set("X-User-ID", uuid.NewString())
	req.Header.Set("Idempotency-Key", "caller-start")
	rec := httptest.NewRecorder()
	h.EvaluateRoutePlan(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("caller start %d %s", rec.Code, rec.Body.String())
	}
	missing := httptest.NewRequest(http.MethodPost, "/v1/network/route-plans/evaluate", strings.NewReader(
		`{"planning_mode":"CURRENT_TRIP","shipment_id":"`+shipment.String()+`","candidate_load_ids":["`+load.String()+`"]}`,
	))
	missing.Header.Set("X-Tenant-ID", uuid.NewString())
	missing.Header.Set("X-User-ID", uuid.NewString())
	rec = httptest.NewRecorder()
	h.EvaluateRoutePlan(rec, missing)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("missing key %d", rec.Code)
	}
	planID := uuid.NewString()
	accept := httptest.NewRequest(http.MethodPost, "/v1/network/route-plans/"+planID+"/accept", strings.NewReader(`{"version":1,"shipment_status":"IN_TRANSIT"}`))
	route := chi.NewRouteContext()
	route.URLParams.Add("id", planID)
	accept = accept.WithContext(context.WithValue(accept.Context(), chi.RouteCtxKey, route))
	accept.Header.Set("X-Tenant-ID", uuid.NewString())
	accept.Header.Set("X-User-ID", uuid.NewString())
	accept.Header.Set("Idempotency-Key", "caller-status")
	rec = httptest.NewRecorder()
	h.AcceptRoutePlan(rec, accept)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("caller status %d %s", rec.Code, rec.Body.String())
	}
	blank := httptest.NewRequest(http.MethodPost, "/v1/network/route-plans/"+planID+"/activate", strings.NewReader(`{"version":1}`))
	blankRoute := chi.NewRouteContext()
	blankRoute.URLParams.Add("id", planID)
	blank = blank.WithContext(context.WithValue(blank.Context(), chi.RouteCtxKey, blankRoute))
	blank.Header.Set("X-Tenant-ID", uuid.NewString())
	blank.Header.Set("X-User-ID", uuid.NewString())
	blank.Header.Set("Idempotency-Key", "   ")
	rec = httptest.NewRecorder()
	h.ActivateRoutePlan(rec, blank)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("blank key %d %s", rec.Code, rec.Body.String())
	}
}

func TestNLO04CHandlerMapsDomainValidationTo422(t *testing.T) {
	h := New(slog.New(slog.DiscardHandler), service.New(repository.NewMemory(), nil))
	for _, reason := range []string{"ACTIVATION_NOT_ACCEPTED", "SERVICE_DURATION_UNKNOWN", "SHIPMENT_STATUS_NOT_ELIGIBLE"} {
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPost, "/v1/network/route-plans/"+uuid.NewString()+"/activate", nil)
		h.finishRoutePlan(rec, req, "route_plan_activate", apperrors.Validation("route plan cannot be activated", map[string]any{"reason": reason}))
		if rec.Code != http.StatusUnprocessableEntity || !strings.Contains(rec.Body.String(), reason) {
			t.Fatalf("%s -> %d %s", reason, rec.Code, rec.Body.String())
		}
	}
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/v1/network/route-plans/"+uuid.NewString()+"/accept", nil)
	h.finishRoutePlan(rec, req, "route_plan_accept", apperrors.Conflict("route plan is stale", map[string]any{"reason": "PLAN_STALE"}))
	if rec.Code != http.StatusConflict || !strings.Contains(rec.Body.String(), "PLAN_STALE") {
		t.Fatalf("stale -> %d %s", rec.Code, rec.Body.String())
	}
}

func TestNLO04DI2ClientDurationDenied(t *testing.T) {
	h := New(slog.New(slog.DiscardHandler), service.New(repository.NewMemory(), nil))
	shipment := uuid.New()
	load := uuid.New()
	for _, key := range []string{"service_duration", "service_duration_seconds", "pickup_duration_seconds", "delivery_duration_seconds"} {
		body := `{"planning_mode":"CURRENT_TRIP","shipment_id":"` + shipment.String() + `","candidate_load_ids":["` + load.String() + `"],"` + key + `":60}`
		req := httptest.NewRequest(http.MethodPost, "/v1/network/route-plans/evaluate", strings.NewReader(body))
		req.Header.Set("X-Tenant-ID", uuid.NewString())
		req.Header.Set("X-User-ID", uuid.NewString())
		req.Header.Set("Idempotency-Key", "duration-"+key)
		rec := httptest.NewRecorder()
		h.EvaluateRoutePlan(rec, req)
		if rec.Code != http.StatusBadRequest || !strings.Contains(rec.Body.String(), "service duration") {
			t.Fatalf("%s -> %d %s", key, rec.Code, rec.Body.String())
		}
	}
}
