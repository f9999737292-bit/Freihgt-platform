package domain

// CaseSeverityInput aggregates health and linked-work signals for deterministic severity.
type CaseSeverityInput struct {
	Health            CaseHealth
	HasP1Exception    bool
	HasP2Exception    bool
	HasCriticalImpact bool
	HasCriticalRisk   bool
	HasHighRisk       bool
}

// DeriveCaseSeverity returns the deterministic derived severity from health and signals.
// Higher severity always wins; manual override is applied separately as effective severity.
func DeriveCaseSeverity(in CaseSeverityInput) string {
	if (in.HasP1Exception && in.Health.HasSLABreach) || (in.HasCriticalImpact && in.HasP1Exception) {
		return CaseSeverityCritical
	}
	if in.HasP1Exception || in.Health.HasSLABreach || in.HasCriticalRisk {
		return CaseSeverityHigh
	}
	if in.HasP2Exception || in.Health.HasSLAWarning || in.HasHighRisk {
		return CaseSeverityMedium
	}
	return CaseSeverityLow
}
