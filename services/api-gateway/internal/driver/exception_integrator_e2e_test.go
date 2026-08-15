package driver

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/freight-platform/api-gateway/internal/controltowerreadmodel"
)

func TestExceptionIntegrator_E2EAdapterFlow(t *testing.T) {
	var ensureCalled, automationCalled bool
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/internal/v1/control-tower/critical-events/workflows/ensure":
			ensureCalled = true
			_ = json.NewEncoder(w).Encode(map[string]any{"createdEventIds": []string{"a1b2c3d4e5f67890abcdef1234567890"}})
		case "/internal/v1/control-tower/automation/evaluate":
			automationCalled = true
			_, _ = w.Write([]byte(`{"matches":[]}`))
		default:
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
	}))
	defer srv.Close()

	client := controltowerreadmodel.NewClient(srv.Client(), controltowerreadmodel.Config{
		BaseURL: srv.URL,
		Mode:    controltowerreadmodel.ModePrimary,
		Timeout: time.Second,
	}, controltowerreadmodel.NewMetrics())
	integrator := NewExceptionIntegrator(client, true, time.Second)
	err := integrator.Integrate(t.Context(), RequestContext{
		TenantID: "tenant-1", UserID: "user-1", RequestID: "req-1",
	}, ExceptionIntegrationInput{
		ExceptionID: "a1b2c3d4e5f67890abcdef1234567890",
		ShipmentID:  "ship-1",
		Category:    "VEHICLE_BREAKDOWN",
		OccurredAt:  time.Now().UTC(),
	})
	if err != nil {
		t.Fatalf("integrate failed: %v", err)
	}
	if !ensureCalled || !automationCalled {
		t.Fatalf("ensure=%v automation=%v", ensureCalled, automationCalled)
	}
}
