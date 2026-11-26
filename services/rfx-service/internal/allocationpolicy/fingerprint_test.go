package allocationpolicy

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestFingerprint_W1022_CanonicalStability(t *testing.T) {
	compact := mustRead(t, filepath.Join("testdata", "golden", "w1_007_score.json"))
	pretty := []byte(`{
  "schema_version" : "allocation_policy_ast.v1",
  "policy_id" : "other-id",
  "scoring_model" : "ADDITIVE_WEIGHTS",
  "root" : {
    "kind" : "AND",
    "args" : [
      { "weight" : "0.40", "kind" : "SCORE", "attr" : "offered_price" },
      { "attr" : "on_time_score", "kind" : "SCORE", "weight" : "0.35" },
      { "kind" : "SCORE", "attr" : "contract_fit", "weight" : "0.25" }
    ]
  }
}`)
	fp1, err := FingerprintRawJSON(compact)
	if err != nil {
		t.Fatal(err)
	}
	fp2, err := FingerprintRawJSON(pretty)
	if err != nil {
		t.Fatal(err)
	}
	if fp1 != fp2 {
		t.Fatalf("fingerprint diverged:\n%s\n%s", fp1, fp2)
	}
	if !strings.HasPrefix(fp1, "sha256:") {
		t.Fatalf("prefix: %s", fp1)
	}
}

func TestFingerprint_DefaultsAndPolicyIDExcluded(t *testing.T) {
	a := []byte(`{"schema_version":"allocation_policy_ast.v1","policy_id":"a","scoring_model":"ADDITIVE_WEIGHTS","root":{"kind":"SCORE","attr":"offered_price","weight":"1.00"}}`)
	b := []byte(`{"schema_version":"allocation_policy_ast.v1","policy_id":"b","scoring_model":"ADDITIVE_WEIGHTS","share_policy_mode":"CAP_ONLY","tie_break":{"strategy":"CARRIER_ID_ASC"},"root":{"kind":"SCORE","attr":"offered_price","weight":"1.0"}}`)
	fpA, err := FingerprintRawJSON(a)
	if err != nil {
		t.Fatal(err)
	}
	fpB, err := FingerprintRawJSON(b)
	if err != nil {
		t.Fatal(err)
	}
	if fpA != fpB {
		t.Fatalf("defaults/policy_id/decimal normalize mismatch:\n%s\n%s", fpA, fpB)
	}
}

func TestFingerprint_MaterialChange(t *testing.T) {
	base := []byte(`{"schema_version":"allocation_policy_ast.v1","policy_id":"p","scoring_model":"ADDITIVE_WEIGHTS","root":{"kind":"AND","args":[{"kind":"SCORE","attr":"offered_price","weight":"0.50"},{"kind":"SCORE","attr":"contract_fit","weight":"0.50"}]}}`)
	changedWeight := []byte(`{"schema_version":"allocation_policy_ast.v1","policy_id":"p","scoring_model":"ADDITIVE_WEIGHTS","root":{"kind":"AND","args":[{"kind":"SCORE","attr":"offered_price","weight":"0.60"},{"kind":"SCORE","attr":"contract_fit","weight":"0.40"}]}}`)
	changedFilter := []byte(`{"schema_version":"allocation_policy_ast.v1","policy_id":"p","scoring_model":"ADDITIVE_WEIGHTS","root":{"kind":"AND","args":[{"kind":"FILTER","op":"EQ","attr":"contract_status","value":{"type":"ENUM","enum":"ACTIVE"}},{"kind":"SCORE","attr":"offered_price","weight":"0.50"},{"kind":"SCORE","attr":"contract_fit","weight":"0.50"}]}}`)
	fp0, _ := FingerprintRawJSON(base)
	fp1, _ := FingerprintRawJSON(changedWeight)
	fp2, _ := FingerprintRawJSON(changedFilter)
	if fp0 == fp1 || fp0 == fp2 {
		t.Fatalf("expected material fingerprint change")
	}
}

func TestFingerprint_ASTOrderMatters(t *testing.T) {
	a := []byte(`{"schema_version":"allocation_policy_ast.v1","policy_id":"p","scoring_model":"ADDITIVE_WEIGHTS","root":{"kind":"AND","args":[{"kind":"SCORE","attr":"offered_price","weight":"0.50"},{"kind":"SCORE","attr":"contract_fit","weight":"0.50"}]}}`)
	b := []byte(`{"schema_version":"allocation_policy_ast.v1","policy_id":"p","scoring_model":"ADDITIVE_WEIGHTS","root":{"kind":"AND","args":[{"kind":"SCORE","attr":"contract_fit","weight":"0.50"},{"kind":"SCORE","attr":"offered_price","weight":"0.50"}]}}`)
	fpA, _ := FingerprintRawJSON(a)
	fpB, _ := FingerprintRawJSON(b)
	if fpA == fpB {
		t.Fatal("AST array order must affect fingerprint")
	}
}

func mustRead(t *testing.T, path string) []byte {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return b
}
