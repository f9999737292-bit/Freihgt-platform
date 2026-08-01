package controltowerreadmodel

import (
	"strings"
	"testing"

	"github.com/prometheus/client_golang/prometheus"
)

func TestMetricsExposeAfterNewMetrics(t *testing.T) {
	_ = NewMetrics()
	mfs, err := prometheus.DefaultGatherer.Gather()
	if err != nil {
		t.Fatalf("gather: %v", err)
	}
	found := false
	for _, mf := range mfs {
		if mf.Name != nil && strings.HasPrefix(*mf.Name, "control_tower_read_model_") {
			found = true
			break
		}
	}
	if !found {
		t.Fatal("expected control_tower_read_model metrics in default registry")
	}
}
