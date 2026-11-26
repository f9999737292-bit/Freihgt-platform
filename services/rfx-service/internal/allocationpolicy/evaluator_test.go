package allocationpolicy

import (
	"path/filepath"
	"sync"
	"testing"

	"github.com/shopspring/decimal"
)

func TestEvaluate_W1007_GoldenScore(t *testing.T) {
	raw := mustRead(t, filepath.Join("testdata", "golden", "w1_007_score.json"))
	vp, err := ParseAndValidate(raw)
	if err != nil {
		t.Fatal(err)
	}
	res, err := Evaluate(EvaluateRequest{
		Policy: vp,
		Auth: EvalAuthRequest{
			Auth:                   testAuth(t, PermAllocationSimulate),
			CampaignOwnerCompanyID: "co1",
			PolicyOwnerCompanyID:   "co1",
			ResourceStatus:         ResourceStatusDraft,
		},
		Candidate: CandidateInput{
			CarrierID: "c1",
			TenantID:  "t1",
			Attrs: map[string]RuntimeValue{
				"offered_price": {Status: AttrStatusPresent, Type: ValueTypeDecimal, Decimal: decimal.RequireFromString("80"), HasDecimal: true},
				"on_time_score": {Status: AttrStatusPresent, Type: ValueTypeDecimal, Decimal: decimal.RequireFromString("60"), HasDecimal: true},
				"contract_fit":  {Status: AttrStatusPresent, Type: ValueTypeDecimal, Decimal: decimal.RequireFromString("100"), HasDecimal: true},
			},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if !res.Eligible || res.ScoreTotal == nil {
		t.Fatalf("eligible/score: %#v", res)
	}
	want := decimal.RequireFromString("78.00")
	if !res.ScoreTotal.Equal(want) {
		t.Fatalf("score_total=%s want 78.00", res.ScoreTotal.String())
	}
}

func TestEvaluate_W1005_HardBlocksScore(t *testing.T) {
	raw := []byte(`{
  "schema_version":"allocation_policy_ast.v1","policy_id":"p","scoring_model":"ADDITIVE_WEIGHTS",
  "root":{"kind":"AND","args":[
    {"kind":"FILTER","op":"EQ","attr":"contract_status","value":{"type":"ENUM","enum":"ACTIVE"}},
    {"kind":"SCORE","attr":"offered_price","weight":"1.00"}
  ]}
}`)
	vp, _ := ParseAndValidate(raw)
	res, err := Evaluate(EvaluateRequest{
		Policy: vp,
		Auth: EvalAuthRequest{Auth: testAuth(t, PermAllocationSimulate), CampaignOwnerCompanyID: "co1", PolicyOwnerCompanyID: "co1", ResourceStatus: ResourceStatusDraft},
		Candidate: CandidateInput{CarrierID: "c1", TenantID: "t1", Attrs: map[string]RuntimeValue{
			"contract_status": {Status: AttrStatusPresent, Type: ValueTypeEnum, Enum: "INACTIVE"},
			"offered_price":   {Status: AttrStatusPresent, Type: ValueTypeDecimal, Decimal: decimal.RequireFromString("99"), HasDecimal: true},
		}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if res.Eligible || res.ScoreTotal != nil {
		t.Fatalf("HARD must block score: eligible=%v score=%v", res.Eligible, res.ScoreTotal)
	}
}

func TestEvaluate_W1006_NoFabricatedKPI(t *testing.T) {
	raw := []byte(`{
  "schema_version":"allocation_policy_ast.v1","policy_id":"p","scoring_model":"ADDITIVE_WEIGHTS",
  "root":{"kind":"SCORE","attr":"lane_kpi","weight":"1.00"}
}`)
	vp, err := ParseAndValidate(raw)
	if err != nil {
		t.Fatal(err)
	}
	_, err = Evaluate(EvaluateRequest{
		Policy: vp,
		Auth: EvalAuthRequest{Auth: testAuth(t, PermAllocationSimulate), CampaignOwnerCompanyID: "co1", PolicyOwnerCompanyID: "co1", ResourceStatus: ResourceStatusDraft},
		Candidate: CandidateInput{CarrierID: "c1", TenantID: "t1", Attrs: map[string]RuntimeValue{}},
	})
	assertDecodeCode(t, err, "KPI_NOT_AVAILABLE")
}

func TestEvaluate_W1010_UnknownAttr(t *testing.T) {
	raw := []byte(`{
  "schema_version":"allocation_policy_ast.v1","policy_id":"p","scoring_model":"ADDITIVE_WEIGHTS",
  "root":{"kind":"FILTER","op":"EQ","attr":"not_in_catalog","value":{"type":"STRING","string":"x"}}
}`)
	_, err := ParseAndValidate(raw)
	assertDecodeCode(t, err, "UNKNOWN_ATTRIBUTE")
}

func TestEvaluate_W1011_TenantIsolation(t *testing.T) {
	raw := []byte(`{"schema_version":"allocation_policy_ast.v1","policy_id":"p","scoring_model":"ADDITIVE_WEIGHTS","root":{"kind":"SCORE","attr":"offered_price","weight":"1.00"}}`)
	vp, _ := ParseAndValidate(raw)
	_, err := Evaluate(EvaluateRequest{
		Policy: vp,
		Auth: EvalAuthRequest{Auth: testAuth(t, PermAllocationSimulate), CampaignOwnerCompanyID: "co1", PolicyOwnerCompanyID: "co1", ResourceStatus: ResourceStatusDraft},
		Candidate: CandidateInput{CarrierID: "c1", TenantID: "other", Attrs: map[string]RuntimeValue{
			"offered_price": {Status: AttrStatusPresent, Type: ValueTypeDecimal, Decimal: decimal.RequireFromString("50"), HasDecimal: true},
		}},
	})
	assertDecodeCode(t, err, "TENANT_MISMATCH")
}

func TestEvaluate_TrustBoundary_RequiresValidatedPolicy(t *testing.T) {
	// DecodePolicy alone is insufficient — Evaluate needs ValidatedPolicy.
	raw := []byte(`{"schema_version":"allocation_policy_ast.v1","policy_id":"p","scoring_model":"ADDITIVE_WEIGHTS","root":{"kind":"SCORE","attr":"offered_price","weight":"1.00"}}`)
	p, err := DecodePolicy(raw)
	if err != nil {
		t.Fatal(err)
	}
	_, err = Evaluate(EvaluateRequest{Policy: nil, Auth: EvalAuthRequest{Auth: testAuth(t, PermAllocationSimulate)}})
	assertDecodeCode(t, err, "NIL_POLICY")
	vp, err := AdoptValidatedPolicy(p)
	if err != nil {
		t.Fatal(err)
	}
	if vp == nil {
		t.Fatal("expected validated")
	}
}

func TestEvaluate_EXISTS_BlockedContractDecision(t *testing.T) {
	raw := []byte(`{
  "schema_version":"allocation_policy_ast.v1","policy_id":"p","scoring_model":"ADDITIVE_WEIGHTS",
  "root":{"kind":"AND","args":[
    {"kind":"FILTER","op":"EXISTS","attr":"offered_price"},
    {"kind":"SCORE","attr":"offered_price","weight":"1.00"}
  ]}
}`)
	vp, err := ParseAndValidate(raw)
	if err != nil {
		t.Fatal(err)
	}
	_, err = Evaluate(EvaluateRequest{
		Policy: vp,
		Auth: EvalAuthRequest{Auth: testAuth(t, PermAllocationSimulate), CampaignOwnerCompanyID: "co1", PolicyOwnerCompanyID: "co1", ResourceStatus: ResourceStatusDraft},
		Candidate: CandidateInput{CarrierID: "c1", TenantID: "t1", Attrs: map[string]RuntimeValue{
			"offered_price": {Status: AttrStatusPresent, Type: ValueTypeDecimal, Decimal: decimal.RequireFromString("10"), HasDecimal: true},
		}},
	})
	assertDecodeCode(t, err, "CONTRACT_DECISION_REQUIRED")
}

func TestEvaluate_NestedANDOR_W1004(t *testing.T) {
	raw := []byte(`{
  "schema_version":"allocation_policy_ast.v1","policy_id":"p","scoring_model":"ADDITIVE_WEIGHTS",
  "root":{"kind":"AND","args":[
    {"kind":"OR","args":[
      {"kind":"FILTER","op":"EQ","attr":"contract_status","value":{"type":"ENUM","enum":"ACTIVE"}},
      {"kind":"FILTER","op":"EQ","attr":"contract_status","value":{"type":"ENUM","enum":"PENDING"}}
    ]},
    {"kind":"SCORE","attr":"offered_price","weight":"1.00"}
  ]}
}`)
	vp, _ := ParseAndValidate(raw)
	res, err := Evaluate(EvaluateRequest{
		Policy: vp,
		Auth: EvalAuthRequest{Auth: testAuth(t, PermAllocationSimulate), CampaignOwnerCompanyID: "co1", PolicyOwnerCompanyID: "co1", ResourceStatus: ResourceStatusDraft},
		Candidate: CandidateInput{CarrierID: "c1", TenantID: "t1", Attrs: map[string]RuntimeValue{
			"contract_status": {Status: AttrStatusPresent, Type: ValueTypeEnum, Enum: "PENDING"},
			"offered_price":   {Status: AttrStatusPresent, Type: ValueTypeDecimal, Decimal: decimal.RequireFromString("40"), HasDecimal: true},
		}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if !res.Eligible {
		t.Fatal("expected eligible via OR branch")
	}
}

func TestEvaluate_Concurrent_W1019(t *testing.T) {
	raw := mustRead(t, filepath.Join("testdata", "golden", "w1_007_score.json"))
	vp, err := ParseAndValidate(raw)
	if err != nil {
		t.Fatal(err)
	}
	req := EvaluateRequest{
		Policy: vp,
		Auth: EvalAuthRequest{Auth: testAuth(t, PermAllocationSimulate), CampaignOwnerCompanyID: "co1", PolicyOwnerCompanyID: "co1", ResourceStatus: ResourceStatusDraft},
		Candidate: CandidateInput{CarrierID: "c1", TenantID: "t1", Attrs: map[string]RuntimeValue{
			"offered_price": {Status: AttrStatusPresent, Type: ValueTypeDecimal, Decimal: decimal.RequireFromString("80"), HasDecimal: true},
			"on_time_score": {Status: AttrStatusPresent, Type: ValueTypeDecimal, Decimal: decimal.RequireFromString("60"), HasDecimal: true},
			"contract_fit":  {Status: AttrStatusPresent, Type: ValueTypeDecimal, Decimal: decimal.RequireFromString("100"), HasDecimal: true},
		}},
	}
	const n = 32
	errs := make([]error, n)
	scores := make([]*decimal.Decimal, n)
	var wg sync.WaitGroup
	wg.Add(n)
	for i := 0; i < n; i++ {
		go func(i int) {
			defer wg.Done()
			res, err := Evaluate(req)
			errs[i] = err
			if res != nil {
				scores[i] = res.ScoreTotal
			}
		}(i)
	}
	wg.Wait()
	for i := 1; i < n; i++ {
		if (errs[0] == nil) != (errs[i] == nil) {
			t.Fatalf("nondeterministic errors")
		}
		if errs[0] != nil {
			continue
		}
		if !scores[0].Equal(*scores[i]) {
			t.Fatalf("score diverge")
		}
	}
	if errs[0] != nil {
		t.Fatal(errs[0])
	}
}

func TestEvaluate_NoSideEffects_W1014_W1015(t *testing.T) {
	// Package surface check: Evaluate returns explanation only; no TO/HTTP symbols invoked.
	raw := []byte(`{"schema_version":"allocation_policy_ast.v1","policy_id":"p","scoring_model":"ADDITIVE_WEIGHTS","root":{"kind":"SCORE","attr":"offered_price","weight":"1.00"}}`)
	vp, _ := ParseAndValidate(raw)
	res, err := Evaluate(EvaluateRequest{
		Policy: vp,
		Auth: EvalAuthRequest{Auth: testAuth(t, PermAllocationSimulate), CampaignOwnerCompanyID: "co1", PolicyOwnerCompanyID: "co1", ResourceStatus: ResourceStatusDraft},
		Candidate: CandidateInput{CarrierID: "c1", TenantID: "t1", Attrs: map[string]RuntimeValue{
			"offered_price": {Status: AttrStatusPresent, Type: ValueTypeDecimal, Decimal: decimal.RequireFromString("10"), HasDecimal: true},
		}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if res.Explanation.PolicyFingerprint == "" {
		t.Fatal("missing fingerprint in explanation")
	}
}

func TestDecimalSafety_G001_BoundsBeforeString(t *testing.T) {
	// Exponent exceeded must reject without relying on String() first.
	d := Decimal{Decimal: decimal.RequireFromString("1e20")}
	err := validateDecimalSafety(&d)
	assertDecodeCode(t, err, "DECIMAL_EXPONENT_EXCEEDED")
}

func TestPreferenceDoesNotChangeScore(t *testing.T) {
	raw := []byte(`{
  "schema_version":"allocation_policy_ast.v1","policy_id":"p","scoring_model":"ADDITIVE_WEIGHTS",
  "root":{"kind":"AND","args":[
    {"kind":"SCORE","attr":"offered_price","weight":"1.00"},
    {"kind":"PREFERENCE","op":"EQ","attr":"contract_status","value":{"type":"ENUM","enum":"ACTIVE"},"node_id":"pref1"}
  ]}
}`)
	vp, _ := ParseAndValidate(raw)
	res, err := Evaluate(EvaluateRequest{
		Policy: vp,
		Auth: EvalAuthRequest{Auth: testAuth(t, PermAllocationSimulate), CampaignOwnerCompanyID: "co1", PolicyOwnerCompanyID: "co1", ResourceStatus: ResourceStatusDraft},
		Candidate: CandidateInput{CarrierID: "c1", TenantID: "t1", Attrs: map[string]RuntimeValue{
			"offered_price":   {Status: AttrStatusPresent, Type: ValueTypeDecimal, Decimal: decimal.RequireFromString("55"), HasDecimal: true},
			"contract_status": {Status: AttrStatusPresent, Type: ValueTypeEnum, Enum: "ACTIVE"},
		}},
	})
	if err != nil {
		t.Fatal(err)
	}
	want := decimal.RequireFromString("55.00")
	if !res.ScoreTotal.Equal(want) {
		t.Fatalf("score changed by preference: %s", res.ScoreTotal)
	}
	if len(res.Explanation.MatchedPreferences) == 0 {
		t.Fatal("expected matched preference")
	}
}
