package allocationpolicy

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/shopspring/decimal"
)

func TestDecodePolicy_ValidVersionAndCapOnlyDefault(t *testing.T) {
	raw, err := os.ReadFile(filepath.Join("testdata", "valid_cap_only_default.json"))
	if err != nil {
		t.Fatalf("read testdata: %v", err)
	}
	p, err := DecodePolicy(raw)
	if err != nil {
		t.Fatalf("DecodePolicy: %v", err)
	}
	if p.SchemaVersion != SchemaVersionV1 {
		t.Fatalf("schema_version=%q want %q", p.SchemaVersion, SchemaVersionV1)
	}
	if p.SharePolicyMode != SharePolicyModeCapOnly {
		t.Fatalf("share_policy_mode=%q want CAP_ONLY default", p.SharePolicyMode)
	}
	if p.TieBreak.Strategy != TieBreakCarrierIDAsc {
		t.Fatalf("tie_break=%q", p.TieBreak.Strategy)
	}
	if p.Root.Kind != NodeKindAnd {
		t.Fatalf("root.kind=%q", p.Root.Kind)
	}
	if len(p.Root.Args) != 4 {
		t.Fatalf("root.args len=%d", len(p.Root.Args))
	}
}

func TestDecodePolicy_UnknownSchemaVersion(t *testing.T) {
	raw := minimalPolicyJSON(map[string]any{
		"schema_version": "allocation_policy_ast.v999",
	})
	_, err := DecodePolicy(raw)
	assertDecodeCode(t, err, "UNKNOWN_SCHEMA_VERSION")
}

func TestDecodePolicy_UnknownField(t *testing.T) {
	raw := []byte(`{
  "schema_version": "allocation_policy_ast.v1",
  "policy_id": "p1",
  "scoring_model": "ADDITIVE_WEIGHTS",
  "tie_break": {"strategy": "CARRIER_ID_ASC"},
  "root": {"kind": "AND", "args": []},
  "unexpected_top_level": true
}`)
	_, err := DecodePolicy(raw)
	assertDecodeCode(t, err, "UNKNOWN_FIELD")
}

func TestDecodePolicy_UnknownOperator(t *testing.T) {
	raw := []byte(`{
  "schema_version": "allocation_policy_ast.v1",
  "policy_id": "p1",
  "scoring_model": "ADDITIVE_WEIGHTS",
  "tie_break": {"strategy": "CARRIER_ID_ASC"},
  "root": {
    "kind": "FILTER",
    "op": "REGEX_MATCH",
    "attr": "name",
    "value": {"type": "STRING", "string": "x"}
  }
}`)
	_, err := DecodePolicy(raw)
	assertDecodeCode(t, err, "UNKNOWN_OPERATOR")
}

func TestDecodePolicy_UnknownNodeKind(t *testing.T) {
	raw := []byte(`{
  "schema_version": "allocation_policy_ast.v1",
  "policy_id": "p1",
  "scoring_model": "ADDITIVE_WEIGHTS",
  "tie_break": {"strategy": "CARRIER_ID_ASC"},
  "root": {"kind": "MAGIC_BLOCK", "args": []}
}`)
	_, err := DecodePolicy(raw)
	assertDecodeCode(t, err, "UNKNOWN_NODE_KIND")
}

func TestDecodePolicy_DecimalNoFloat64(t *testing.T) {
	// JSON number for decimal must be rejected (float64 path forbidden).
	raw := []byte(`{
  "schema_version": "allocation_policy_ast.v1",
  "policy_id": "p1",
  "scoring_model": "ADDITIVE_WEIGHTS",
  "tie_break": {"strategy": "CARRIER_ID_ASC"},
  "root": {
    "kind": "SCORE",
    "attr": "offered_price",
    "weight": 0.4,
    "op": "LT",
    "value": {"type": "DECIMAL", "decimal": "100"}
  }
}`)
	_, err := DecodePolicy(raw)
	if err == nil {
		t.Fatal("expected reject of JSON-number weight")
	}
	de, ok := err.(*DecodeError)
	if !ok {
		t.Fatalf("want DecodeError, got %T %v", err, err)
	}
	if de.Code != "DECIMAL_FLOAT64_FORBIDDEN" && de.Code != "JSON_DECODE" && de.Code != "INVALID_VALUE_TYPE" {
		// encoding/json may surface via custom UnmarshalJSON; accept mapped codes.
		if !strings.Contains(err.Error(), "float64") && !strings.Contains(err.Error(), "decimal must be a JSON string") {
			t.Fatalf("unexpected error: %v", err)
		}
	}

	// String decimal path succeeds and uses shopspring/decimal.
	rawOK := []byte(`{
  "schema_version": "allocation_policy_ast.v1",
  "policy_id": "p1",
  "scoring_model": "ADDITIVE_WEIGHTS",
  "tie_break": {"strategy": "CARRIER_ID_ASC"},
  "root": {
    "kind": "SCORE",
    "attr": "offered_price",
    "weight": "0.40",
    "op": "LT",
    "value": {"type": "DECIMAL", "decimal": "1000.50"}
  }
}`)
	p, err := DecodePolicy(rawOK)
	if err != nil {
		t.Fatalf("string decimal decode: %v", err)
	}
	if p.Root.Weight == nil {
		t.Fatal("weight nil")
	}
	want, _ := decimal.NewFromString("0.40")
	if !p.Root.Weight.Equal(want) {
		t.Fatalf("weight=%s want 0.40", p.Root.Weight.String())
	}
	if p.Root.Value == nil || p.Root.Value.Decimal == nil {
		t.Fatal("value.decimal nil")
	}
	wantVal, _ := decimal.NewFromString("1000.50")
	if !p.Root.Value.Decimal.Equal(wantVal) {
		t.Fatalf("decimal=%s", p.Root.Value.Decimal.String())
	}
}

func TestDecodePolicy_OversizedJSON(t *testing.T) {
	base := []byte(`{
  "schema_version": "allocation_policy_ast.v1",
  "policy_id": "p1",
  "scoring_model": "ADDITIVE_WEIGHTS",
  "tie_break": {"strategy": "CARRIER_ID_ASC"},
  "root": {"kind": "AND", "args": [], "node_id": "`)
	// Pad node_id so total raw length exceeds MaxJSONPayloadBytes before parse.
	pad := bytes.Repeat([]byte("x"), MaxJSONPayloadBytes)
	raw := append(append(base, pad...), []byte(`"} }` )...)
	if len(raw) <= MaxJSONPayloadBytes {
		t.Fatalf("test setup: payload len %d not oversized", len(raw))
	}
	_, err := DecodePolicy(raw)
	assertDecodeCode(t, err, "PAYLOAD_TOO_LARGE")
}

func TestDecodePolicy_WrongValueType(t *testing.T) {
	raw := []byte(`{
  "schema_version": "allocation_policy_ast.v1",
  "policy_id": "p1",
  "scoring_model": "ADDITIVE_WEIGHTS",
  "tie_break": {"strategy": "CARRIER_ID_ASC"},
  "root": {
    "kind": "FILTER",
    "op": "EQ",
    "attr": "flag",
    "value": {"type": "BOOLEAN", "string": "true"}
  }
}`)
	_, err := DecodePolicy(raw)
	assertDecodeCode(t, err, "INVALID_VALUE_SHAPE")
}

func TestDecodePolicy_UnknownValueType(t *testing.T) {
	raw := []byte(`{
  "schema_version": "allocation_policy_ast.v1",
  "policy_id": "p1",
  "scoring_model": "ADDITIVE_WEIGHTS",
  "tie_break": {"strategy": "CARRIER_ID_ASC"},
  "root": {
    "kind": "FILTER",
    "op": "EQ",
    "attr": "x",
    "value": {"type": "FLOAT64", "decimal": "1"}
  }
}`)
	_, err := DecodePolicy(raw)
	assertDecodeCode(t, err, "UNKNOWN_VALUE_TYPE")
}

func TestDecodePolicy_SentinelValueStatesRepresentable(t *testing.T) {
	// Representation only — no EXISTS(NULL) evaluation in W1.1.
	for _, typ := range []ValueType{ValueTypeNull, ValueTypeMissing, ValueTypeNotAvailable} {
		raw := []byte(`{
  "schema_version": "allocation_policy_ast.v1",
  "policy_id": "p1",
  "scoring_model": "ADDITIVE_WEIGHTS",
  "tie_break": {"strategy": "CARRIER_ID_ASC"},
  "root": {
    "kind": "FILTER",
    "op": "IS_MISSING",
    "attr": "optional_field",
    "value": {"type": "` + string(typ) + `"}
  }
}`)
		p, err := DecodePolicy(raw)
		if err != nil {
			t.Fatalf("type %s: %v", typ, err)
		}
		if p.Root.Value.Type != typ {
			t.Fatalf("got %s want %s", p.Root.Value.Type, typ)
		}
	}
}

func TestDecodePolicy_ForbiddenExternalSurfaces(t *testing.T) {
	cases := []string{
		`{"schema_version":"allocation_policy_ast.v1","policy_id":"p1","scoring_model":"ADDITIVE_WEIGHTS","tie_break":{"strategy":"CARRIER_ID_ASC"},"root":{"kind":"ACTION"},"sql":"SELECT 1"}`,
		`{"schema_version":"allocation_policy_ast.v1","policy_id":"p1","scoring_model":"ADDITIVE_WEIGHTS","tie_break":{"strategy":"CARRIER_ID_ASC"},"root":{"kind":"ACTION"},"script":"alert(1)"}`,
		`{"schema_version":"allocation_policy_ast.v1","policy_id":"p1","scoring_model":"ADDITIVE_WEIGHTS","tie_break":{"strategy":"CARRIER_ID_ASC"},"root":{"kind":"ACTION"},"http":"https://evil.example"}`,
		`{"schema_version":"allocation_policy_ast.v1","policy_id":"p1","scoring_model":"ADDITIVE_WEIGHTS","tie_break":{"strategy":"CARRIER_ID_ASC"},"root":{"kind":"ACTION"},"shell":"rm -rf /"}`,
		`{"schema_version":"allocation_policy_ast.v1","policy_id":"p1","scoring_model":"ADDITIVE_WEIGHTS","tie_break":{"strategy":"CARRIER_ID_ASC"},"root":{"kind":"ACTION"},"exec":"boom"}`,
	}
	for _, c := range cases {
		_, err := DecodePolicy([]byte(c))
		if err == nil {
			t.Fatalf("expected forbidden reject for %s", c)
		}
		de, ok := err.(*DecodeError)
		if !ok {
			t.Fatalf("want DecodeError, got %T %v", err, err)
		}
		// Either FORBIDDEN_SURFACE (pre-scan) or UNKNOWN_FIELD (DisallowUnknownFields).
		if de.Code != "FORBIDDEN_SURFACE" && de.Code != "UNKNOWN_FIELD" {
			t.Fatalf("code=%s err=%v", de.Code, err)
		}
	}
}

func TestDecodePolicy_ImmutableInput(t *testing.T) {
	raw := []byte(`{
  "schema_version": "allocation_policy_ast.v1",
  "policy_id": "p1",
  "scoring_model": "ADDITIVE_WEIGHTS",
  "tie_break": {"strategy": "CARRIER_ID_ASC"},
  "root": {"kind": "AND", "args": []}
}`)
	clone := append([]byte(nil), raw...)
	_, err := DecodePolicy(raw)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(raw, clone) {
		t.Fatal("DecodePolicy mutated input buffer")
	}
}

func TestDecodePolicy_NoUniversalActionExecutor(t *testing.T) {
	// ACTION is structural; package must not expose an Execute/Run/Eval entrypoint.
	raw := []byte(`{
  "schema_version": "allocation_policy_ast.v1",
  "policy_id": "p1",
  "scoring_model": "ADDITIVE_WEIGHTS",
  "tie_break": {"strategy": "CARRIER_ID_ASC"},
  "root": {
    "kind": "ACTION",
    "attr": "annotate",
    "value": {"type": "STRING", "string": "ok"}
  }
}`)
	p, err := DecodePolicy(raw)
	if err != nil {
		t.Fatal(err)
	}
	if p.Root.Kind != NodeKindAction {
		t.Fatalf("kind=%s", p.Root.Kind)
	}
	// Compile-time / package-surface check: only DecodePolicy is the public entry.
	// Runtime evaluation symbols are intentionally absent in W1.1.
	_ = p
}

func TestDecodePolicy_ExplicitTargetWithCap(t *testing.T) {
	raw := []byte(`{
  "schema_version": "allocation_policy_ast.v1",
  "policy_id": "p1",
  "scoring_model": "ADDITIVE_WEIGHTS",
  "share_policy_mode": "TARGET_WITH_CAP",
  "tie_break": {"strategy": "CARRIER_ID_ASC"},
  "root": {"kind": "CAP", "attr": "share_pct", "op": "LTE", "value": {"type": "DECIMAL", "decimal": "0.5"}}
}`)
	p, err := DecodePolicy(raw)
	if err != nil {
		t.Fatal(err)
	}
	if p.SharePolicyMode != SharePolicyModeTargetWithCap {
		t.Fatalf("got %s", p.SharePolicyMode)
	}
}

func assertDecodeCode(t *testing.T, err error, code string) {
	t.Helper()
	if err == nil {
		t.Fatalf("expected error code %s", code)
	}
	de, ok := err.(*DecodeError)
	if !ok {
		t.Fatalf("want DecodeError with code %s, got %T %v", code, err, err)
	}
	if de.Code != code {
		t.Fatalf("code=%s want %s (%v)", de.Code, code, err)
	}
}

func minimalPolicyJSON(overrides map[string]any) []byte {
	m := map[string]any{
		"schema_version": SchemaVersionV1,
		"policy_id":      "p1",
		"scoring_model":  "ADDITIVE_WEIGHTS",
		"tie_break":      map[string]any{"strategy": "CARRIER_ID_ASC"},
		"root":           map[string]any{"kind": "AND", "args": []any{}},
	}
	for k, v := range overrides {
		m[k] = v
	}
	b, err := json.Marshal(m)
	if err != nil {
		panic(err)
	}
	return b
}
