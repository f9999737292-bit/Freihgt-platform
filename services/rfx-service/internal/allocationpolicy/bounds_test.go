package allocationpolicy

import (
	"fmt"
	"strings"
	"testing"
)

func TestBounds_DepthExactly16Accepted(t *testing.T) {
	raw := nestedANDPolicy(16)
	if _, err := DecodeAndValidate(raw); err != nil {
		t.Fatalf("depth 16 must pass: %v", err)
	}
}

func TestBounds_Depth17Rejected(t *testing.T) {
	raw := nestedANDPolicy(17)
	_, err := DecodeAndValidate(raw)
	assertDecodeCode(t, err, "AST_DEPTH_EXCEEDED")
}

func TestBounds_NodesExactly256Accepted(t *testing.T) {
	raw := manySCOREPolicy(255) // root AND + 255 SCORE = 256 nodes; weights sum 1
	if _, err := DecodeAndValidate(raw); err != nil {
		t.Fatalf("256 nodes must pass: %v", err)
	}
}

func TestBounds_Nodes257Rejected(t *testing.T) {
	raw := manySCOREPolicy(256) // AND + 256 SCORE = 257
	_, err := DecodeAndValidate(raw)
	assertDecodeCode(t, err, "AST_NODE_LIMIT_EXCEEDED")
}

func TestBounds_INSet64Accepted(t *testing.T) {
	vals := make([]string, 0, 64)
	for i := 0; i < 64; i++ {
		vals = append(vals, fmt.Sprintf(`{"type":"STRING","string":"v%d"}`, i))
	}
	raw := []byte(fmt.Sprintf(`{
  "schema_version":"allocation_policy_ast.v1",
  "policy_id":"p1",
  "scoring_model":"ADDITIVE_WEIGHTS",
  "root":{"kind":"AND","args":[
    {"kind":"FILTER","op":"IN","attr":"code","values":[%s]},
    {"kind":"SCORE","attr":"a","weight":"1.00"}
  ]}
}`, strings.Join(vals, ",")))
	if _, err := DecodeAndValidate(raw); err != nil {
		t.Fatalf("IN 64 must pass: %v", err)
	}
}

func TestBounds_INSet65Rejected(t *testing.T) {
	vals := make([]string, 0, 65)
	for i := 0; i < 65; i++ {
		vals = append(vals, fmt.Sprintf(`{"type":"STRING","string":"v%d"}`, i))
	}
	raw := []byte(fmt.Sprintf(`{
  "schema_version":"allocation_policy_ast.v1",
  "policy_id":"p1",
  "scoring_model":"ADDITIVE_WEIGHTS",
  "root":{"kind":"AND","args":[
    {"kind":"FILTER","op":"IN","attr":"code","values":[%s]},
    {"kind":"SCORE","attr":"a","weight":"1.00"}
  ]}
}`, strings.Join(vals, ",")))
	_, err := DecodeAndValidate(raw)
	assertDecodeCode(t, err, "IN_SET_SIZE_EXCEEDED")
}

func TestBounds_String256Accepted(t *testing.T) {
	s := strings.Repeat("a", 256)
	raw := []byte(fmt.Sprintf(`{
  "schema_version":"allocation_policy_ast.v1",
  "policy_id":"p1",
  "scoring_model":"ADDITIVE_WEIGHTS",
  "root":{"kind":"AND","args":[
    {"kind":"FILTER","op":"EQ","attr":"name","value":{"type":"STRING","string":"%s"}},
    {"kind":"SCORE","attr":"a","weight":"1.00"}
  ]}
}`, s))
	if _, err := DecodeAndValidate(raw); err != nil {
		t.Fatalf("string 256 must pass: %v", err)
	}
}

func TestBounds_String257Rejected(t *testing.T) {
	s := strings.Repeat("a", 257)
	raw := []byte(fmt.Sprintf(`{
  "schema_version":"allocation_policy_ast.v1",
  "policy_id":"p1",
  "scoring_model":"ADDITIVE_WEIGHTS",
  "root":{"kind":"AND","args":[
    {"kind":"FILTER","op":"EQ","attr":"name","value":{"type":"STRING","string":"%s"}},
    {"kind":"SCORE","attr":"a","weight":"1.00"}
  ]}
}`, s))
	_, err := DecodeAndValidate(raw)
	assertDecodeCode(t, err, "STRING_LENGTH_EXCEEDED")
}

// nestedANDPolicy builds depth N: root is depth 1; each nested AND adds 1.
// At the deepest leaf a SCORE weight 1.00 satisfies ADDITIVE_WEIGHTS.
func nestedANDPolicy(depth int) []byte {
	if depth < 1 {
		depth = 1
	}
	var b strings.Builder
	b.WriteString(`{"schema_version":"allocation_policy_ast.v1","policy_id":"p1","scoring_model":"ADDITIVE_WEIGHTS","root":`)
	for i := 1; i < depth; i++ {
		b.WriteString(`{"kind":"AND","args":[`)
	}
	b.WriteString(`{"kind":"SCORE","attr":"a","weight":"1.00"}`)
	for i := 1; i < depth; i++ {
		b.WriteString(`]}`)
	}
	b.WriteString(`}`)
	return []byte(b.String())
}

// manySCOREPolicy builds AND with n SCORE children of equal weight summing to 1.
func manySCOREPolicy(n int) []byte {
	if n <= 0 {
		n = 1
	}
	// Exact decimal split: first n-1 get floor share; last gets remainder to sum 1.
	// Use weight "0" for all but last? Better: each gets 1/n via string if n divides.
	// For arbitrary n use shopspring in builder via equal parts of 1/n truncated —
	// simplest: one SCORE weight 1.00 and n-1 SCORE weight 0 — but 0 weights allowed
	// (non-negative). Sum still 1.00.
	parts := make([]string, 0, n)
	for i := 0; i < n-1; i++ {
		parts = append(parts, fmt.Sprintf(`{"kind":"SCORE","attr":"a%d","weight":"0"}`, i))
	}
	parts = append(parts, fmt.Sprintf(`{"kind":"SCORE","attr":"a%d","weight":"1.00"}`, n-1))
	return []byte(fmt.Sprintf(`{
  "schema_version":"allocation_policy_ast.v1",
  "policy_id":"p1",
  "scoring_model":"ADDITIVE_WEIGHTS",
  "root":{"kind":"AND","args":[%s]}
}`, strings.Join(parts, ",")))
}
