package kpi

import (
	"encoding/json"
	"math"
	"strings"
	"testing"
	"time"
)

func TestFiveMappingsAndQuality(t *testing.T) {
	now := time.Date(2026, 10, 7, 18, 0, 0, 0, time.UTC)
	snap := Snapshot{
		ShipmentTotal:             10,
		OnTimeDeliveryDenominator: 10,
		OnTimeDeliveryNumerator:   8,
		ReturnCaseCount:           2,
		RedirectCaseCount:         1,
	}
	cases := []struct {
		id            string
		measureType   string
		value         string
		completeness  string
		hasRatioParts bool
	}{
		{ShipmentsTotal, "COUNT", "10", "COMPLETE", false},
		{OnTimeDelivery, "COUNT", "8", "PARTIAL", false},
		{OnTimeDeliveryRate, "RATIO", "0.8", "PARTIAL", true},
		{ReturnCases, "COUNT", "2", "COMPLETE", false},
		{RedirectCases, "COUNT", "1", "COMPLETE", false},
	}
	for _, tt := range cases {
		t.Run(tt.id, func(t *testing.T) {
			got, err := Build(tt.id, snap, now)
			if err != nil {
				t.Fatal(err)
			}
			if got.DefinitionVersion != 1 || got.DataFreshness.Status != "UNKNOWN" || got.Completeness != tt.completeness {
				t.Fatalf("%+v", got)
			}
			if got.GeneratedAt != "2026-10-07T18:00:00Z" {
				t.Fatalf("generatedAt=%s", got.GeneratedAt)
			}
			if _, ok := any(got.DataFreshness).(interface{ SourceObservedAt() string }); ok {
				t.Fatal("freshness must not grow extra clocks")
			}
			raw, err := json.Marshal(got)
			if err != nil {
				t.Fatal(err)
			}
			if strings.Contains(string(raw), "sourceObservedAt") || strings.Contains(string(raw), "ageSeconds") || strings.Contains(string(raw), "publicationRevision") {
				t.Fatalf("unexpected fields %s", raw)
			}
			if got.Measure.Type != tt.measureType || got.Measure.Value == nil || got.Measure.Value.String() != tt.value {
				t.Fatalf("measure %+v", got.Measure)
			}
			if tt.hasRatioParts {
				if got.Measure.Numerator == nil || *got.Measure.Numerator != 8 || got.Measure.Denominator == nil || *got.Measure.Denominator != 10 {
					t.Fatalf("ratio parts %+v", got.Measure)
				}
			}
			assertFinite(t, got.Measure.Value.String())
		})
	}
}

func TestZeroDenominatorIsNullNotZeroOrOne(t *testing.T) {
	got, err := Build(OnTimeDeliveryRate, Snapshot{}, time.Unix(0, 0).UTC())
	if err != nil {
		t.Fatal(err)
	}
	if got.Measure.Value != nil || *got.Measure.Numerator != 0 || *got.Measure.Denominator != 0 {
		t.Fatalf("%+v", got.Measure)
	}
	if got.Completeness != "PARTIAL" {
		t.Fatalf("completeness=%s", got.Completeness)
	}
	raw, _ := json.Marshal(got.Measure)
	if strings.Contains(string(raw), "NaN") || strings.Contains(string(raw), "Inf") {
		t.Fatal(string(raw))
	}
}

func TestRatioEdges(t *testing.T) {
	now := time.Unix(0, 0).UTC()
	equal, err := Build(OnTimeDeliveryRate, Snapshot{ShipmentTotal: 4, OnTimeDeliveryDenominator: 4, OnTimeDeliveryNumerator: 4}, now)
	if err != nil || equal.Measure.Value == nil || equal.Measure.Value.String() != "1" {
		t.Fatalf("%+v %v", equal.Measure, err)
	}
	zeroNumerator, err := Build(OnTimeDeliveryRate, Snapshot{ShipmentTotal: 4, OnTimeDeliveryDenominator: 4}, now)
	if err != nil || zeroNumerator.Measure.Value == nil || zeroNumerator.Measure.Value.String() != "0" {
		t.Fatalf("%+v %v", zeroNumerator.Measure, err)
	}
}

func TestUnknownKPI(t *testing.T) {
	if _, err := Build("OPS_OTIF", Snapshot{}, time.Now()); err == nil {
		t.Fatal("expected unknown kpi error")
	}
}

func TestAN03BOperationsAndCarrierKPIs(t *testing.T) {
	now := time.Date(2026, 10, 8, 12, 0, 0, 0, time.UTC)
	snap := Snapshot{
		ShipmentTotal:             10,
		OnTimePickupDenominator:   5,
		OnTimePickupNumerator:     3,
		OnTimeDeliveryDenominator: 10,
		OnTimeDeliveryNumerator:   8,
		ReturnCaseCount:           2,
		RedirectCaseCount:         1,
		Carriers: []CarrierSnapshot{
			{CarrierCompanyID: "33333333-3333-3333-3333-333333333333", OnTimePickupDenominator: 2, OnTimePickupNumerator: 1, OnTimeDeliveryDenominator: 4, OnTimeDeliveryNumerator: 2},
			{CarrierCompanyID: "22222222-2222-2222-2222-222222222222", OnTimePickupDenominator: 0, OnTimePickupNumerator: 0, OnTimeDeliveryDenominator: 1, OnTimeDeliveryNumerator: 1},
		},
	}
	t.Run("AN03B_01", func(t *testing.T) {
		got, err := Build(OnTimePickup, snap, now)
		if err != nil || got.Measure.Type != "COUNT" || got.Measure.Value == nil || got.Measure.Value.String() != "3" || got.Completeness != "PARTIAL" || got.DefinitionVersion != 1 {
			t.Fatalf("%+v %v", got, err)
		}
	})
	t.Run("AN03B_02", func(t *testing.T) {
		got, err := Build(OnTimePickupRate, snap, now)
		if err != nil || got.KPIID != OnTimePickupRate || got.Measure.Type != "RATIO" || got.Measure.Value == nil || got.Measure.Value.String() != "0.6" || *got.Measure.Numerator != 3 || *got.Measure.Denominator != 5 {
			t.Fatalf("%+v %v", got, err)
		}
	})
	t.Run("AN03B_03", func(t *testing.T) {
		got, err := Build(OnTimePickupRate, Snapshot{}, now)
		if err != nil || got.Measure.Value != nil || *got.Measure.Numerator != 0 || *got.Measure.Denominator != 0 {
			t.Fatalf("%+v %v", got, err)
		}
	})
	t.Run("AN03B_04", func(t *testing.T) {
		got, err := Build(LatePickup, snap, now)
		if err != nil || got.Measure.Value == nil || got.Measure.Value.String() != "2" {
			t.Fatalf("%+v %v", got, err)
		}
	})
	t.Run("AN03B_05", func(t *testing.T) {
		got, err := Build(LateDelivery, snap, now)
		if err != nil || got.Measure.Value == nil || got.Measure.Value.String() != "2" {
			t.Fatalf("%+v %v", got, err)
		}
	})
	t.Run("AN03B_06", func(t *testing.T) {
		if _, err := Build(LatePickup, Snapshot{OnTimePickupDenominator: 1, OnTimePickupNumerator: 2}, now); err == nil {
			t.Fatal("negative late pickup was accepted")
		}
		if _, err := Build(LateDelivery, Snapshot{OnTimeDeliveryDenominator: 1, OnTimeDeliveryNumerator: 2}, now); err == nil {
			t.Fatal("negative late delivery was accepted")
		}
	})
	t.Run("AN03B_07", func(t *testing.T) {
		got, err := BuildCarrier(CarrierPickupRate, snap, now)
		if err != nil || got.Dimension != "CARRIER" || len(got.Items) != 2 {
			t.Fatalf("%+v %v", got, err)
		}
		if got.Items[1].CarrierCompanyID != "33333333-3333-3333-3333-333333333333" || got.Items[1].Measure.Value == nil || got.Items[1].Measure.Value.String() != "0.5" {
			t.Fatalf("%+v", got.Items)
		}
	})
	t.Run("AN03B_08", func(t *testing.T) {
		got, err := BuildCarrier(CarrierDeliveryRate, snap, now)
		if err != nil || got.Items[1].Measure.Value == nil || got.Items[1].Measure.Value.String() != "0.5" || *got.Items[1].Measure.Numerator != 2 || *got.Items[1].Measure.Denominator != 4 {
			t.Fatalf("%+v %v", got, err)
		}
	})
	t.Run("AN03B_09", func(t *testing.T) {
		got, err := BuildCarrier(CarrierPickupRate, snap, now)
		if err != nil || got.Items[0].Measure.Value != nil || *got.Items[0].Measure.Numerator != 0 || *got.Items[0].Measure.Denominator != 0 {
			t.Fatalf("%+v %v", got, err)
		}
	})
	t.Run("AN03B_10", func(t *testing.T) {
		got, err := BuildCarrier(CarrierDeliveryRate, snap, now)
		if err != nil || got.Items[0].CarrierCompanyID != "22222222-2222-2222-2222-222222222222" || got.Items[1].CarrierCompanyID != "33333333-3333-3333-3333-333333333333" {
			t.Fatalf("%+v %v", got.Items, err)
		}
	})
	t.Run("AN03B_11", func(t *testing.T) {
		got, err := BuildCarrier(CarrierPickupRate, Snapshot{}, now)
		if err != nil || got.Items == nil || len(got.Items) != 0 {
			t.Fatalf("%+v %v", got, err)
		}
		raw, err := json.Marshal(got)
		if err != nil || !strings.Contains(string(raw), `"items":[]`) || strings.Contains(string(raw), `"value":`) {
			t.Fatalf("%s %v", raw, err)
		}
	})
	t.Run("AN03B_19", func(t *testing.T) {
		delivery, err := Build(OnTimeDeliveryRate, snap, now)
		if err != nil || delivery.KPIID != OnTimeDeliveryRate || delivery.Measure.Value == nil || delivery.Measure.Value.String() != "0.8" {
			t.Fatalf("%+v %v", delivery, err)
		}
		for _, id := range []string{ShipmentsTotal, OnTimeDelivery, ReturnCases, RedirectCases} {
			got, err := Build(id, snap, now)
			if err != nil || got.KPIID != id || got.DefinitionVersion != 1 {
				t.Fatalf("%s %+v %v", id, got, err)
			}
		}
	})
}

func assertFinite(t *testing.T, text string) {
	t.Helper()
	value, err := json.Number(text).Float64()
	if err != nil || math.IsNaN(value) || math.IsInf(value, 0) {
		t.Fatalf("value %s err %v", text, err)
	}
}
