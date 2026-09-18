package erpjson

import (
	"fmt"
	"strings"
	"testing"
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

func TestIngestRejectsDepthBomb(t *testing.T) {
	var b strings.Builder
	b.WriteString(strings.Repeat(`{"a":`, MaxJSONDepth+1))
	b.WriteString(`1`)
	b.WriteString(strings.Repeat(`}`, MaxJSONDepth+1))
	if err := validateStructure([]byte(b.String()), MaxJSONDepth); err == nil {
		t.Fatal("expected depth rejection")
	}
}
