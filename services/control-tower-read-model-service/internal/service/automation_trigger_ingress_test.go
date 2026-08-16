package service

import (
	"context"
	"testing"

	"github.com/google/uuid"

	"github.com/freight-platform/control-tower-read-model-service/internal/domain"
)

func TestIsAutomationOrigin_LoopGuard(t *testing.T) {
	if !IsAutomationOrigin("automation:exec:123") {
		t.Fatal("expected automation origin")
	}
	if IsAutomationOrigin("risk-sync:abc") {
		t.Fatal("expected non-automation origin")
	}
}

func TestAutomationTriggerIngress_SkipsAutomationOrigin(t *testing.T) {
	svc := &AutomationService{}
	ingress := NewAutomationTriggerIngress(svc, nil, nil)
	trigger := domain.AutomationTrigger{
		TriggerType: "case_created",
		TriggerID:   "case:1:v1",
		CausationID: AutomationCausationPrefix + "rec:1",
	}
	out, err := ingress.HandleTrigger(context.Background(), uuid.New(), trigger, true)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(out.Matches) != 0 || len(out.Recommendations) != 0 {
		t.Fatalf("expected skipped trigger, got %#v", out)
	}
}

func TestBuildIdempotencyKey_Deterministic(t *testing.T) {
	tenantID := uuid.MustParse("11111111-1111-1111-1111-111111111111")
	ruleID := uuid.MustParse("22222222-2222-2222-2222-222222222222")
	shipmentID := uuid.MustParse("33333333-3333-3333-3333-333333333333")
	rule := domain.AutomationRule{ID: ruleID, Version: 2}
	trigger := domain.AutomationTrigger{
		TriggerType: "eta_at_risk",
		TriggerID:   "eta:ship:state1",
		ShipmentID:  &shipmentID,
		Attributes:  domain.TriggerAttributes{StateVersion: "v1"},
	}
	k1 := buildIdempotencyKey(tenantID, rule, trigger)
	k2 := buildIdempotencyKey(tenantID, rule, trigger)
	if k1 != k2 {
		t.Fatalf("idempotency key not deterministic: %q vs %q", k1, k2)
	}
}
