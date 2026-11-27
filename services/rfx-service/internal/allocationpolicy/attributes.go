package allocationpolicy

import "fmt"

// Attribute availability / evidence mode.
type AttributeAvailability string

const (
	AttrAvailable     AttributeAvailability = "AVAILABLE"
	AttrNotAvailable  AttributeAvailability = "NOT_AVAILABLE"
	AttrTestOnly      AttributeAvailability = "TEST_ONLY"
)

// AttributeSource classifies where values may come from.
type AttributeSource string

const (
	SourceServerOwned AttributeSource = "SERVER_OWNED"
	SourceCandidate   AttributeSource = "CANDIDATE"
	SourcePolicyMeta  AttributeSource = "POLICY_META"
	SourceTestFixture AttributeSource = "TEST_FIXTURE"
)

// AttributeSpec is the fail-closed allowlist entry (J-W1.2-G002 / W1-010).
type AttributeSpec struct {
	Name         string
	Type         ValueType
	AllowedOps   map[Operator]struct{}
	Source       AttributeSource
	TenantScoped bool
	Availability AttributeAvailability
	HardAllowed  bool
	ScoreAllowed bool
	TestOnly     bool
}

var attributeCatalog = map[string]AttributeSpec{}

func init() {
	register := func(s AttributeSpec, ops ...Operator) {
		s.AllowedOps = make(map[Operator]struct{}, len(ops))
		for _, op := range ops {
			s.AllowedOps[op] = struct{}{}
		}
		attributeCatalog[s.Name] = s
	}

	cmpAll := []Operator{OpEQ, OpNE, OpGT, OpGTE, OpLT, OpLTE, OpIN, OpNOTIN, OpBETWEEN}
	eqSet := []Operator{OpEQ, OpNE, OpIN, OpNOTIN}
	// EXISTS/IS_MISSING allowed; runtime status table is CONTROLLER_EXISTS_NULL_FREEZE=YES.
	presence := []Operator{OpEXISTS, OpISMissing}

	register(AttributeSpec{
		Name: "tenant_id", Type: ValueTypeString, Source: SourceServerOwned,
		TenantScoped: true, Availability: AttrAvailable, HardAllowed: true,
	}, append(eqSet, presence...)...)

	register(AttributeSpec{
		Name: "buyer_company_id", Type: ValueTypeString, Source: SourceServerOwned,
		TenantScoped: true, Availability: AttrAvailable, HardAllowed: true,
	}, append(eqSet, presence...)...)

	register(AttributeSpec{
		Name: "carrier_company_id", Type: ValueTypeString, Source: SourceCandidate,
		TenantScoped: true, Availability: AttrAvailable, HardAllowed: true, ScoreAllowed: true,
	}, append(eqSet, presence...)...)

	register(AttributeSpec{
		Name: "carrier_id", Type: ValueTypeString, Source: SourceCandidate,
		TenantScoped: true, Availability: AttrAvailable, HardAllowed: true, ScoreAllowed: true,
	}, append(eqSet, presence...)...)

	register(AttributeSpec{
		Name: "campaign_id", Type: ValueTypeString, Source: SourceServerOwned,
		TenantScoped: true, Availability: AttrAvailable, HardAllowed: true,
	}, append(eqSet, presence...)...)

	register(AttributeSpec{
		Name: "contract_status", Type: ValueTypeEnum, Source: SourceCandidate,
		TenantScoped: true, Availability: AttrAvailable, HardAllowed: true,
	}, append(eqSet, presence...)...)

	register(AttributeSpec{
		Name: "offered_price", Type: ValueTypeDecimal, Source: SourceCandidate,
		TenantScoped: true, Availability: AttrAvailable, HardAllowed: true, ScoreAllowed: true,
	}, append(cmpAll, presence...)...)

	register(AttributeSpec{
		Name: "contract_fit", Type: ValueTypeDecimal, Source: SourceCandidate,
		TenantScoped: true, Availability: AttrAvailable, ScoreAllowed: true,
	}, append(cmpAll, presence...)...)

	register(AttributeSpec{
		Name: "on_time_score", Type: ValueTypeDecimal, Source: SourceCandidate,
		TenantScoped: true, Availability: AttrAvailable, ScoreAllowed: true, HardAllowed: true,
	}, append(cmpAll, presence...)...)

	register(AttributeSpec{
		Name: "share_pct", Type: ValueTypeDecimal, Source: SourceCandidate,
		TenantScoped: true, Availability: AttrAvailable, HardAllowed: true,
	}, append(cmpAll, presence...)...)

	// KPI without confirmed production evidence — NOT_AVAILABLE (W1 honesty).
	register(AttributeSpec{
		Name: "lane_kpi", Type: ValueTypeDecimal, Source: SourceCandidate,
		TenantScoped: true, Availability: AttrNotAvailable, ScoreAllowed: true, HardAllowed: true,
	}, append(cmpAll, presence...)...)

	register(AttributeSpec{
		Name: "overall_kpi", Type: ValueTypeDecimal, Source: SourceCandidate,
		TenantScoped: true, Availability: AttrNotAvailable, ScoreAllowed: true, HardAllowed: true,
	}, append(cmpAll, presence...)...)

	// Explicit TEST_ONLY synthetic component for golden fixtures.
	register(AttributeSpec{
		Name: "test_component_score", Type: ValueTypeDecimal, Source: SourceTestFixture,
		TenantScoped: true, Availability: AttrTestOnly, ScoreAllowed: true, TestOnly: true,
	}, append(cmpAll, presence...)...)

	register(AttributeSpec{
		Name: "shortlist", Type: ValueTypeBoolean, Source: SourcePolicyMeta,
		Availability: AttrAvailable,
	}, OpEQ, OpNE)

	register(AttributeSpec{
		Name: "reserve", Type: ValueTypeBoolean, Source: SourcePolicyMeta,
		Availability: AttrAvailable,
	}, OpEQ, OpNE)

	register(AttributeSpec{
		Name: "exception", Type: ValueTypeBoolean, Source: SourcePolicyMeta,
		Availability: AttrAvailable,
	}, OpEQ, OpNE)
}

// LookupAttribute returns the catalog spec or false when unknown (deny).
func LookupAttribute(name string) (AttributeSpec, bool) {
	s, ok := attributeCatalog[name]
	return s, ok
}

func validateAttributeUsage(n *Node) error {
	spec, ok := LookupAttribute(n.Attr)
	if !ok {
		return validationErr("UNKNOWN_ATTRIBUTE",
			fmt.Sprintf("attribute %q is not in the allowlist (UNKNOWN_ATTRIBUTE_POLICY=DENY)", n.Attr))
	}
	switch n.Kind {
	case NodeKindFilter, NodeKindFloor, NodeKindCap:
		if !spec.HardAllowed && n.Kind != NodeKindCap {
			// CAP uses HardAllowed; FLOOR/FILTER require HardAllowed.
		}
		if (n.Kind == NodeKindFilter || n.Kind == NodeKindFloor) && !spec.HardAllowed {
			return validationErr("ATTRIBUTE_NOT_ALLOWED",
				fmt.Sprintf("attribute %q not allowed in %s", n.Attr, n.Kind))
		}
		if n.Kind == NodeKindCap && !spec.HardAllowed {
			return validationErr("ATTRIBUTE_NOT_ALLOWED",
				fmt.Sprintf("attribute %q not allowed in CAP", n.Attr))
		}
	case NodeKindScore:
		if !spec.ScoreAllowed {
			return validationErr("ATTRIBUTE_NOT_ALLOWED",
				fmt.Sprintf("attribute %q not allowed in SCORE", n.Attr))
		}
	}
	if n.Op != "" {
		if _, ok := spec.AllowedOps[n.Op]; !ok {
			return validationErr("ATTRIBUTE_OPERATOR_DENIED",
				fmt.Sprintf("operator %s not allowed on attribute %q", n.Op, n.Attr))
		}
	}
	return nil
}
