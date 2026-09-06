//go:build integration

package scoringv3

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/freight-platform/rfx-service/internal/http/handlers"
)

func TestScoreModelTenantQuerySpoofDenied(t *testing.T) {
	env := setupTestEnv(t)
	fix := seedBuyerFixture(t, env)
	sf := seedScoringFixture(t, env, fix)
	scoreHandler := handlers.NewScoreHandler(env.scoreModelSvc, env.scoringSvc, env.rfxSvc)
	r := chi.NewRouter()
	r.Get("/v1/rfx-events/{id}/score-model", scoreHandler.GetScoreModel)

	req := httptest.NewRequest(http.MethodGet, "/v1/rfx-events/"+sf.Event.ID.String()+"/score-model?tenant_id="+uuid.NewString(), nil)
	req.Header.Set("X-Tenant-ID", fix.TenantID.String())
	req.Header.Set("X-User-ID", fix.BuyerA.UserID.String())
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected 403 for tenant_id query spoof, got %d body=%s", rec.Code, rec.Body.String())
	}
}
