package allocationpolicy

import (
	"path/filepath"
	"testing"

	"github.com/shopspring/decimal"
)

func nullEvalAuth(t *testing.T) EvalAuthRequest {
	t.Helper()
	return EvalAuthRequest{
		Auth:                   testAuth(t, PermAllocationSimulate),
		CampaignOwnerCompanyID: "co1",
		PolicyOwnerCompanyID:   "co1",
		ResourceStatus:         ResourceStatusDraft,
	}
}

// Status filter on carrier_id; SCORE always uses present offered_price so eligibility
// reflects only the status operator under test.
func policyStatusFilter(t *testing.T, op Operator) *ValidatedPolicy {
	t.Helper()
	raw := []byte(`{
  "schema_version":"allocation_policy_ast.v1","policy_id":"p","scoring_model":"ADDITIVE_WEIGHTS",
  "root":{"kind":"AND","args":[
    {"kind":"FILTER","op":"` + string(op) + `","attr":"carrier_id"},
    {"kind":"SCORE","attr":"offered_price","weight":"1.00"}
  ]}
}`)
	vp, err := ParseAndValidate(raw)
	if err != nil {
		t.Fatal(err)
	}
	return vp
}

func presentPrice() map[string]RuntimeValue {
	return map[string]RuntimeValue{
		"offered_price": {Status: AttrStatusPresent, Type: ValueTypeDecimal, Decimal: decimal.RequireFromString("10"), HasDecimal: true},
	}
}

func evalWithCarrierStatus(t *testing.T, vp *ValidatedPolicy, st AttrStatus) *EvaluationResult {
	t.Helper()
	attrs := presentPrice()
	switch st {
	case AttrStatusPresent:
		attrs["carrier_id"] = RuntimeValue{Status: AttrStatusPresent, Type: ValueTypeString, String: "c1"}
	case AttrStatusNull:
		attrs["carrier_id"] = RuntimeValue{Status: AttrStatusNull, Type: ValueTypeString}
	case AttrStatusNotAvailable:
		attrs["carrier_id"] = RuntimeValue{Status: AttrStatusNotAvailable}
	case AttrStatusMissing:
		// omit carrier_id key
	default:
		attrs["carrier_id"] = RuntimeValue{Status: st}
	}
	res, err := Evaluate(EvaluateRequest{
		Policy: vp, Auth: nullEvalAuth(t),
		Candidate: CandidateInput{CarrierID: "c1", TenantID: "t1", Attrs: attrs},
	})
	if err != nil {
		t.Fatalf("status=%s: %v", st, err)
	}
	return res
}

func TestNULL_001_008_ExistsIsMissingMatrix(t *testing.T) {
	cases := []struct {
		id       string
		op       Operator
		status   AttrStatus
		eligible bool
	}{
		{"NULL-001", OpEXISTS, AttrStatusPresent, true},
		{"NULL-002", OpISMissing, AttrStatusPresent, false},
		{"NULL-003", OpEXISTS, AttrStatusNull, true},
		{"NULL-004", OpISMissing, AttrStatusNull, false},
		{"NULL-005", OpEXISTS, AttrStatusMissing, false},
		{"NULL-006", OpISMissing, AttrStatusMissing, true},
		{"NULL-007", OpEXISTS, AttrStatusNotAvailable, false},
		{"NULL-008", OpISMissing, AttrStatusNotAvailable, true},
	}
	for _, tc := range cases {
		t.Run(tc.id, func(t *testing.T) {
			vp := policyStatusFilter(t, tc.op)
			res := evalWithCarrierStatus(t, vp, tc.status)
			if res.Eligible != tc.eligible {
				t.Fatalf("eligible=%v want %v", res.Eligible, tc.eligible)
			}
			if tc.eligible && res.ScoreTotal == nil {
				t.Fatal("eligible requires score")
			}
			if !tc.eligible && res.ScoreTotal != nil {
				t.Fatal("ineligible must null score")
			}
		})
	}
}

func TestNULL_009_012_ComparisonsUnknown(t *testing.T) {
	mk := func(op, extra string) *ValidatedPolicy {
		t.Helper()
		raw := []byte(`{
  "schema_version":"allocation_policy_ast.v1","policy_id":"p","scoring_model":"ADDITIVE_WEIGHTS",
  "root":{"kind":"AND","args":[
    {"kind":"FILTER","op":"` + op + `","attr":"offered_price"` + extra + `},
    {"kind":"SCORE","attr":"contract_fit","weight":"1.00"}
  ]}
}`)
		vp, err := ParseAndValidate(raw)
		if err != nil {
			t.Fatal(err)
		}
		return vp
	}
	run := func(t *testing.T, vp *ValidatedPolicy, st AttrStatus) {
		t.Helper()
		attrs := map[string]RuntimeValue{
			"contract_fit": {Status: AttrStatusPresent, Type: ValueTypeDecimal, Decimal: decimal.RequireFromString("10"), HasDecimal: true},
		}
		switch st {
		case AttrStatusNull:
			attrs["offered_price"] = RuntimeValue{Status: AttrStatusNull, Type: ValueTypeDecimal}
		case AttrStatusMissing:
			// omit
		case AttrStatusNotAvailable:
			attrs["offered_price"] = RuntimeValue{Status: AttrStatusNotAvailable}
		}
		res, err := Evaluate(EvaluateRequest{
			Policy: vp, Auth: nullEvalAuth(t),
			Candidate: CandidateInput{CarrierID: "c1", TenantID: "t1", Attrs: attrs},
		})
		if err != nil {
			t.Fatal(err)
		}
		if res.Eligible || res.ScoreTotal != nil {
			t.Fatalf("want HARD UNKNOWN ineligible for status=%s", st)
		}
	}
	t.Run("NULL-009", func(t *testing.T) {
		run(t, mk("EQ", `,"value":{"type":"DECIMAL","decimal":"1"}`), AttrStatusNull)
	})
	t.Run("NULL-010", func(t *testing.T) {
		run(t, mk("NE", `,"value":{"type":"DECIMAL","decimal":"1"}`), AttrStatusNull)
	})
	t.Run("NULL-011", func(t *testing.T) {
		run(t, mk("GT", `,"value":{"type":"DECIMAL","decimal":"1"}`), AttrStatusMissing)
	})
	t.Run("NULL-012", func(t *testing.T) {
		run(t, mk("BETWEEN", `,"values":[{"type":"DECIMAL","decimal":"0"},{"type":"DECIMAL","decimal":"100"}]`), AttrStatusNotAvailable)
	})
}

func TestNULL_013_HardUnknown(t *testing.T) {
	raw := []byte(`{
  "schema_version":"allocation_policy_ast.v1","policy_id":"p","scoring_model":"ADDITIVE_WEIGHTS",
  "root":{"kind":"AND","args":[
    {"kind":"FILTER","op":"EQ","attr":"offered_price","value":{"type":"DECIMAL","decimal":"1"}},
    {"kind":"SCORE","attr":"contract_fit","weight":"1.00"}
  ]}
}`)
	vp, err := ParseAndValidate(raw)
	if err != nil {
		t.Fatal(err)
	}
	res, err := Evaluate(EvaluateRequest{
		Policy: vp, Auth: nullEvalAuth(t),
		Candidate: CandidateInput{CarrierID: "c1", TenantID: "t1", Attrs: map[string]RuntimeValue{
			"offered_price": {Status: AttrStatusNull, Type: ValueTypeDecimal},
			"contract_fit":  {Status: AttrStatusPresent, Type: ValueTypeDecimal, Decimal: decimal.RequireFromString("10"), HasDecimal: true},
		}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if res.Eligible || res.ScoreTotal != nil {
		t.Fatal("NULL-013 HARD UNKNOWN must fail closed")
	}
}

func TestNULL_014_016_TriLogic(t *testing.T) {
	if andTri(TriFalse, TriUnknown) != TriFalse {
		t.Fatal("NULL-014")
	}
	if orTri(TriFalse, TriUnknown) != TriUnknown {
		t.Fatal("NULL-015")
	}
	if notTri(TriUnknown) != TriUnknown {
		t.Fatal("NULL-016")
	}
}

func TestNULL_017_KPINotFabricated(t *testing.T) {
	raw := []byte(`{
  "schema_version":"allocation_policy_ast.v1","policy_id":"p","scoring_model":"ADDITIVE_WEIGHTS",
  "root":{"kind":"AND","args":[
    {"kind":"FILTER","op":"EXISTS","attr":"lane_kpi"},
    {"kind":"SCORE","attr":"offered_price","weight":"1.00"}
  ]}
}`)
	vp, err := ParseAndValidate(raw)
	if err != nil {
		t.Fatal(err)
	}
	res, err := Evaluate(EvaluateRequest{
		Policy: vp, Auth: nullEvalAuth(t),
		Candidate: CandidateInput{CarrierID: "c1", TenantID: "t1", Attrs: map[string]RuntimeValue{
			"lane_kpi":      {Status: AttrStatusPresent, Type: ValueTypeDecimal, Decimal: decimal.RequireFromString("99"), HasDecimal: true},
			"offered_price": {Status: AttrStatusPresent, Type: ValueTypeDecimal, Decimal: decimal.RequireFromString("10"), HasDecimal: true},
		}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if res.Eligible || res.ScoreTotal != nil {
		t.Fatal("catalog NOT_AVAILABLE must override injected PRESENT")
	}
}

func TestNULL_018_TenantIsolation(t *testing.T) {
	vp := policyStatusFilter(t, OpEXISTS)
	_, err := Evaluate(EvaluateRequest{
		Policy: vp, Auth: nullEvalAuth(t),
		Candidate: CandidateInput{CarrierID: "c1", TenantID: "other", Attrs: presentPrice()},
	})
	assertDecodeCode(t, err, "TENANT_MISMATCH")
}

func TestNULL_019_PermissionDenied(t *testing.T) {
	vp := policyStatusFilter(t, OpEXISTS)
	auth := nullEvalAuth(t)
	auth.Auth.Permissions = map[string]struct{}{PermTenderRead: {}}
	_, err := Evaluate(EvaluateRequest{
		Policy: vp, Auth: auth,
		Candidate: CandidateInput{CarrierID: "c1", TenantID: "t1", Attrs: presentPrice()},
	})
	assertDecodeCode(t, err, "DRAFT_DENIED")
}

func TestNULL_020_ExplanationDeterministic(t *testing.T) {
	vp := policyStatusFilter(t, OpEXISTS)
	res := evalWithCarrierStatus(t, vp, AttrStatusPresent)
	if res.Explanation.PolicyFingerprint == "" {
		t.Fatal("missing fingerprint")
	}
	fp, err := Fingerprint(vp)
	if err != nil || fp != res.Explanation.PolicyFingerprint {
		t.Fatalf("fp mismatch")
	}
}

func TestNULL_021_OR_FilterScore_StillRejected(t *testing.T) {
	_, err := DecodeAndValidate([]byte(`{
  "schema_version":"allocation_policy_ast.v1","policy_id":"p","scoring_model":"ADDITIVE_WEIGHTS",
  "root":{"kind":"OR","args":[
    {"kind":"FILTER","op":"EXISTS","attr":"offered_price"},
    {"kind":"SCORE","attr":"offered_price","weight":"1.00"}
  ]}
}`))
	assertDecodeCode(t, err, "MIXED_HARD_NONHARD_BOOLEAN_TREE")
}

func TestNULL_022_FingerprintUnchanged(t *testing.T) {
	raw := mustRead(t, filepath.Join("testdata", "golden", "w1_007_score.json"))
	fp1, err := FingerprintRawJSON(raw)
	if err != nil {
		t.Fatal(err)
	}
	fp2, err := FingerprintRawJSON(raw)
	if err != nil || fp1 != fp2 {
		t.Fatal("fingerprint changed")
	}
}

func TestNULL_023_ValidatedPolicyImmutability(t *testing.T) {
	raw := mustRead(t, filepath.Join("testdata", "golden", "w1_007_score.json"))
	vp, err := ParseAndValidate(raw)
	if err != nil {
		t.Fatal(err)
	}
	fpBefore, _ := Fingerprint(vp)
	ex := vp.Policy()
	ex.Root.Args[0].Attr = "hacked"
	fpAfter, _ := Fingerprint(vp)
	if fpBefore != fpAfter {
		t.Fatal("mutation leaked")
	}
}

func TestNULL_024_InvalidAttrStatus(t *testing.T) {
	vp := policyStatusFilter(t, OpEXISTS)
	attrs := presentPrice()
	attrs["carrier_id"] = RuntimeValue{Status: AttrStatus("WEIRD")}
	_, err := Evaluate(EvaluateRequest{
		Policy: vp, Auth: nullEvalAuth(t),
		Candidate: CandidateInput{CarrierID: "c1", TenantID: "t1", Attrs: attrs},
	})
	assertDecodeCode(t, err, "INVALID_ATTRIBUTE_STATUS")
}

func TestNULL_GoldenScoreStill78(t *testing.T) {
	raw := mustRead(t, filepath.Join("testdata", "golden", "w1_007_score.json"))
	vp, err := ParseAndValidate(raw)
	if err != nil {
		t.Fatal(err)
	}
	res, err := Evaluate(EvaluateRequest{
		Policy: vp, Auth: nullEvalAuth(t),
		Candidate: CandidateInput{CarrierID: "c1", TenantID: "t1", Attrs: map[string]RuntimeValue{
			"offered_price": {Status: AttrStatusPresent, Type: ValueTypeDecimal, Decimal: decimal.RequireFromString("80"), HasDecimal: true},
			"on_time_score": {Status: AttrStatusPresent, Type: ValueTypeDecimal, Decimal: decimal.RequireFromString("60"), HasDecimal: true},
			"contract_fit":  {Status: AttrStatusPresent, Type: ValueTypeDecimal, Decimal: decimal.RequireFromString("100"), HasDecimal: true},
		}},
	})
	if err != nil {
		t.Fatal(err)
	}
	want := decimal.RequireFromString("78.00")
	if !res.ScoreTotal.Equal(want) {
		t.Fatalf("got %s", res.ScoreTotal)
	}
}
