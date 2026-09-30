package domain

import "testing"

func TestClassifyEventPayloadRoutesFamilies(t *testing.T) {
	cases := []struct {
		payload string
		kind    string
	}{
		{`{"eventType":"shipment.status.changed"}`, RouteShipmentStatus},
		{`{"event_type":"shipment.created"}`, RouteShipmentStatus},
		{`{"event_type":"shipment.execution_plan.created"}`, RouteExecution},
		{`{"event_type":"shipment.execution_plan.superseded"}`, RouteExecution},
		{`{"event_type":"shipment.route_stop.current"}`, RouteExecution},
		{`{"eventType":"tracking.stop.approaching"}`, RouteApproach},
		{`{"event_type":"optimizer.score.ready"}`, RouteUnknown},
		{`not-json`, RouteUnknown},
	}
	for _, tc := range cases {
		kind, _ := ClassifyEventPayload([]byte(tc.payload))
		if kind != tc.kind {
			t.Fatalf("%s => %s, want %s", tc.payload, kind, tc.kind)
		}
	}
}

func TestForbiddenExecutionFields(t *testing.T) {
	if !PayloadHasForbiddenExecutionField([]byte(`{"stops":[{"latitude":1}]}`)) {
		t.Fatal("latitude must be rejected")
	}
	if !PayloadHasForbiddenExecutionField([]byte(`{"price":"1"}`)) {
		t.Fatal("price must be rejected")
	}
	if PayloadHasForbiddenExecutionField([]byte(`{"event_type":"shipment.execution_plan.created","carrier_company_id":"x"}`)) {
		t.Fatal("operational payload rejected")
	}
}

func TestOptionalDriverContext(t *testing.T) {
	legacy := DriverDomainEventEnvelope{EventType: "driver.delay.reported"}
	stop, action, err := OptionalDriverContext(legacy)
	if err != nil || stop != nil || action != nil {
		t.Fatalf("legacy %+v %+v %v", stop, action, err)
	}
	bad := DriverDomainEventEnvelope{ExecutionStopID: "not-a-uuid"}
	if _, _, err := OptionalDriverContext(bad); err == nil || err.Code != DriverEventErrorMalformedStopID {
		t.Fatalf("stop err %#v", err)
	}
	badAction := DriverDomainEventEnvelope{Metadata: map[string]any{"action_id": "nope"}}
	if _, _, err := OptionalDriverContext(badAction); err == nil || err.Code != DriverEventErrorMalformedActionID {
		t.Fatalf("action err %#v", err)
	}
	_, perm := ParseDriverEventEnvelope([]byte(`{"eventId":"11111111-1111-1111-1111-111111111111","eventType":"driver.delay.reported","tenantId":"22222222-2222-2222-2222-222222222222","shipmentId":"33333333-3333-3333-3333-333333333333","executionStopId":"bad"}`))
	if perm == nil || perm.Code != DriverEventErrorMalformedStopID {
		t.Fatalf("parse %#v", perm)
	}
	_, legacyErr := ParseDriverEventEnvelope([]byte(`{"eventId":"11111111-1111-1111-1111-111111111111","eventType":"driver.problem.reported","tenantId":"22222222-2222-2222-2222-222222222222","shipmentId":"33333333-3333-3333-3333-333333333333"}`))
	if legacyErr != nil {
		t.Fatal(legacyErr)
	}
}
