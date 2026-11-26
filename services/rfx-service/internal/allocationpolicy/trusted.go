package allocationpolicy

// ValidatedPolicy is the only policy handle accepted by Fingerprint / Evaluate.
// It closes J-W1.2-G003: DecodePolicy output alone cannot enter evaluation.
type ValidatedPolicy struct {
	policy Policy
}

// Policy returns a copy of the validated policy document.
func (v *ValidatedPolicy) Policy() Policy {
	if v == nil {
		return Policy{}
	}
	return v.policy
}

// ParseAndValidate decodes + validates untrusted JSON into a ValidatedPolicy.
func ParseAndValidate(raw []byte) (*ValidatedPolicy, error) {
	p, err := DecodeAndValidate(raw)
	if err != nil {
		return nil, err
	}
	return &ValidatedPolicy{policy: *p}, nil
}

// AdoptValidatedPolicy re-validates a constructed Policy before trust elevation.
func AdoptValidatedPolicy(p *Policy) (*ValidatedPolicy, error) {
	if p == nil {
		return nil, validationErr("NIL_POLICY", "policy is nil")
	}
	// Copy then validate so caller cannot mutate after adoption without re-validate.
	cp := *p
	if err := ValidatePolicy(&cp); err != nil {
		return nil, err
	}
	return &ValidatedPolicy{policy: cp}, nil
}
