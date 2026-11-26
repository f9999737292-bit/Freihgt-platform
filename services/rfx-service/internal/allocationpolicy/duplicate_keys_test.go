package allocationpolicy

import "testing"

func TestDuplicateKeys_TopLevelRejected(t *testing.T) {
	raw := []byte(`{
  "schema_version":"allocation_policy_ast.v1",
  "policy_id":"a",
  "policy_id":"b",
  "scoring_model":"ADDITIVE_WEIGHTS",
  "root":{"kind":"SCORE","attr":"a","weight":"1.00"}
}`)
	_, err := DecodePolicy(raw)
	assertDecodeCode(t, err, "DUPLICATE_JSON_KEY")
}

func TestDuplicateKeys_NestedRejected(t *testing.T) {
	raw := []byte(`{
  "schema_version":"allocation_policy_ast.v1",
  "policy_id":"p1",
  "scoring_model":"ADDITIVE_WEIGHTS",
  "root":{"kind":"FILTER","kind":"ACTION","attr":"shortlist"}
}`)
	_, err := DecodePolicy(raw)
	assertDecodeCode(t, err, "DUPLICATE_JSON_KEY")
}

func TestDuplicateKeys_ArrayObjectNestedRejected(t *testing.T) {
	raw := []byte(`{
  "schema_version":"allocation_policy_ast.v1",
  "policy_id":"p1",
  "scoring_model":"ADDITIVE_WEIGHTS",
  "root":{"kind":"AND","args":[
    {"attr":"x","attr":"y","kind":"FILTER","op":"EXISTS"}
  ]}
}`)
	_, err := DecodePolicy(raw)
	assertDecodeCode(t, err, "DUPLICATE_JSON_KEY")
}

func TestDuplicateKeys_EscapedEquivalentNameRejected(t *testing.T) {
	// "policy_id" spelled with unicode escape still decodes to same key.
	raw := []byte(`{
  "schema_version":"allocation_policy_ast.v1",
  "policy_id":"a",
  "\u0070olicy_id":"b",
  "scoring_model":"ADDITIVE_WEIGHTS",
  "root":{"kind":"SCORE","attr":"a","weight":"1.00"}
}`)
	_, err := DecodePolicy(raw)
	assertDecodeCode(t, err, "DUPLICATE_JSON_KEY")
}

func TestDuplicateKeys_SameValueStillRejected(t *testing.T) {
	raw := []byte(`{
  "schema_version":"allocation_policy_ast.v1",
  "policy_id":"a",
  "policy_id":"a",
  "scoring_model":"ADDITIVE_WEIGHTS",
  "root":{"kind":"SCORE","attr":"a","weight":"1.00"}
}`)
	_, err := DecodePolicy(raw)
	assertDecodeCode(t, err, "DUPLICATE_JSON_KEY")
}
