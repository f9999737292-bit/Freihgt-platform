package controltower

import (
	"strings"
	"testing"
	"time"

	"github.com/freight-platform/api-gateway/internal/config"
	"github.com/freight-platform/api-gateway/internal/controltowerreadmodel"
	"github.com/freight-platform/api-gateway/internal/platform/logger"
	"github.com/prometheus/client_golang/prometheus"
)

func TestControlTowerMetricsRegisteredOnHandlerInit(t *testing.T) {
	cfg := config.Config{
		ServiceName: "api-gateway",
		Environment: "test",
		HTTPPort:    8080,
	}
	cfg.Services.Shipment = "http://127.0.0.1:8085"
	cfg.ControlTower.ReadModel = controltowerreadmodel.Config{
		Mode:    controltowerreadmodel.ModeShadow,
		BaseURL: "http://127.0.0.1:8089",
		Timeout: 800 * time.Millisecond,
	}

	log := logger.New("api-gateway", "debug", "test")
	_ = NewHandler(log, cfg)

	required := []string{
		"control_tower_read_model_shadow_comparison_total",
		"control_tower_read_model_requests_total",
		"control_tower_legacy_status_aggregate_requests_total",
	}
	mfs, err := prometheus.DefaultGatherer.Gather()
	if err != nil {
		t.Fatalf("gather: %v", err)
	}
	names := make(map[string]struct{}, len(mfs))
	for _, mf := range mfs {
		if mf.Name != nil {
			names[*mf.Name] = struct{}{}
		}
	}
	for _, name := range required {
		if _, ok := names[name]; !ok {
			var sample []string
			for n := range names {
				if strings.Contains(n, "control_tower") {
					sample = append(sample, n)
				}
			}
			t.Fatalf("missing metric %q; control_tower metrics: %v", name, sample)
		}
	}
}
