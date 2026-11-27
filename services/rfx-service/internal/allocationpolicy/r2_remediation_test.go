package allocationpolicy

import (
	"sync"
	"testing"

	"github.com/shopspring/decimal"
)

func TestW1R2_001_OR_FilterFalse_Score_Rejected(t *testing.T) {
	raw := []byte(`{
  "schema_version":"allocation_policy_ast.v1","policy_id":"p","scoring_model":"ADDITIVE_WEIGHTS",
  "root":{"kind":"OR","args":[
    {"kind":"FILTER","op":"EQ","attr":"contract_status","value":{"type":"ENUM","enum":"ACTIVE"}},
    {"kind":"SCORE","attr":"offered_price","weight":"1.00"}
  ]}
}`)
	_, err := DecodeAndValidate(raw)
	assertDecodeCode(t, err, "MIXED_HARD_NONHARD_BOOLEAN_TREE")
}

func TestW1R2_002_OR_Cap_Preference_Rejected(t *testing.T) {
	raw := []byte(`{
  "schema_version":"allocation_policy_ast.v1","policy_id":"p","scoring_model":"ADDITIVE_WEIGHTS",
  "root":{"kind":"AND","args":[
    {"kind":"OR","args":[
      {"kind":"CAP","attr":"share_pct","op":"LTE","value":{"type":"DECIMAL","decimal":"0.5"}},
      {"kind":"PREFERENCE","op":"EQ","attr":"contract_status","value":{"type":"ENUM","enum":"ACTIVE"}}
    ]},
    {"kind":"SCORE","attr":"offered_price","weight":"1.00"}
  ]}
}`)
	_, err := DecodeAndValidate(raw)
	assertDecodeCode(t, err, "MIXED_HARD_NONHARD_BOOLEAN_TREE")
}

func TestW1R2_003_NOT_Score_Rejected(t *testing.T) {
	_, err := DecodeAndValidate([]byte(`{
  "schema_version":"allocation_policy_ast.v1","policy_id":"p","scoring_model":"ADDITIVE_WEIGHTS",
  "root":{"kind":"AND","args":[
    {"kind":"NOT","args":[{"kind":"SCORE","attr":"offered_price","weight":"0"}]},
    {"kind":"SCORE","attr":"contract_fit","weight":"1.00"}
  ]}
}`))
	assertDecodeCode(t, err, "MIXED_HARD_NONHARD_BOOLEAN_TREE")
}

func TestW1R2_004_AND_FilterFalse_Score_Ineligible(t *testing.T) {
	raw := []byte(`{
  "schema_version":"allocation_policy_ast.v1","policy_id":"p","scoring_model":"ADDITIVE_WEIGHTS",
  "root":{"kind":"AND","args":[
    {"kind":"FILTER","op":"EQ","attr":"contract_status","value":{"type":"ENUM","enum":"ACTIVE"}},
    {"kind":"SCORE","attr":"offered_price","weight":"1.00"}
  ]}
}`)
	vp, err := ParseAndValidate(raw)
	if err != nil {
		t.Fatal(err)
	}
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
		t.Fatalf("want ineligible+nil score, got eligible=%v score=%v", res.Eligible, res.ScoreTotal)
	}
}

func TestW1R2_005_006_PureHardOR(t *testing.T) {
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
	vp, err := ParseAndValidate(raw)
	if err != nil {
		t.Fatal(err)
	}
	auth := EvalAuthRequest{Auth: testAuth(t, PermAllocationSimulate), CampaignOwnerCompanyID: "co1", PolicyOwnerCompanyID: "co1", ResourceStatus: ResourceStatusDraft}
	resTrue, err := Evaluate(EvaluateRequest{Policy: vp, Auth: auth, Candidate: CandidateInput{CarrierID: "c1", TenantID: "t1", Attrs: map[string]RuntimeValue{
		"contract_status": {Status: AttrStatusPresent, Type: ValueTypeEnum, Enum: "ACTIVE"},
		"offered_price":   {Status: AttrStatusPresent, Type: ValueTypeDecimal, Decimal: decimal.RequireFromString("10"), HasDecimal: true},
	}}})
	if err != nil || !resTrue.Eligible {
		t.Fatalf("OR true branch: err=%v eligible=%v", err, resTrue != nil && resTrue.Eligible)
	}
	resFalse, err := Evaluate(EvaluateRequest{Policy: vp, Auth: auth, Candidate: CandidateInput{CarrierID: "c1", TenantID: "t1", Attrs: map[string]RuntimeValue{
		"contract_status": {Status: AttrStatusPresent, Type: ValueTypeEnum, Enum: "INACTIVE"},
		"offered_price":   {Status: AttrStatusPresent, Type: ValueTypeDecimal, Decimal: decimal.RequireFromString("10"), HasDecimal: true},
	}}})
	if err != nil {
		t.Fatal(err)
	}
	if resFalse.Eligible || resFalse.ScoreTotal != nil {
		t.Fatalf("OR both false must be ineligible")
	}
}

func TestW1R2_007_OR_NestedAND_WithScore_Rejected(t *testing.T) {
	raw := []byte(`{
  "schema_version":"allocation_policy_ast.v1","policy_id":"p","scoring_model":"ADDITIVE_WEIGHTS",
  "root":{"kind":"OR","args":[
    {"kind":"FILTER","op":"EQ","attr":"contract_status","value":{"type":"ENUM","enum":"ACTIVE"}},
    {"kind":"AND","args":[
      {"kind":"FILTER","op":"EQ","attr":"contract_status","value":{"type":"ENUM","enum":"PENDING"}},
      {"kind":"SCORE","attr":"offered_price","weight":"1.00"}
    ]}
  ]}
}`)
	_, err := DecodeAndValidate(raw)
	assertDecodeCode(t, err, "MIXED_HARD_NONHARD_BOOLEAN_TREE")
}

func TestW1R2_008_NOT_AND_FilterScore_Rejected(t *testing.T) {
	raw := []byte(`{
  "schema_version":"allocation_policy_ast.v1","policy_id":"p","scoring_model":"ADDITIVE_WEIGHTS",
  "root":{"kind":"AND","args":[
    {"kind":"NOT","args":[{
      "kind":"AND","args":[
        {"kind":"FILTER","op":"EQ","attr":"contract_status","value":{"type":"ENUM","enum":"ACTIVE"}},
        {"kind":"SCORE","attr":"offered_price","weight":"1.00"}
      ]
    }]}
  ]}
}`)
	_, err := DecodeAndValidate(raw)
	assertDecodeCode(t, err, "MIXED_HARD_NONHARD_BOOLEAN_TREE")
}

func TestW1R2_009_HardUnknown_NoBypass(t *testing.T) {
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
			// missing contract_status → UNKNOWN HARD
			"offered_price": {Status: AttrStatusPresent, Type: ValueTypeDecimal, Decimal: decimal.RequireFromString("99"), HasDecimal: true},
		}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if res.Eligible || res.ScoreTotal != nil {
		t.Fatal("HARD UNKNOWN must not be bypassed by SCORE")
	}
}

func TestW1R2_010_015_Immutability(t *testing.T) {
	raw := []byte(`{
  "schema_version":"allocation_policy_ast.v1","policy_id":"p","scoring_model":"ADDITIVE_WEIGHTS",
  "root":{"kind":"AND","args":[
    {"kind":"FILTER","op":"EQ","attr":"contract_status","value":{"type":"ENUM","enum":"ACTIVE"}},
    {"kind":"SCORE","attr":"offered_price","weight":"1.00"}
  ]}
}`)
	p, err := DecodeAndValidate(raw)
	if err != nil {
		t.Fatal(err)
	}
	vp, err := AdoptValidatedPolicy(p)
	if err != nil {
		t.Fatal(err)
	}
	fpBefore, err := Fingerprint(vp)
	if err != nil {
		t.Fatal(err)
	}
	auth := EvalAuthRequest{Auth: testAuth(t, PermAllocationSimulate), CampaignOwnerCompanyID: "co1", PolicyOwnerCompanyID: "co1", ResourceStatus: ResourceStatusDraft}
	cand := CandidateInput{CarrierID: "c1", TenantID: "t1", Attrs: map[string]RuntimeValue{
		"contract_status": {Status: AttrStatusPresent, Type: ValueTypeEnum, Enum: "ACTIVE"},
		"offered_price":   {Status: AttrStatusPresent, Type: ValueTypeDecimal, Decimal: decimal.RequireFromString("40"), HasDecimal: true},
	}}
	resBefore, err := Evaluate(EvaluateRequest{Policy: vp, Auth: auth, Candidate: cand})
	if err != nil {
		t.Fatal(err)
	}

	// Mutate original decoded policy.
	p.Root.Args[0].Attr = "carrier_id"
	p.Root.Args[1].Weight = &Decimal{Decimal: decimal.RequireFromString("0.5")}
	if p.Root.Args[0].Value != nil && p.Root.Args[0].Value.Enum != nil {
		*p.Root.Args[0].Value.Enum = "HACKED"
	}

	// Mutate exported Policy() copy.
	exported := vp.Policy()
	exported.Root.Args = append(exported.Root.Args, Node{Kind: NodeKindScore, Attr: "contract_fit", Weight: &Decimal{Decimal: decimal.RequireFromString("0")}})
	if exported.Root.Args[1].Weight != nil {
		exported.Root.Args[1].Weight.Decimal = decimal.RequireFromString("0.01")
	}
	if exported.Root.Args[0].Value != nil {
		exported.Root.Args[0].Value.Type = ValueTypeString
	}

	fpAfter, err := Fingerprint(vp)
	if err != nil {
		t.Fatal(err)
	}
	if fpBefore != fpAfter {
		t.Fatalf("fingerprint changed after external mutation:\n%s\n%s", fpBefore, fpAfter)
	}
	resAfter, err := Evaluate(EvaluateRequest{Policy: vp, Auth: auth, Candidate: cand})
	if err != nil {
		t.Fatal(err)
	}
	if resBefore.Eligible != resAfter.Eligible || !resBefore.ScoreTotal.Equal(*resAfter.ScoreTotal) {
		t.Fatalf("evaluation changed after mutation")
	}
	if resBefore.Explanation.PolicyFingerprint != resAfter.Explanation.PolicyFingerprint {
		t.Fatal("explanation fingerprint drift")
	}
}

func TestW1R2_016_ConcurrentReadOnly(t *testing.T) {
	raw := mustRead(t, "testdata/golden/w1_007_score.json")
	vp, err := ParseAndValidate(raw)
	if err != nil {
		t.Fatal(err)
	}
	auth := EvalAuthRequest{Auth: testAuth(t, PermAllocationSimulate), CampaignOwnerCompanyID: "co1", PolicyOwnerCompanyID: "co1", ResourceStatus: ResourceStatusDraft}
	cand := CandidateInput{CarrierID: "c1", TenantID: "t1", Attrs: map[string]RuntimeValue{
		"offered_price": {Status: AttrStatusPresent, Type: ValueTypeDecimal, Decimal: decimal.RequireFromString("80"), HasDecimal: true},
		"on_time_score": {Status: AttrStatusPresent, Type: ValueTypeDecimal, Decimal: decimal.RequireFromString("60"), HasDecimal: true},
		"contract_fit":  {Status: AttrStatusPresent, Type: ValueTypeDecimal, Decimal: decimal.RequireFromString("100"), HasDecimal: true},
	}}
	const n = 32
	var wg sync.WaitGroup
	wg.Add(n)
	errs := make([]error, n)
	for i := 0; i < n; i++ {
		go func(i int) {
			defer wg.Done()
			if i%2 == 0 {
				_, errs[i] = Fingerprint(vp)
			} else {
				_, errs[i] = Evaluate(EvaluateRequest{Policy: vp, Auth: auth, Candidate: cand})
			}
		}(i)
	}
	wg.Wait()
	for i, err := range errs {
		if err != nil {
			t.Fatalf("goroutine %d: %v", i, err)
		}
	}
}

func TestW1R2_OR_Fallback_Rejected(t *testing.T) {
	raw := []byte(`{
  "schema_version":"allocation_policy_ast.v1","policy_id":"p","scoring_model":"ADDITIVE_WEIGHTS",
  "root":{"kind":"AND","args":[
    {"kind":"OR","args":[
      {"kind":"FILTER","op":"EQ","attr":"contract_status","value":{"type":"ENUM","enum":"ACTIVE"}},
      {"kind":"FALLBACK","attr":"offered_price","op":"EQ","value":{"type":"DECIMAL","decimal":"1"}}
    ]},
    {"kind":"SCORE","attr":"offered_price","weight":"1.00"}
  ]}
}`)
	_, err := DecodeAndValidate(raw)
	assertDecodeCode(t, err, "MIXED_HARD_NONHARD_BOOLEAN_TREE")
}
