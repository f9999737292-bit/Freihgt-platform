package allocationpolicy

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/shopspring/decimal"
)

const FingerprintVersion = "allocation_policy_fingerprint.v1"

// Fingerprint returns sha256:<hex> for a validated policy (W1_AST_SCHEMA §8 / W1-022).
// policy_id is excluded. Defaults are already applied by DecodePolicy.
func Fingerprint(vp *ValidatedPolicy) (string, error) {
	if vp == nil {
		return "", validationErr("NIL_POLICY", "validated policy is nil")
	}
	canon, err := canonicalPolicyBytes(&vp.policy)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(canon)
	return "sha256:" + hex.EncodeToString(sum[:]), nil
}

func canonicalPolicyBytes(p *Policy) ([]byte, error) {
	doc := map[string]any{
		"schema_version":    p.SchemaVersion,
		"scoring_model":     p.ScoringModel,
		"share_policy_mode": string(p.SharePolicyMode),
		"tie_break": map[string]any{
			"strategy": string(p.TieBreak.Strategy),
		},
		"root": canonicalNode(p.Root),
	}
	// encoding/json sorts map keys lexicographically; arrays preserve order.
	b, err := json.Marshal(doc)
	if err != nil {
		return nil, validationErr("FINGERPRINT_CANONICALIZE", err.Error())
	}
	return b, nil
}

func canonicalNode(n Node) map[string]any {
	out := map[string]any{
		"kind": string(n.Kind),
	}
	if n.Op != "" {
		out["op"] = string(n.Op)
	}
	if n.Attr != "" {
		out["attr"] = n.Attr
	}
	if n.NodeID != "" {
		out["node_id"] = n.NodeID
	}
	if n.Weight != nil {
		out["weight"] = CanonicalDecimalString(n.Weight.Decimal)
	}
	if n.Value != nil {
		out["value"] = canonicalValue(*n.Value)
	}
	if len(n.Values) > 0 {
		vals := make([]any, 0, len(n.Values))
		for i := range n.Values {
			vals = append(vals, canonicalValue(n.Values[i]))
		}
		out["values"] = vals
	}
	if len(n.Args) > 0 {
		args := make([]any, 0, len(n.Args))
		for i := range n.Args {
			args = append(args, canonicalNode(n.Args[i]))
		}
		out["args"] = args
	}
	return out
}

func canonicalValue(v Value) map[string]any {
	out := map[string]any{"type": string(v.Type)}
	switch v.Type {
	case ValueTypeString:
		if v.String != nil {
			out["string"] = *v.String
		}
	case ValueTypeBoolean:
		if v.Boolean != nil {
			out["boolean"] = *v.Boolean
		}
	case ValueTypeEnum:
		if v.Enum != nil {
			out["enum"] = *v.Enum
		}
	case ValueTypeTimestamp:
		if v.Timestamp != nil {
			out["timestamp"] = *v.Timestamp
		}
	case ValueTypeDecimal:
		if v.Decimal != nil {
			out["decimal"] = CanonicalDecimalString(v.Decimal.Decimal)
		}
	}
	return out
}

// CanonicalDecimalString normalizes decimals for fingerprinting:
// no exponent, no trailing zeros (except "0"), no '+' sign.
func CanonicalDecimalString(d decimal.Decimal) string {
	// After validateDecimalSafety, String() is bounded and non-scientific for our inputs.
	s := strings.TrimPrefix(d.String(), "+")
	if strings.ContainsAny(s, "eE") {
		// Bounded inputs should not be scientific; fail closed rather than invent format.
		s = d.StringFixed(18)
		s = strings.TrimPrefix(s, "+")
	}
	if strings.Contains(s, ".") {
		s = strings.TrimRight(s, "0")
		s = strings.TrimRight(s, ".")
	}
	if s == "" || s == "-0" {
		return "0"
	}
	return s
}

// FingerprintRawJSON is a convenience for tests: ParseAndValidate then Fingerprint.
func FingerprintRawJSON(raw []byte) (string, error) {
	vp, err := ParseAndValidate(raw)
	if err != nil {
		return "", err
	}
	fp, err := Fingerprint(vp)
	if err != nil {
		return "", err
	}
	if !strings.HasPrefix(fp, "sha256:") || len(fp) != len("sha256:")+64 {
		return "", fmt.Errorf("unexpected fingerprint format %q", fp)
	}
	return fp, nil
}
