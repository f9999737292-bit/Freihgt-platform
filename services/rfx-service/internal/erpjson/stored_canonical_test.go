package erpjson

import (
	"testing"

	"github.com/freight-platform/rfx-service/internal/domain"
)

func TestParseStoredCreateCanonicalAndStableHash(t *testing.T) {
	raw := []byte(`{"event":{"currency":"USD","title":"T","type":"SPOT_RFQ","timezone":"UTC"},"external":{"object_id":"OBJ-1","system":"sap"},"mapping_context":{"mapping_set_id":"11111111-1111-1111-1111-111111111111","mapping_set_version":1},"requested_operation":"CREATE_DRAFT","schema_version":"BINTRANS_RFX_ERP_JSON_V1"}`)
	stored, err := ParseStoredCreateCanonical(raw)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if stored.External.System != "SAP" || stored.RequestedOperation != OperationCreateDraft {
		t.Fatalf("stored=%+v", stored)
	}
	hash, err := StableHash(raw)
	if err != nil || len(hash) != 64 {
		t.Fatalf("hash=%s err=%v", hash, err)
	}
	if domain.MachineCodeCanonicalHashMismatch == "" {
		t.Fatal("machine code must be defined")
	}
}
