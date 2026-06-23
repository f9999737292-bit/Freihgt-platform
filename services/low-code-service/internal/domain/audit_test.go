package domain

import (
	"encoding/json"
	"testing"

	"github.com/google/uuid"
)

func TestBuildCustomFieldValuesAuditPayload(t *testing.T) {
	templateID := uuid.MustParse("b1111111-1111-4111-8111-111111111102")
	resolved := []ResolvedCustomFieldValueForAudit{
		{FieldCode: "internal_cost_center", ValueJSON: []byte(`"CC-2002"`)},
	}
	oldValues := map[string][]byte{
		"internal_cost_center": []byte(`"CC-1001"`),
	}

	oldJSON, newJSON, changedFields, err := BuildCustomFieldValuesAuditPayload(templateID, resolved, oldValues)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(changedFields) != 1 || changedFields[0] != "internal_cost_center" {
		t.Fatalf("unexpected changed fields: %#v", changedFields)
	}

	var oldPayload map[string]any
	if err := json.Unmarshal(oldJSON, &oldPayload); err != nil {
		t.Fatalf("old json: %v", err)
	}
	var newPayload map[string]any
	if err := json.Unmarshal(newJSON, &newPayload); err != nil {
		t.Fatalf("new json: %v", err)
	}

	if newPayload["event_kind"] != AuditEventKindCustomFieldValuesUpdated {
		t.Fatalf("expected event kind, got %#v", newPayload["event_kind"])
	}
}

func TestParseAuditEventAction(t *testing.T) {
	raw := json.RawMessage(`{"event_kind":"CUSTOM_FIELD_VALUES_UPDATED"}`)
	action := ParseAuditEventAction(AuditDBActionUpdate, raw)
	if action != AuditEventKindCustomFieldValuesUpdated {
		t.Fatalf("expected mapped action, got %s", action)
	}
}

func TestBuildCustomFieldValuesMigratedToActiveAuditPayload(t *testing.T) {
	sourceID := uuid.New()
	targetID := uuid.New()
	_, newJSON, err := BuildCustomFieldValuesMigratedToActiveAuditPayload(
		sourceID,
		targetID,
		MigrationPreviewItem{
			Status:       MigrationPreviewStatusSafe,
			CopiedFields: []string{"cargo_class"},
		},
		true,
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	var payload map[string]any
	if err := json.Unmarshal(newJSON, &payload); err != nil {
		t.Fatalf("json: %v", err)
	}
	if payload["event_kind"] != AuditEventKindCustomFieldValuesMigratedToActive {
		t.Fatalf("unexpected event kind: %#v", payload["event_kind"])
	}
	action := ParseAuditEventAction(AuditDBActionUpdate, newJSON)
	if action != AuditEventKindCustomFieldValuesMigratedToActive {
		t.Fatalf("expected parsed action, got %s", action)
	}
}
