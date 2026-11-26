package allocationpolicy

import (
	"bytes"
	"os"
	"path/filepath"
	"sync"
	"testing"
)

func TestValidate_ValidAST(t *testing.T) {
	raw, err := os.ReadFile(filepath.Join("testdata", "valid_w12_policy.json"))
	if err != nil {
		t.Fatal(err)
	}
	p, err := DecodeAndValidate(raw)
	if err != nil {
		t.Fatalf("valid AST: %v", err)
	}
	if p.ScoringModel != ScoringModelAdditiveWeights {
		t.Fatalf("scoring_model=%s", p.ScoringModel)
	}
	if p.TieBreak.Strategy != TieBreakCarrierIDAsc {
		t.Fatalf("tie_break=%s", p.TieBreak.Strategy)
	}
}

func TestValidate_UnknownScoringModel(t *testing.T) {
	cases := []struct {
		name string
		raw  string
		code string
	}{
		{"unknown", `{"schema_version":"allocation_policy_ast.v1","policy_id":"p1","scoring_model":"WEIGHTED_LEGACY","root":{"kind":"SCORE","attr":"offered_price","weight":"1.00"}}`, "UNKNOWN_SCORING_MODEL"},
		{"legacy_70_30", `{"schema_version":"allocation_policy_ast.v1","policy_id":"p1","scoring_model":"70/30","root":{"kind":"SCORE","attr":"offered_price","weight":"1.00"}}`, "UNKNOWN_SCORING_MODEL"},
		{"legacy_commercial", `{"schema_version":"allocation_policy_ast.v1","policy_id":"p1","scoring_model":"LEGACY_COMMERCIAL_70_30","root":{"kind":"SCORE","attr":"offered_price","weight":"1.00"}}`, "UNKNOWN_SCORING_MODEL"},
		{"empty", `{"schema_version":"allocation_policy_ast.v1","policy_id":"p1","scoring_model":"","root":{"kind":"SCORE","attr":"offered_price","weight":"1.00"}}`, "MISSING_SCORING_MODEL"},
		{"null", `{"schema_version":"allocation_policy_ast.v1","policy_id":"p1","scoring_model":null,"root":{"kind":"SCORE","attr":"offered_price","weight":"1.00"}}`, "NULL_SCORING_MODEL"},
		{"wrong_type", `{"schema_version":"allocation_policy_ast.v1","policy_id":"p1","scoring_model":123,"root":{"kind":"SCORE","attr":"offered_price","weight":"1.00"}}`, "INVALID_SCORING_MODEL_TYPE"},
		{"missing", `{"schema_version":"allocation_policy_ast.v1","policy_id":"p1","root":{"kind":"SCORE","attr":"offered_price","weight":"1.00"}}`, "MISSING_SCORING_MODEL"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := DecodePolicy([]byte(tc.raw))
			assertDecodeCode(t, err, tc.code)
		})
	}
}

func TestValidate_TieBreakNormalization(t *testing.T) {
	omitted := []byte(`{
  "schema_version":"allocation_policy_ast.v1",
  "policy_id":"p1",
  "scoring_model":"ADDITIVE_WEIGHTS",
  "root":{"kind":"SCORE","attr":"offered_price","weight":"1.00"}
}`)
	explicit := []byte(`{
  "schema_version":"allocation_policy_ast.v1",
  "policy_id":"p1",
  "scoring_model":"ADDITIVE_WEIGHTS",
  "tie_break":{"strategy":"CARRIER_ID_ASC"},
  "root":{"kind":"SCORE","attr":"offered_price","weight":"1.00"}
}`)
	p1, err := DecodeAndValidate(omitted)
	if err != nil {
		t.Fatal(err)
	}
	p2, err := DecodeAndValidate(explicit)
	if err != nil {
		t.Fatal(err)
	}
	if p1.TieBreak.Strategy != TieBreakCarrierIDAsc || p2.TieBreak.Strategy != TieBreakCarrierIDAsc {
		t.Fatalf("tie_break not normalized: %v vs %v", p1.TieBreak, p2.TieBreak)
	}
	if p1.TieBreak != p2.TieBreak {
		t.Fatalf("omitted vs explicit tie_break diverge: %#v %#v", p1.TieBreak, p2.TieBreak)
	}

	neg := []struct {
		name string
		raw  string
		code string
	}{
		{"null", `{"schema_version":"allocation_policy_ast.v1","policy_id":"p1","scoring_model":"ADDITIVE_WEIGHTS","tie_break":null,"root":{"kind":"SCORE","attr":"offered_price","weight":"1.00"}}`, "NULL_TIE_BREAK"},
		{"empty_object", `{"schema_version":"allocation_policy_ast.v1","policy_id":"p1","scoring_model":"ADDITIVE_WEIGHTS","tie_break":{},"root":{"kind":"SCORE","attr":"offered_price","weight":"1.00"}}`, "MISSING_TIE_BREAK_STRATEGY"},
		{"wrong_type", `{"schema_version":"allocation_policy_ast.v1","policy_id":"p1","scoring_model":"ADDITIVE_WEIGHTS","tie_break":"CARRIER_ID_ASC","root":{"kind":"SCORE","attr":"offered_price","weight":"1.00"}}`, "INVALID_TIE_BREAK_TYPE"},
		{"unknown", `{"schema_version":"allocation_policy_ast.v1","policy_id":"p1","scoring_model":"ADDITIVE_WEIGHTS","tie_break":{"strategy":"SCORE_DESC"},"root":{"kind":"SCORE","attr":"offered_price","weight":"1.00"}}`, "UNKNOWN_TIE_BREAK"},
	}
	for _, tc := range neg {
		t.Run(tc.name, func(t *testing.T) {
			_, err := DecodePolicy([]byte(tc.raw))
			assertDecodeCode(t, err, tc.code)
		})
	}
}

func TestValidate_InvalidNodeArity(t *testing.T) {
	raw := []byte(`{
  "schema_version":"allocation_policy_ast.v1",
  "policy_id":"p1",
  "scoring_model":"ADDITIVE_WEIGHTS",
  "root":{"kind":"NOT","args":[]}
}`)
	_, err := DecodeAndValidate(raw)
	assertDecodeCode(t, err, "INVALID_NODE_ARITY")

	raw2 := []byte(`{
  "schema_version":"allocation_policy_ast.v1",
  "policy_id":"p1",
  "scoring_model":"ADDITIVE_WEIGHTS",
  "root":{"kind":"NOT","args":[
    {"kind":"SCORE","attr":"offered_price","weight":"0.50"},
    {"kind":"SCORE","attr":"contract_fit","weight":"0.50"}
  ]}
}`)
	_, err = DecodeAndValidate(raw2)
	assertDecodeCode(t, err, "INVALID_NODE_ARITY")
}

func TestValidate_EmptyANDArgsWithWeightsViaChild(t *testing.T) {
	// Empty AND alone cannot satisfy weight sum — reject INVALID_WEIGHT_SUM.
	raw := []byte(`{
  "schema_version":"allocation_policy_ast.v1",
  "policy_id":"p1",
  "scoring_model":"ADDITIVE_WEIGHTS",
  "root":{"kind":"AND","args":[]}
}`)
	_, err := DecodeAndValidate(raw)
	assertDecodeCode(t, err, "INVALID_WEIGHT_SUM")
}

func TestValidate_IncompatibleOperandTypes(t *testing.T) {
	cases := []struct {
		name string
		raw  string
	}{
		{"GT_string", `{"schema_version":"allocation_policy_ast.v1","policy_id":"p1","scoring_model":"ADDITIVE_WEIGHTS","root":{"kind":"AND","args":[{"kind":"FILTER","op":"GT","attr":"offered_price","value":{"type":"STRING","string":"x"}},{"kind":"SCORE","attr":"offered_price","weight":"1.00"}]}}`},
		{"BETWEEN_boolean", `{"schema_version":"allocation_policy_ast.v1","policy_id":"p1","scoring_model":"ADDITIVE_WEIGHTS","root":{"kind":"AND","args":[{"kind":"FILTER","op":"BETWEEN","attr":"offered_price","values":[{"type":"BOOLEAN","boolean":false},{"type":"BOOLEAN","boolean":true}]},{"kind":"SCORE","attr":"offered_price","weight":"1.00"}]}}`},
		{"EQ_null_literal", `{"schema_version":"allocation_policy_ast.v1","policy_id":"p1","scoring_model":"ADDITIVE_WEIGHTS","root":{"kind":"AND","args":[{"kind":"FILTER","op":"EQ","attr":"contract_status","value":{"type":"NULL"}},{"kind":"SCORE","attr":"offered_price","weight":"1.00"}]}}`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := DecodeAndValidate([]byte(tc.raw))
			assertDecodeCode(t, err, "INCOMPATIBLE_TYPES")
		})
	}
}

func TestValidate_InvalidDecimalWeight(t *testing.T) {
	cases := []struct {
		name string
		w    string
		code string
	}{
		{"negative", "-0.1", "INVALID_WEIGHT"},
		{"nan", "NaN", "INVALID_DECIMAL"},
		{"inf", "Infinity", "INVALID_DECIMAL"},
		{"huge_exp", "1e309", "DECIMAL_EXPONENT_EXCEEDED"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			raw := []byte(`{
  "schema_version":"allocation_policy_ast.v1",
  "policy_id":"p1",
  "scoring_model":"ADDITIVE_WEIGHTS",
  "root":{"kind":"SCORE","attr":"offered_price","weight":"` + tc.w + `"}
}`)
			_, err := DecodeAndValidate(raw)
			if err == nil {
				t.Fatal("expected error")
			}
			de, ok := err.(*DecodeError)
			if !ok {
				t.Fatalf("want DecodeError got %T %v", err, err)
			}
			// NaN/Inf may fail at UnmarshalJSON (INVALID_DECIMAL) before validate.
			if de.Code != tc.code && de.Code != "INVALID_DECIMAL" && de.Code != "JSON_DECODE" {
				t.Fatalf("code=%s want %s (%v)", de.Code, tc.code, err)
			}
		})
	}
}

func TestValidate_NonUnitWeightSum(t *testing.T) {
	raw := []byte(`{
  "schema_version":"allocation_policy_ast.v1",
  "policy_id":"p1",
  "scoring_model":"ADDITIVE_WEIGHTS",
  "root":{"kind":"AND","args":[
    {"kind":"SCORE","attr":"offered_price","weight":"0.50"},
    {"kind":"SCORE","attr":"on_time_score","weight":"0.50"},
    {"kind":"SCORE","attr":"contract_fit","weight":"0.10"}
  ]}
}`)
	_, err := DecodeAndValidate(raw)
	assertDecodeCode(t, err, "INVALID_WEIGHT_SUM")
}

func TestValidate_MalformedJSON(t *testing.T) {
	_, err := DecodeAndValidate([]byte(`{not json`))
	if err == nil {
		t.Fatal("expected malformed reject")
	}
}

func TestValidate_ImmutableInput(t *testing.T) {
	raw := []byte(`{
  "schema_version":"allocation_policy_ast.v1",
  "policy_id":"p1",
  "scoring_model":"ADDITIVE_WEIGHTS",
  "root":{"kind":"SCORE","attr":"offered_price","weight":"1.00"}
}`)
	clone := append([]byte(nil), raw...)
	if _, err := DecodeAndValidate(raw); err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(raw, clone) {
		t.Fatal("input mutated")
	}
}

func TestValidate_Concurrent32(t *testing.T) {
	raw := []byte(`{
  "schema_version":"allocation_policy_ast.v1",
  "policy_id":"p1",
  "scoring_model":"ADDITIVE_WEIGHTS",
  "root":{"kind":"SCORE","attr":"offered_price","weight":"1.00"}
}`)
	const n = 32
	errs := make([]error, n)
	var wg sync.WaitGroup
	wg.Add(n)
	for i := 0; i < n; i++ {
		go func(i int) {
			defer wg.Done()
			_, errs[i] = DecodeAndValidate(raw)
		}(i)
	}
	wg.Wait()
	for i := 1; i < n; i++ {
		if (errs[0] == nil) != (errs[i] == nil) {
			t.Fatalf("non-deterministic outcomes: 0=%v i=%d %v", errs[0], i, errs[i])
		}
		if errs[0] != nil && errs[i] != nil && errs[0].Error() != errs[i].Error() {
			t.Fatalf("error divergence: %v vs %v", errs[0], errs[i])
		}
	}
	if errs[0] != nil {
		t.Fatalf("expected success: %v", errs[0])
	}
}

func TestValidate_DeterministicErrors(t *testing.T) {
	raw := []byte(`{
  "schema_version":"allocation_policy_ast.v1",
  "policy_id":"p1",
  "scoring_model":"NOPE",
  "root":{"kind":"SCORE","attr":"offered_price","weight":"1.00"}
}`)
	var first string
	for i := 0; i < 8; i++ {
		_, err := DecodePolicy(raw)
		if err == nil {
			t.Fatal("expected error")
		}
		if i == 0 {
			first = err.Error()
		} else if err.Error() != first {
			t.Fatalf("nondeterministic error: %q vs %q", first, err.Error())
		}
	}
}

func TestValidate_NoExternalExecutionSurface(t *testing.T) {
	raw := []byte(`{
  "schema_version":"allocation_policy_ast.v1",
  "policy_id":"p1",
  "scoring_model":"ADDITIVE_WEIGHTS",
  "shell":"rm -rf /",
  "root":{"kind":"SCORE","attr":"offered_price","weight":"1.00"}
}`)
	_, err := DecodeAndValidate(raw)
	if err == nil {
		t.Fatal("expected forbidden/unknown reject")
	}
	de := err.(*DecodeError)
	if de.Code != "FORBIDDEN_SURFACE" && de.Code != "UNKNOWN_FIELD" && de.Code != "DUPLICATE_JSON_KEY" {
		t.Fatalf("code=%s", de.Code)
	}
}

func TestValidate_InvalidActionAttr(t *testing.T) {
	raw := []byte(`{
  "schema_version":"allocation_policy_ast.v1",
  "policy_id":"p1",
  "scoring_model":"ADDITIVE_WEIGHTS",
  "root":{"kind":"AND","args":[
    {"kind":"ACTION","attr":"award_now"},
    {"kind":"SCORE","attr":"offered_price","weight":"1.00"}
  ]}
}`)
	_, err := DecodeAndValidate(raw)
	assertDecodeCode(t, err, "INVALID_ACTION_ATTR")
}

func TestValidate_EXISTSWithoutEval(t *testing.T) {
	// Structural accept of EXISTS / IS_MISSING; no EXISTS(NULL) evaluation.
	raw := []byte(`{
  "schema_version":"allocation_policy_ast.v1",
  "policy_id":"p1",
  "scoring_model":"ADDITIVE_WEIGHTS",
  "root":{"kind":"AND","args":[
    {"kind":"FILTER","op":"EXISTS","attr":"offered_price"},
    {"kind":"FILTER","op":"IS_MISSING","attr":"offered_price"},
    {"kind":"SCORE","attr":"offered_price","weight":"1.00"}
  ]}
}`)
	if _, err := DecodeAndValidate(raw); err != nil {
		t.Fatal(err)
	}
}

func TestValidate_MissingRequiredFields(t *testing.T) {
	_, err := DecodePolicy([]byte(`{
  "schema_version":"allocation_policy_ast.v1",
  "scoring_model":"ADDITIVE_WEIGHTS",
  "root":{"kind":"SCORE","attr":"offered_price","weight":"1.00"}
}`))
	assertDecodeCode(t, err, "MISSING_POLICY_ID")
}
