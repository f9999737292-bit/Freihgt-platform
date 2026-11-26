package allocationpolicy

import (
	"fmt"
	"sort"
	"strings"

	"github.com/shopspring/decimal"
)

// Tri is three-valued logic (W1_AST_SCHEMA §6.1).
type Tri int

const (
	TriFalse Tri = iota
	TriTrue
	TriUnknown
)

func (t Tri) String() string {
	switch t {
	case TriTrue:
		return "TRUE"
	case TriFalse:
		return "FALSE"
	default:
		return "UNKNOWN"
	}
}

func andTri(a, b Tri) Tri {
	if a == TriFalse || b == TriFalse {
		return TriFalse
	}
	if a == TriUnknown || b == TriUnknown {
		return TriUnknown
	}
	return TriTrue
}

func orTri(a, b Tri) Tri {
	if a == TriTrue || b == TriTrue {
		return TriTrue
	}
	if a == TriUnknown || b == TriUnknown {
		return TriUnknown
	}
	return TriFalse
}

func notTri(a Tri) Tri {
	switch a {
	case TriTrue:
		return TriFalse
	case TriFalse:
		return TriTrue
	default:
		return TriUnknown
	}
}

// AttrStatus models runtime attribute presence (not fabricated KPI).
type AttrStatus string

const (
	AttrStatusPresent      AttrStatus = "PRESENT"
	AttrStatusMissing      AttrStatus = "MISSING"
	AttrStatusNull         AttrStatus = "NULL"
	AttrStatusNotAvailable AttrStatus = "NOT_AVAILABLE"
)

// RuntimeValue is a candidate attribute binding for evaluation.
type RuntimeValue struct {
	Status    AttrStatus
	Type      ValueType
	String    string
	Boolean   bool
	Enum      string
	Timestamp string
	Decimal   decimal.Decimal
	HasDecimal bool
}

// CandidateInput is one carrier candidate under evaluation.
type CandidateInput struct {
	CarrierID string
	TenantID  string
	Attrs     map[string]RuntimeValue
}

// EvaluateRequest is the only supported evaluation entry (J-W1.2-G003).
type EvaluateRequest struct {
	Policy *ValidatedPolicy
	Auth   EvalAuthRequest
	Candidate CandidateInput
	AllowTestOnlyAttrs bool
}

// Evaluate runs the W1.3 in-process engine. No SQL/HTTP/shell/Award/TMS/EDO.
func Evaluate(req EvaluateRequest) (*EvaluationResult, error) {
	if req.Policy == nil {
		return nil, validationErr("NIL_POLICY", "ValidatedPolicy is required")
	}
	req.Auth.CandidateTenantID = req.Candidate.TenantID
	if err := authorizeEvaluate(req.Auth); err != nil {
		return nil, err
	}

	fp, err := Fingerprint(req.Policy)
	if err != nil {
		return nil, err
	}

	st := &evalState{
		policy:             &req.Policy.policy,
		attrs:              cloneAttrs(req.Candidate.Attrs),
		allowTestOnly:      req.AllowTestOnlyAttrs,
		hardFailures:       nil,
		unavailable:        nil,
		matchedPrefs:       nil,
		appliedCAP:         nil,
		reasons:            nil,
		scoreParts:         nil,
		preferenceRank:     int(^uint(0) >> 1), // +inf sentinel as MaxInt
	}

	// Collect FALLBACK rewrites first (do not fabricate KPI for NOT_AVAILABLE catalog attrs).
	if err := st.applyFallbacks(&req.Policy.policy.Root); err != nil {
		return nil, err
	}

	hard, err := st.evalHard(&req.Policy.policy.Root)
	if err != nil {
		return nil, err
	}
	eligible := hard == TriTrue
	res := &EvaluationResult{
		Eligible:       eligible,
		CarrierID:      req.Candidate.CarrierID,
		PreferenceRank: st.preferenceRank,
		Explanation: Explanation{
			PolicyFingerprint:  fp,
			Eligible:           eligible,
			HardFailures:       append([]string(nil), st.hardFailures...),
			UnavailableAttrs:   append([]string(nil), st.unavailable...),
			MatchedPreferences: append([]string(nil), st.matchedPrefs...),
			AppliedCAP:         append([]string(nil), st.appliedCAP...),
			ReasonCodes:        append([]string(nil), st.reasons...),
			PreferenceRank:     st.preferenceRank,
		},
	}
	sort.Strings(res.Explanation.HardFailures)
	sort.Strings(res.Explanation.UnavailableAttrs)
	sort.Strings(res.Explanation.MatchedPreferences)
	sort.Strings(res.Explanation.AppliedCAP)
	sort.Strings(res.Explanation.ReasonCodes)

	if !eligible {
		res.ScoreTotal = nil
		res.Explanation.ScoreTotal = nil
		res.Explanation.ReasonCodes = appendUnique(res.Explanation.ReasonCodes, "HARD_INELIGIBLE")
		return res, nil
	}

	if err := st.computeScores(&req.Policy.policy.Root); err != nil {
		return nil, err
	}
	total := RoundScoreTotal(st.scoreRaw)
	res.ScoreTotal = &total
	res.Explanation.ScoreTotal = &total
	res.Explanation.ScoreParts = append([]ScorePart(nil), st.scoreParts...)

	if err := st.collectPreferences(&req.Policy.policy.Root); err != nil {
		return nil, err
	}
	res.PreferenceRank = st.preferenceRank
	res.Explanation.PreferenceRank = st.preferenceRank
	res.Explanation.MatchedPreferences = append([]string(nil), st.matchedPrefs...)
	sort.Strings(res.Explanation.MatchedPreferences)
	return res, nil
}

// RankCandidates sorts eligible candidates by (score DESC, preference_rank ASC, carrier_id ASC).
func RankCandidates(results []*EvaluationResult) []*EvaluationResult {
	out := append([]*EvaluationResult(nil), results...)
	sort.SliceStable(out, func(i, j int) bool {
		a, b := out[i], out[j]
		if a.Eligible != b.Eligible {
			return a.Eligible && !b.Eligible
		}
		if !a.Eligible {
			return a.CarrierID < b.CarrierID
		}
		sa, sb := scoreOrMin(a.ScoreTotal), scoreOrMin(b.ScoreTotal)
		if !sa.Equal(sb) {
			return sa.GreaterThan(sb)
		}
		if a.PreferenceRank != b.PreferenceRank {
			return a.PreferenceRank < b.PreferenceRank
		}
		return a.CarrierID < b.CarrierID
	})
	return out
}

func scoreOrMin(d *decimal.Decimal) decimal.Decimal {
	if d == nil {
		return decimal.RequireFromString("-1")
	}
	return *d
}

type evalState struct {
	policy         *Policy
	attrs          map[string]RuntimeValue
	allowTestOnly  bool
	steps          int
	hardFailures   []string
	unavailable    []string
	matchedPrefs   []string
	appliedCAP     []string
	reasons        []string
	scoreParts     []ScorePart
	scoreRaw       decimal.Decimal
	preferenceRank int
	prefIndex      int
}

func (st *evalState) step() error {
	st.steps++
	if st.steps > MaxEvaluationSteps {
		return validationErr("EVALUATION_BUDGET_EXCEEDED",
			fmt.Sprintf("evaluation exceeded %d steps", MaxEvaluationSteps))
	}
	return nil
}

func (st *evalState) applyFallbacks(n *Node) error {
	if err := st.step(); err != nil {
		return err
	}
	switch n.Kind {
	case NodeKindAnd, NodeKindOr, NodeKindNot:
		for i := range n.Args {
			if err := st.applyFallbacks(&n.Args[i]); err != nil {
				return err
			}
		}
	case NodeKindFallback:
		if n.Attr == "" || n.Value == nil {
			return nil
		}
		cur, ok := st.attrs[n.Attr]
		if !ok || cur.Status == AttrStatusMissing {
			rv, err := literalToRuntime(*n.Value)
			if err != nil {
				return err
			}
			spec, ok := LookupAttribute(n.Attr)
			if ok && spec.Availability == AttrNotAvailable {
				// Never fabricate production KPI via FALLBACK.
				st.unavailable = appendUnique(st.unavailable, n.Attr)
				st.reasons = appendUnique(st.reasons, "FALLBACK_BLOCKED_NOT_AVAILABLE")
				return nil
			}
			st.attrs[n.Attr] = rv
		}
	}
	return nil
}

func (st *evalState) evalHard(n *Node) (Tri, error) {
	if err := st.step(); err != nil {
		return TriUnknown, err
	}
	switch n.Kind {
	case NodeKindAnd:
		acc := TriTrue
		for i := range n.Args {
			// Skip non-HARD siblings under AND — they must never prove eligibility.
			if isNonHardLeaf(n.Args[i].Kind) {
				continue
			}
			t, err := st.evalHard(&n.Args[i])
			if err != nil {
				return TriUnknown, err
			}
			acc = andTri(acc, t)
		}
		return acc, nil
	case NodeKindOr:
		// Defensive: OR must be pure HARD (also enforced in ValidatePolicy).
		if err := validateBooleanHardTree(n, true); err != nil {
			return TriUnknown, err
		}
		acc := TriFalse
		for i := range n.Args {
			t, err := st.evalHard(&n.Args[i])
			if err != nil {
				return TriUnknown, err
			}
			acc = orTri(acc, t)
		}
		return acc, nil
	case NodeKindNot:
		if err := validateBooleanHardTree(n, true); err != nil {
			return TriUnknown, err
		}
		t, err := st.evalHard(&n.Args[0])
		if err != nil {
			return TriUnknown, err
		}
		return notTri(t), nil
	case NodeKindFilter, NodeKindFloor, NodeKindCap:
		t, err := st.evalComparison(n)
		if err != nil {
			return TriUnknown, err
		}
		if n.Kind == NodeKindCap {
			st.appliedCAP = appendUnique(st.appliedCAP, n.Attr)
		}
		if t != TriTrue {
			id := n.NodeID
			if id == "" {
				id = string(n.Kind) + ":" + n.Attr + ":" + string(n.Op)
			}
			st.hardFailures = appendUnique(st.hardFailures, id)
			if t == TriUnknown {
				st.reasons = appendUnique(st.reasons, "HARD_UNKNOWN")
			}
		}
		return t, nil
	case NodeKindScore, NodeKindPreference, NodeKindFallback, NodeKindApproval, NodeKindAction:
		// Non-HARD leaves are AND-neutral (skipped by parent AND) / root-no-HARD.
		// They must never appear under OR/NOT — enforced by ValidatePolicy + OR/NOT guards above.
		// Returning TRUE here must not be reachable under OR/NOT after those guards.
		return TriTrue, nil
	default:
		return TriUnknown, validationErr("UNKNOWN_NODE_KIND", string(n.Kind))
	}
}

func isNonHardLeaf(k NodeKind) bool {
	switch k {
	case NodeKindScore, NodeKindPreference, NodeKindFallback, NodeKindApproval, NodeKindAction:
		return true
	default:
		return false
	}
}

func (st *evalState) evalComparison(n *Node) (Tri, error) {
	if n.Op == OpEXISTS || n.Op == OpISMissing {
		return TriUnknown, validationErr("CONTRACT_DECISION_REQUIRED",
			"EXISTS/IS_MISSING evaluation blocked until CONTROLLER_EXISTS_NULL_FREEZE")
	}
	spec, ok := LookupAttribute(n.Attr)
	if !ok {
		return TriUnknown, validationErr("UNKNOWN_ATTRIBUTE", n.Attr)
	}
	if spec.TestOnly && !st.allowTestOnly {
		return TriUnknown, validationErr("TEST_ONLY_ATTRIBUTE", n.Attr)
	}
	if spec.Availability == AttrNotAvailable {
		st.unavailable = appendUnique(st.unavailable, n.Attr)
		return TriUnknown, nil
	}

	rv, ok := st.attrs[n.Attr]
	if !ok {
		rv = RuntimeValue{Status: AttrStatusMissing}
	}
	switch rv.Status {
	case AttrStatusMissing, AttrStatusNotAvailable:
		st.unavailable = appendUnique(st.unavailable, n.Attr)
		return TriUnknown, nil
	case AttrStatusNull:
		// Disputed EXISTS(NULL)/compare-on-NULL semantics — fail closed without inventing TRUE/FALSE.
		return TriUnknown, validationErr("CONTRACT_DECISION_REQUIRED",
			"NULL attribute comparison blocked until CONTROLLER_EXISTS_NULL_FREEZE")
	}

	switch n.Op {
	case OpEQ, OpNE:
		okCmp, err := compareEqual(rv, n.Value)
		if err != nil {
			return TriUnknown, err
		}
		if n.Op == OpNE {
			okCmp = !okCmp
		}
		if okCmp {
			return TriTrue, nil
		}
		return TriFalse, nil
	case OpGT, OpGTE, OpLT, OpLTE:
		cmp, err := compareOrdered(rv, n.Value)
		if err != nil {
			return TriUnknown, err
		}
		switch n.Op {
		case OpGT:
			if cmp > 0 {
				return TriTrue, nil
			}
		case OpGTE:
			if cmp >= 0 {
				return TriTrue, nil
			}
		case OpLT:
			if cmp < 0 {
				return TriTrue, nil
			}
		case OpLTE:
			if cmp <= 0 {
				return TriTrue, nil
			}
		}
		return TriFalse, nil
	case OpIN, OpNOTIN:
		found, err := inSet(rv, n.Values)
		if err != nil {
			return TriUnknown, err
		}
		if n.Op == OpNOTIN {
			found = !found
		}
		if found {
			return TriTrue, nil
		}
		return TriFalse, nil
	case OpBETWEEN:
		if len(n.Values) != 2 {
			return TriUnknown, validationErr("INVALID_NODE_ARITY", "BETWEEN needs 2 values")
		}
		c1, err := compareOrdered(rv, &n.Values[0])
		if err != nil {
			return TriUnknown, err
		}
		c2, err := compareOrdered(rv, &n.Values[1])
		if err != nil {
			return TriUnknown, err
		}
		if c1 >= 0 && c2 <= 0 {
			return TriTrue, nil
		}
		return TriFalse, nil
	default:
		return TriUnknown, validationErr("UNKNOWN_OPERATOR", string(n.Op))
	}
}

func (st *evalState) computeScores(n *Node) error {
	if err := st.step(); err != nil {
		return err
	}
	switch n.Kind {
	case NodeKindAnd, NodeKindOr, NodeKindNot:
		for i := range n.Args {
			if err := st.computeScores(&n.Args[i]); err != nil {
				return err
			}
		}
	case NodeKindScore:
		if n.Weight == nil {
			return validationErr("MISSING_WEIGHT", "SCORE weight required")
		}
		spec, ok := LookupAttribute(n.Attr)
		if !ok {
			return validationErr("UNKNOWN_ATTRIBUTE", n.Attr)
		}
		if spec.Availability == AttrNotAvailable {
			st.unavailable = appendUnique(st.unavailable, n.Attr)
			return validationErr("KPI_NOT_AVAILABLE",
				fmt.Sprintf("attribute %q is NOT_AVAILABLE; refusing synthetic score", n.Attr))
		}
		rv, ok := st.attrs[n.Attr]
		if !ok || rv.Status != AttrStatusPresent || !rv.HasDecimal {
			return validationErr("MISSING_COMPONENT_SCORE",
				fmt.Sprintf("SCORE attribute %q missing present decimal component", n.Attr))
		}
		if err := validateComponentScore(rv.Decimal); err != nil {
			return err
		}
		part := n.Weight.Decimal.Mul(rv.Decimal)
		st.scoreRaw = st.scoreRaw.Add(part)
		st.scoreParts = append(st.scoreParts, ScorePart{
			Attr:   n.Attr,
			Weight: n.Weight.Decimal,
			Value:  rv.Decimal,
			Part:   part,
			NodeID: n.NodeID,
		})
	}
	return nil
}

func (st *evalState) collectPreferences(n *Node) error {
	if err := st.step(); err != nil {
		return err
	}
	switch n.Kind {
	case NodeKindAnd, NodeKindOr, NodeKindNot:
		for i := range n.Args {
			if err := st.collectPreferences(&n.Args[i]); err != nil {
				return err
			}
		}
	case NodeKindPreference:
		t, err := st.evalComparison(n)
		if err != nil {
			return err
		}
		idx := st.prefIndex
		st.prefIndex++
		if t == TriTrue {
			id := n.NodeID
			if id == "" {
				id = fmt.Sprintf("pref:%d:%s", idx, n.Attr)
			}
			st.matchedPrefs = append(st.matchedPrefs, id)
			if idx < st.preferenceRank {
				st.preferenceRank = idx
			}
		}
	}
	return nil
}

func cloneAttrs(in map[string]RuntimeValue) map[string]RuntimeValue {
	out := make(map[string]RuntimeValue, len(in))
	for k, v := range in {
		out[k] = v
	}
	return out
}

func appendUnique(ss []string, v string) []string {
	for _, x := range ss {
		if x == v {
			return ss
		}
	}
	return append(ss, v)
}

func literalToRuntime(v Value) (RuntimeValue, error) {
	switch v.Type {
	case ValueTypeString:
		return RuntimeValue{Status: AttrStatusPresent, Type: v.Type, String: derefStr(v.String)}, nil
	case ValueTypeBoolean:
		return RuntimeValue{Status: AttrStatusPresent, Type: v.Type, Boolean: derefBool(v.Boolean)}, nil
	case ValueTypeEnum:
		return RuntimeValue{Status: AttrStatusPresent, Type: v.Type, Enum: derefStr(v.Enum)}, nil
	case ValueTypeTimestamp:
		return RuntimeValue{Status: AttrStatusPresent, Type: v.Type, Timestamp: derefStr(v.Timestamp)}, nil
	case ValueTypeDecimal:
		if v.Decimal == nil {
			return RuntimeValue{}, validationErr("INVALID_VALUE_SHAPE", "decimal missing")
		}
		return RuntimeValue{Status: AttrStatusPresent, Type: v.Type, Decimal: v.Decimal.Decimal, HasDecimal: true}, nil
	default:
		return RuntimeValue{}, validationErr("INCOMPATIBLE_TYPES", "fallback literal type not supported")
	}
}

func derefStr(p *string) string {
	if p == nil {
		return ""
	}
	return *p
}
func derefBool(p *bool) bool {
	if p == nil {
		return false
	}
	return *p
}

func compareEqual(rv RuntimeValue, lit *Value) (bool, error) {
	if lit == nil {
		return false, validationErr("MISSING_VALUE", "comparison value required")
	}
	if rv.Type != lit.Type && !(rv.Type == "" && lit.Type != "") {
		// Prefer literal type as expected; require match when runtime type set.
		if rv.Type != "" && rv.Type != lit.Type {
			return false, validationErr("INCOMPATIBLE_TYPES",
				fmt.Sprintf("cannot compare %s with %s", rv.Type, lit.Type))
		}
	}
	switch lit.Type {
	case ValueTypeString:
		return rv.String == derefStr(lit.String), nil
	case ValueTypeBoolean:
		return rv.Boolean == derefBool(lit.Boolean), nil
	case ValueTypeEnum:
		return rv.Enum == derefStr(lit.Enum), nil
	case ValueTypeTimestamp:
		return rv.Timestamp == derefStr(lit.Timestamp), nil
	case ValueTypeDecimal:
		if !rv.HasDecimal || lit.Decimal == nil {
			return false, validationErr("INCOMPATIBLE_TYPES", "decimal compare requires decimals")
		}
		return rv.Decimal.Equal(lit.Decimal.Decimal), nil
	default:
		return false, validationErr("INCOMPATIBLE_TYPES", string(lit.Type))
	}
}

func compareOrdered(rv RuntimeValue, lit *Value) (int, error) {
	if lit == nil {
		return 0, validationErr("MISSING_VALUE", "ordered compare requires value")
	}
	switch lit.Type {
	case ValueTypeDecimal:
		if !rv.HasDecimal || lit.Decimal == nil {
			return 0, validationErr("INCOMPATIBLE_TYPES", "ordered decimal compare mismatch")
		}
		return rv.Decimal.Cmp(lit.Decimal.Decimal), nil
	case ValueTypeTimestamp:
		a, b := rv.Timestamp, derefStr(lit.Timestamp)
		return strings.Compare(a, b), nil
	default:
		return 0, validationErr("INCOMPATIBLE_TYPES", "ordered compare requires DECIMAL or TIMESTAMP")
	}
}

func inSet(rv RuntimeValue, values []Value) (bool, error) {
	for i := range values {
		ok, err := compareEqual(rv, &values[i])
		if err != nil {
			return false, err
		}
		if ok {
			return true, nil
		}
	}
	return false, nil
}
