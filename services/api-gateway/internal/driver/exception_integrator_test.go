package driver

import "testing"

func TestMapDriverExceptionSeverityServerControlled(t *testing.T) {
	if mapDriverExceptionSeverity("ACCIDENT") != "high" {
		t.Fatal("expected high severity for accident")
	}
	if mapDriverExceptionSeverity("TRAFFIC") != "medium" {
		t.Fatal("expected medium severity for traffic")
	}
	if mapDriverExceptionSeverity("OTHER") != "low" {
		t.Fatal("expected low severity default")
	}
}

func TestMapDriverExceptionEventType(t *testing.T) {
	if mapDriverExceptionEventType("VEHICLE_BREAKDOWN") != "vehicle_breakdown" {
		t.Fatalf("unexpected event type mapping")
	}
}

func TestExceptionIntegratorSkipsReplay(t *testing.T) {
	integrator := NewExceptionIntegrator(nil, true, 0)
	err := integrator.Integrate(t.Context(), RequestContext{TenantID: "t1"}, ExceptionIntegrationInput{
		ExceptionID: "e1", ShipmentID: "s1", Category: "TRAFFIC", Replayed: true,
	})
	if err != nil {
		t.Fatalf("replay should be skipped without error: %v", err)
	}
}
