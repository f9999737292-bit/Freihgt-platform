package allocationpolicy

import "github.com/shopspring/decimal"

// ValidatedPolicy is the only policy handle accepted by Fingerprint / Evaluate.
// It closes J-W1.2-G003: DecodePolicy output alone cannot enter evaluation.
// Internal storage is a deep-isolated snapshot (J-W1.3-H002).
type ValidatedPolicy struct {
	policy Policy
}

// Policy returns an independent deep copy of the validated policy document.
func (v *ValidatedPolicy) Policy() Policy {
	if v == nil {
		return Policy{}
	}
	return clonePolicy(v.policy)
}

// ParseAndValidate decodes + validates untrusted JSON into a ValidatedPolicy.
func ParseAndValidate(raw []byte) (*ValidatedPolicy, error) {
	p, err := DecodeAndValidate(raw)
	if err != nil {
		return nil, err
	}
	return &ValidatedPolicy{policy: clonePolicy(*p)}, nil
}

// AdoptValidatedPolicy re-validates a constructed Policy before trust elevation.
// The caller's Policy and the returned ValidatedPolicy do not share nested pointers.
func AdoptValidatedPolicy(p *Policy) (*ValidatedPolicy, error) {
	if p == nil {
		return nil, validationErr("NIL_POLICY", "policy is nil")
	}
	cp := clonePolicy(*p)
	if err := ValidatePolicy(&cp); err != nil {
		return nil, err
	}
	// Validate may have mutated nothing, but re-clone so validation cannot retain aliases.
	return &ValidatedPolicy{policy: clonePolicy(cp)}, nil
}

func clonePolicy(p Policy) Policy {
	return Policy{
		SchemaVersion:   p.SchemaVersion,
		PolicyID:        p.PolicyID,
		ScoringModel:    p.ScoringModel,
		SharePolicyMode: p.SharePolicyMode,
		TieBreak:        p.TieBreak,
		Root:            cloneNode(p.Root),
	}
}

func cloneNode(n Node) Node {
	out := Node{
		Kind:   n.Kind,
		Op:     n.Op,
		Attr:   n.Attr,
		NodeID: n.NodeID,
	}
	if n.Weight != nil {
		out.Weight = cloneDecimalPtr(n.Weight)
	}
	if n.Value != nil {
		v := cloneValue(*n.Value)
		out.Value = &v
	}
	if len(n.Values) > 0 {
		out.Values = make([]Value, len(n.Values))
		for i := range n.Values {
			out.Values[i] = cloneValue(n.Values[i])
		}
	}
	if len(n.Args) > 0 {
		out.Args = make([]Node, len(n.Args))
		for i := range n.Args {
			out.Args[i] = cloneNode(n.Args[i])
		}
	}
	return out
}

func cloneValue(v Value) Value {
	out := Value{Type: v.Type}
	if v.String != nil {
		s := *v.String
		out.String = &s
	}
	if v.Boolean != nil {
		b := *v.Boolean
		out.Boolean = &b
	}
	if v.Enum != nil {
		s := *v.Enum
		out.Enum = &s
	}
	if v.Timestamp != nil {
		s := *v.Timestamp
		out.Timestamp = &s
	}
	if v.Decimal != nil {
		out.Decimal = cloneDecimalPtr(v.Decimal)
	}
	return out
}

func cloneDecimalPtr(d *Decimal) *Decimal {
	if d == nil {
		return nil
	}
	// shopspring Decimal.Copy duplicates coefficient big.Int storage.
	cp := Decimal{Decimal: d.Copy()}
	// Ensure we do not retain alias if Copy is value-shallow on some versions:
	if cp.Coefficient() != nil && d.Coefficient() != nil && cp.Coefficient() == d.Coefficient() {
		rebuild := decimal.NewFromBigInt(d.Coefficient(), d.Exponent())
		cp = Decimal{Decimal: rebuild}
	}
	return &cp
}
