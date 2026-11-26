package allocationpolicy

import "testing"

func TestSchemaConstants_W11Vocabulary(t *testing.T) {
	if SchemaVersionV1 != "allocation_policy_ast.v1" {
		t.Fatalf("schema version=%q", SchemaVersionV1)
	}
	if MaxJSONPayloadBytes != 65536 {
		t.Fatalf("payload limit=%d", MaxJSONPayloadBytes)
	}
	if SharePolicyModeDefault != SharePolicyModeCapOnly {
		t.Fatalf("default share mode=%q", SharePolicyModeDefault)
	}

	kinds := []NodeKind{
		NodeKindFilter, NodeKindScore, NodeKindPreference, NodeKindCap,
		NodeKindFloor, NodeKindFallback, NodeKindApproval, NodeKindAction,
		NodeKindAnd, NodeKindOr, NodeKindNot,
	}
	for _, k := range kinds {
		if _, ok := allowedNodeKinds[k]; !ok {
			t.Fatalf("node kind %q missing from allowlist", k)
		}
	}

	ops := []Operator{
		OpEQ, OpNE, OpGT, OpGTE, OpLT, OpLTE, OpIN, OpNOTIN, OpBETWEEN,
		OpEXISTS, OpISMissing, OpAND, OpOR, OpNOT,
	}
	for _, op := range ops {
		if _, ok := allowedOperators[op]; !ok {
			t.Fatalf("operator %q missing from allowlist", op)
		}
	}

	vtypes := []ValueType{
		ValueTypeDecimal, ValueTypeString, ValueTypeBoolean, ValueTypeEnum,
		ValueTypeTimestamp, ValueTypeNull, ValueTypeMissing, ValueTypeNotAvailable,
	}
	for _, vt := range vtypes {
		if _, ok := allowedValueTypes[vt]; !ok {
			t.Fatalf("value type %q missing from allowlist", vt)
		}
	}
}
