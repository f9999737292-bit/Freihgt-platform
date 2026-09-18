package erpjson

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/google/uuid"
)

func TestIngestRejectsDuplicateKeys(t *testing.T) {
	raw := []byte(`{"schema_version":"BINTRANS_RFX_ERP_JSON_V1","schema_version":"X"}`)
	if err := validateStructure(raw, MaxJSONDepth); err == nil {
		t.Fatal("expected duplicate key rejection")
	}
	issues := IngestErrorToIssues(fmt.Errorf(`duplicate key "schema_version"`))
	if len(issues) != 1 || issues[0].MachineCode != MachineCodeDuplicateField {
		t.Fatalf("issues=%+v", issues)
	}
}

func TestIngestRejectsNestedDuplicateKeys(t *testing.T) {
	raw := []byte(`{"schema_version":"BINTRANS_RFX_ERP_JSON_V1","event":{"title":"A","title":"B"}}`)
	if err := validateStructure(raw, MaxJSONDepth); err == nil {
		t.Fatal("expected nested duplicate key rejection")
	}
}

func TestIngestRejectsDepthBomb(t *testing.T) {
	var b strings.Builder
	b.WriteString(strings.Repeat(`{"a":`, MaxJSONDepth+1))
	b.WriteString(`1`)
	b.WriteString(strings.Repeat(`}`, MaxJSONDepth+1))
	if err := validateStructure([]byte(b.String()), MaxJSONDepth); err == nil {
		t.Fatal("expected depth rejection")
	}
}

func TestIngestRejectsTrailingJSON(t *testing.T) {
	var env previewEnvelope
	if err := IngestJSON([]byte(`{"schema_version":"BINTRANS_RFX_ERP_JSON_V1"}{}`), &env); err == nil {
		t.Fatal("expected trailing json rejection")
	}
}

func TestIngestRejectsUnknownTypedFields(t *testing.T) {
	var event EventPayload
	err := IngestJSON([]byte(`{"type":"SPOT_RFQ","title":"T","currency":"USD","timezone":"UTC","owner_id":"x"}`), &event)
	if err == nil {
		t.Fatal("expected unknown event field rejection")
	}
	issues := prefixIssuePaths(IngestErrorToIssues(err), "event")
	if len(issues) != 1 || issues[0].MachineCode != MachineCodeUnknownField || issues[0].Path != "event.owner_id" {
		t.Fatalf("issues=%+v", issues)
	}
	if issues[0].MessageKey != "rfx.erp.unknown_field" {
		t.Fatalf("message_key=%s", issues[0].MessageKey)
	}
}

func TestIngestRejectsClientMappingContext(t *testing.T) {
	raw := []byte(`{"schema_version":"BINTRANS_RFX_ERP_JSON_V1","requested_operation":"CREATE_DRAFT","event":{"type":"SPOT_RFQ","title":"T","currency":"USD","timezone":"UTC"},"mapping_context":{"mapping_set_id":"00000000-0000-0000-0000-000000000001"}}`)
	parsed, err := ParsePreview(context.Background(), uuid.Nil, OperationCreateDraft, raw, nil)
	if err != nil {
		t.Fatalf("ParsePreview: %v", err)
	}
	if parsed == nil || len(parsed.Errors) == 0 {
		t.Fatal("expected client mapping_context rejection")
	}
	found := false
	for _, issue := range parsed.Errors {
		if issue.Path == "mapping_context" && (issue.MachineCode == MachineCodeUnknownField || issue.MachineCode == MachineCodeUnsupportedFieldV1) && strings.HasPrefix(issue.MessageKey, "rfx.erp.") {
			found = true
		}
	}
	if !found {
		t.Fatalf("issues=%+v", parsed.Errors)
	}
}

func TestIngestKeepsRawExtensionsOpen(t *testing.T) {
	raw := []byte(`{"schema_version":"BINTRANS_RFX_ERP_JSON_V1","requested_operation":"CREATE_DRAFT","event":{"type":"SPOT_RFQ","title":"T","currency":"USD","timezone":"UTC"},"extensions":{"freight":{"unit_code":"TNE","custom_note":"keep"}}}`)
	var env previewEnvelope
	if err := IngestJSON(raw, &env); err != nil {
		t.Fatalf("envelope ingest: %v", err)
	}
	if !strings.Contains(string(env.Extensions), "custom_note") {
		t.Fatalf("raw extensions lost extra field: %s", env.Extensions)
	}
}
