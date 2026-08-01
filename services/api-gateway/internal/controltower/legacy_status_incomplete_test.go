package controltower

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/freight-platform/api-gateway/internal/controltower/legacyaggregate"
	"github.com/freight-platform/api-gateway/internal/controltowerreadmodel"
)

func TestResolveLegacyStatusInputIncompleteAggregateFallsBackPageLimited(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{
			"totalShipments": 3,
			"countedShipments": 2,
			"byStatus": {"IN_TRANSIT": 2},
			"complete": false,
			"warnings": ["UNKNOWN_SHIPMENT_STATUS"]
		}`))
	}))
	defer server.Close()

	svc := newLegacyStatusTestService(t, server.URL, controltowerreadmodel.ModeDisabled)
	input := svc.resolveLegacyStatusInput(context.Background(), testLegacyRequestContext(), sampleShipments(), 100)

	if input.FullAggregateAvailable {
		t.Fatal("incomplete aggregate must not be authoritative")
	}
	if !input.FullAggregateIncomplete {
		t.Fatal("expected incomplete aggregate marker")
	}
	if !input.LimitedDataset {
		t.Fatal("expected page-limited fallback")
	}
	if input.TotalShipments != 100 || input.CountedShipments != 2 {
		t.Fatalf("expected page fallback counts, got total=%d counted=%d", input.TotalShipments, input.CountedShipments)
	}
}

func TestResolveLegacyStatusInputIncompleteAggregateShadowComparisonBlocked(t *testing.T) {
	legacy := controltowerreadmodel.LegacyStatusInput{
		TotalShipments:          100,
		CountedShipments:        2,
		ByStatus:                map[string]int64{"IN_TRANSIT": 2},
		LimitedDataset:          true,
		FullAggregateAvailable:  false,
		FullAggregateIncomplete: true,
	}
	out := controltowerreadmodel.Merge(controltowerreadmodel.MergeInput{
		Mode:      controltowerreadmodel.ModeShadow,
		Legacy:    legacy,
		ReadModel: &controltowerreadmodel.RemoteStatusSummary{TotalShipments: 100, ByStatus: map[string]int64{"IN_TRANSIT": 100}},
	})
	if out.Comparison != controltowerreadmodel.ComparisonLegacyFullAggregateIncomplete {
		t.Fatalf("comparison=%q want LEGACY_FULL_AGGREGATE_INCOMPLETE", out.Comparison)
	}
	if out.StatusSummary == nil || !out.StatusSummary.LimitedDataset {
		t.Fatalf("expected page-limited legacy response, got %+v", out.StatusSummary)
	}
}

func TestLegacyAggregateIncompleteReasonBounded(t *testing.T) {
	if legacyaggregate.ReasonIncomplete != "FULL_LEGACY_AGGREGATE_INCOMPLETE" {
		t.Fatalf("reason=%q", legacyaggregate.ReasonIncomplete)
	}
}
