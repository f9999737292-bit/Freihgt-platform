package controltower

import (
	"testing"

	"github.com/freight-platform/api-gateway/internal/controltower/risk"
)

func TestSignalToTriggerType(t *testing.T) {
	cases := map[string]string{
		"eta_delivery_at_risk":       "eta_at_risk",
		"eta_after_planned_delivery": "eta_projected_late",
		"slot_projected_miss":        "slot_projected_miss",
		"slot_actual_missed":         "slot_actual_missed",
		"telemetry_stale":            "tracking_stale",
		"telemetry_lost":             "tracking_lost",
		"unknown_signal":             "",
	}
	for code, want := range cases {
		if got := signalToTriggerType(code); got != want {
			t.Fatalf("signalToTriggerType(%q) = %q, want %q", code, got, want)
		}
	}
}

func TestBuildRiskCreatedTrigger_IdempotentStateVersion(t *testing.T) {
	item := risk.Assessment{
		RiskID: "risk-1", ShipmentID: "ship-1", Level: "high", Score: 85,
		PredictedExceptionType: "delivery_delay_risk",
	}
	payload := buildRiskCreatedTrigger(item)
	if payload["triggerType"] != "risk_created" {
		t.Fatalf("unexpected trigger type: %v", payload["triggerType"])
	}
	id1 := payload["triggerId"].(string)
	id2 := buildRiskCreatedTrigger(item)["triggerId"].(string)
	if id1 != id2 {
		t.Fatalf("trigger id not stable: %q vs %q", id1, id2)
	}
}

func TestMapSignalToTrigger_IncludesShipmentAttributes(t *testing.T) {
	etaStatus := "available"
	projection := "late"
	delay := int64(1800)
	row := ControlTowerShipment{
		ID: "ship-1", ETAStatus: &etaStatus, ArrivalProjection: &projection, ProjectedDelaySeconds: &delay,
	}
	item := risk.Assessment{RiskID: "r1", ShipmentID: "ship-1", Level: "high"}
	sig := risk.Signal{Code: "eta_after_planned_delivery", Weight: 55}
	payload := mapSignalToTrigger(item, sig, row, true)
	if payload == nil {
		t.Fatal("expected trigger payload")
	}
	if payload["triggerType"] != "eta_projected_late" {
		t.Fatalf("unexpected type: %v", payload["triggerType"])
	}
	attrs := payload["attributes"].(map[string]any)
	if attrs["etaStatus"] != "available" {
		t.Fatalf("missing etaStatus in attributes: %#v", attrs)
	}
}

func TestBuildShipmentSLATrigger(t *testing.T) {
	row := ControlTowerShipment{ID: "ship-1", SLAStatus: SLAStatusAtRisk}
	payload := buildShipmentSLATrigger(row)
	if payload == nil || payload["triggerType"] != "sla_warning" {
		t.Fatalf("expected sla_warning trigger, got %#v", payload)
	}
	row.SLAStatus = SLAStatusCritical
	payload = buildShipmentSLATrigger(row)
	if payload["triggerType"] != "sla_breached" {
		t.Fatalf("expected sla_breached trigger, got %#v", payload)
	}
}
