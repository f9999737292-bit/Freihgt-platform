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

func assertFinite(t *testing.T, text string) {
	t.Helper()
	value, err := json.Number(text).Float64()
	if err != nil || math.IsNaN(value) || math.IsInf(value, 0) {
		t.Fatalf("value %s err %v", text, err)
	}
}
