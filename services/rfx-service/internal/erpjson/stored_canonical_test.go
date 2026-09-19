package erpjson

import (
	"testing"

	"github.com/freight-platform/rfx-service/internal/domain"
)

func TestBindUpdateBaselineTokensAndParse(t *testing.T) {
	raw := []byte(`{"event":{"currency":"USD","title":"T","type":"SPOT_RFQ","timezone":"UTC"},"mapping_context":{"mapping_set_id":"11111111-1111-1111-1111-111111111111","mapping_set_version":1},"requested_operation":"UPDATE_DRAFT","schema_version":"BINTRANS_RFX_ERP_JSON_V1"}`)
	bound, err := BindUpdateBaselineTokens(raw, UpdateBaselineTokens{
		EventRowVersion:         3,
		DraftRowVersion:         2,
		BaselineLotsFingerprint: "abc123",
	})
	if err != nil {
		t.Fatalf("bind: %v", err)
	}
	stored, err := ParseStoredUpdateCanonical(bound)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if stored.EventRowVersion != 3 || stored.DraftRowVersion != 2 || stored.BaselineLotsFingerprint != "abc123" {
		t.Fatalf("tokens=%+v", stored)
	}
	if stored.BaselineExternalLinkID != "" || stored.BaselineExternalRevision != "" {
		t.Fatalf("empty link baseline expected, got %+v", stored)
	}
	if stored.RequestedOperation != OperationUpdateDraft {
		t.Fatalf("op=%s", stored.RequestedOperation)
	}
}

func TestBindUpdateBaselineTokensPinsExistingLink(t *testing.T) {
	raw := []byte(`{"event":{"currency":"USD","title":"T","type":"SPOT_RFQ","timezone":"UTC"},"external":{"object_id":"OBJ-1","revision":"2","system":"SAP"},"mapping_context":{"mapping_set_id":"11111111-1111-1111-1111-111111111111","mapping_set_version":1},"requested_operation":"UPDATE_DRAFT","schema_version":"BINTRANS_RFX_ERP_JSON_V1"}`)
	bound, err := BindUpdateBaselineTokens(raw, UpdateBaselineTokens{
		EventRowVersion:          3,
		DraftRowVersion:          2,
		BaselineLotsFingerprint:  "abc123",
		BaselineExternalLinkID:   "22222222-2222-2222-2222-222222222222",
		BaselineExternalRevision: "1",
	})
	if err != nil {
		t.Fatalf("bind: %v", err)
	}
	stored, err := ParseStoredUpdateCanonical(bound)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if stored.BaselineExternalLinkID != "22222222-2222-2222-2222-222222222222" || stored.BaselineExternalRevision != "1" {
		t.Fatalf("link baseline=%+v", stored)
	}
	if stored.External == nil || stored.External.Revision != "2" {
		t.Fatalf("external=%+v", stored.External)
	}
}

func TestParseStoredUpdateCanonicalRejectsMissingTokens(t *testing.T) {
	raw := []byte(`{"event":{"currency":"USD","title":"T","type":"SPOT_RFQ","timezone":"UTC"},"requested_operation":"UPDATE_DRAFT","schema_version":"BINTRANS_RFX_ERP_JSON_V1"}`)
	if _, err := ParseStoredUpdateCanonical(raw); err == nil {
		t.Fatal("expected missing baseline tokens to fail")
	}
}

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
