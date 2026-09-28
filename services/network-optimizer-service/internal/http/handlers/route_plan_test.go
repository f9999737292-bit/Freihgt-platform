package handlers

import (
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/google/uuid"

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
}
