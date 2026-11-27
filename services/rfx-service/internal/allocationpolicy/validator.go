package allocationpolicy

import (
	"fmt"
	"math"
	"strings"

	"github.com/shopspring/decimal"
)

// ValidationError is a fail-closed structural / semantic validation error.
// Alias of DecodeError so callers can type-assert a single error shape.
type ValidationError = DecodeError

func validationErr(code, msg string) error {
	return decodeErr(code, msg)
}

// ValidatePolicy performs W1.2 structural validation on an already-decoded Policy.
// It does not evaluate filters/scores, does not fingerprint, and does not execute ACTION.
//
// EXISTS(NULL) evaluation is intentionally NOT implemented
// (EXISTS_NULL_DECISION=AWAITING_J_CONFIRMATION).
func ValidatePolicy(p *Policy) error {
	if p == nil {
		return validationErr("NIL_POLICY", "policy is nil")
	}
	if p.SchemaVersion != SchemaVersionV1 {
		return validationErr("UNKNOWN_SCHEMA_VERSION",
			fmt.Sprintf("unsupported schema_version %q", p.SchemaVersion))
	}
	if p.ScoringModel != ScoringModelAdditiveWeights {
		return validationErr("UNKNOWN_SCORING_MODEL",
			fmt.Sprintf("unsupported scoring_model %q", p.ScoringModel))
	}
	if _, ok := allowedSharePolicyModes[p.SharePolicyMode]; !ok {
		return validationErr("UNKNOWN_SHARE_POLICY_MODE",
			fmt.Sprintf("unknown share_policy_mode %q", p.SharePolicyMode))
	}
	if _, ok := allowedTieBreakStrategies[p.TieBreak.Strategy]; !ok {
		return validationErr("UNKNOWN_TIE_BREAK",
			fmt.Sprintf("unknown tie_break.strategy %q", p.TieBreak.Strategy))
	}
	if err := checkStringBound("policy_id", p.PolicyID); err != nil {
		return err
	}

	state := &validateState{}
	if err := validateNode(&p.Root, 1, state); err != nil {
		return err
	}
	if state.nodes > MaxASTNodes {
		return validationErr("AST_NODE_LIMIT_EXCEEDED",
			fmt.Sprintf("AST has %d nodes; max %d", state.nodes, MaxASTNodes))
	}

	// ADDITIVE_WEIGHTS: SCORE weights must sum to exactly 1.00 (tolerance 0).
	one := decimal.RequireFromString("1.00")
	if !state.weightSum.Equal(one) {
		return validationErr("INVALID_WEIGHT_SUM",
			fmt.Sprintf("ADDITIVE_WEIGHTS score weights sum to %s; required exactly 1.00", state.weightSum.String()))
	}

	// J-W1.3-H003: OR/NOT subtrees must be pure HARD (no SCORE/PREFERENCE/…).
	if err := validateBooleanHardTree(&p.Root, false); err != nil {
		return err
	}
	return nil
}

// validateBooleanHardTree enforces W1.3 R2 fail-closed boolean structure (J-W1.3-C001/H003).
// underPureHard=true means we are inside OR or NOT: only FILTER/FLOOR/CAP and pure boolean nodes allowed.
func validateBooleanHardTree(n *Node, underPureHard bool) error {
	if n == nil {
		return validationErr("NIL_NODE", "node is nil")
	}
	switch n.Kind {
	case NodeKindAnd:
		// AND may mix HARD and non-HARD at top/structural level, but when nested under OR/NOT
		// the entire AND subtree must itself be pure HARD.
		childPure := underPureHard
		for i := range n.Args {
			if err := validateBooleanHardTree(&n.Args[i], childPure); err != nil {
				return err
			}
		}
		return nil
	case NodeKindOr, NodeKindNot:
		for i := range n.Args {
			if err := validateBooleanHardTree(&n.Args[i], true); err != nil {
				return err
			}
		}
		return nil
	case NodeKindFilter, NodeKindFloor, NodeKindCap:
		return nil
	case NodeKindScore, NodeKindPreference, NodeKindFallback, NodeKindApproval, NodeKindAction:
		if underPureHard {
			return validationErr("MIXED_HARD_NONHARD_BOOLEAN_TREE",
				fmt.Sprintf("%s is not a HARD predicate and cannot appear under OR/NOT", n.Kind))
		}
		return nil
	default:
		return validationErr("UNKNOWN_NODE_KIND", string(n.Kind))
	}
}

type validateState struct {
	nodes     int
	weightSum decimal.Decimal
}

func validateNode(n *Node, depth int, st *validateState) error {
	if n == nil {
		return validationErr("NIL_NODE", "node is nil")
	}
	if depth > MaxASTDepth {
		return validationErr("AST_DEPTH_EXCEEDED",
			fmt.Sprintf("AST depth %d exceeds max %d", depth, MaxASTDepth))
	}
	st.nodes++
	if st.nodes > MaxASTNodes {
		return validationErr("AST_NODE_LIMIT_EXCEEDED",
			fmt.Sprintf("AST exceeds max nodes %d", MaxASTNodes))
	}

	if _, ok := allowedNodeKinds[n.Kind]; !ok {
		return validationErr("UNKNOWN_NODE_KIND", fmt.Sprintf("unknown node kind %q", n.Kind))
	}
	if err := checkStringBound("attr", n.Attr); err != nil {
		return err
	}
	if err := checkStringBound("node_id", n.NodeID); err != nil {
		return err
	}
	// ACTION attrs use the structural shortlist/reserve/exception allowlist, not the KPI catalog.
	if strings.TrimSpace(n.Attr) != "" && n.Kind != NodeKindAction {
		if err := validateAttributeUsage(n); err != nil {
			return err
		}
	}

	switch n.Kind {
	case NodeKindAnd, NodeKindOr:
		if n.Op != "" && n.Op != Operator(n.Kind) {
			return validationErr("INVALID_NODE_SHAPE",
				fmt.Sprintf("%s node op must be empty or %s", n.Kind, n.Kind))
		}
		if n.Weight != nil || n.Value != nil || len(n.Values) > 0 {
			return validationErr("INVALID_NODE_SHAPE",
				fmt.Sprintf("%s node must not carry value/weight payloads", n.Kind))
		}
		for i := range n.Args {
			if err := validateNode(&n.Args[i], depth+1, st); err != nil {
				return err
			}
		}
	case NodeKindNot:
		if len(n.Args) != 1 {
			return validationErr("INVALID_NODE_ARITY",
				fmt.Sprintf("NOT requires exactly 1 arg; got %d", len(n.Args)))
		}
		if n.Weight != nil || n.Value != nil || len(n.Values) > 0 {
			return validationErr("INVALID_NODE_SHAPE", "NOT node must not carry value/weight payloads")
		}
		return validateNode(&n.Args[0], depth+1, st)

	case NodeKindFilter, NodeKindPreference, NodeKindCap, NodeKindFloor, NodeKindFallback:
		if len(n.Args) != 0 {
			return validationErr("INVALID_NODE_SHAPE",
				fmt.Sprintf("%s must not have child args", n.Kind))
		}
		if strings.TrimSpace(n.Attr) == "" {
			return validationErr("MISSING_ATTR", fmt.Sprintf("%s requires attr", n.Kind))
		}
		if n.Weight != nil {
			return validationErr("INVALID_NODE_SHAPE",
				fmt.Sprintf("%s must not carry weight", n.Kind))
		}
		if err := validateComparisonNode(n); err != nil {
			return err
		}

	case NodeKindScore:
		if len(n.Args) != 0 {
			return validationErr("INVALID_NODE_SHAPE", "SCORE must not have child args")
		}
		if strings.TrimSpace(n.Attr) == "" {
			return validationErr("MISSING_ATTR", "SCORE requires attr")
		}
		if n.Weight == nil {
			return validationErr("MISSING_WEIGHT", "SCORE requires weight")
		}
		if err := validateWeight(n.Weight); err != nil {
			return err
		}
		st.weightSum = st.weightSum.Add(n.Weight.Decimal)
		// Optional comparison payload on SCORE (component definition).
		if n.Op != "" || n.Value != nil || len(n.Values) > 0 {
			if err := validateComparisonNode(n); err != nil {
				return err
			}
		}

	case NodeKindApproval:
		// Structural only — no commercial approval workflow invented here.
		if len(n.Args) != 0 {
			return validationErr("INVALID_NODE_SHAPE", "APPROVAL must not have child args")
		}
		if n.Weight != nil {
			return validationErr("INVALID_NODE_SHAPE", "APPROVAL must not carry weight")
		}
		if n.Op != "" {
			if err := validateComparisonNode(n); err != nil {
				return err
			}
		}

	case NodeKindAction:
		// Structural flags only — never Award/TO/EDO/HTTP/SQL/script.
		if len(n.Args) != 0 {
			return validationErr("INVALID_NODE_SHAPE", "ACTION must not have child args")
		}
		if n.Weight != nil {
			return validationErr("INVALID_NODE_SHAPE", "ACTION must not carry weight")
		}
		if n.Op != "" {
			return validationErr("INVALID_NODE_SHAPE", "ACTION must not carry comparison op")
		}
		if _, ok := allowedActionAttrs[n.Attr]; !ok {
			return validationErr("INVALID_ACTION_ATTR",
				fmt.Sprintf("ACTION attr %q not in {shortlist,reserve,exception}", n.Attr))
		}
		if n.Value != nil {
			if err := validateValueBounds(n.Value); err != nil {
				return err
			}
		}
		if len(n.Values) != 0 {
			return validationErr("INVALID_NODE_SHAPE", "ACTION must not carry values set")
		}

	default:
		return validationErr("UNKNOWN_NODE_KIND", fmt.Sprintf("unhandled node kind %q", n.Kind))
	}
	return nil
}

func validateComparisonNode(n *Node) error {
	if n.Op == "" {
		return validationErr("MISSING_OPERATOR", fmt.Sprintf("%s requires op", n.Kind))
	}
	if _, ok := allowedOperators[n.Op]; !ok {
		return validationErr("UNKNOWN_OPERATOR", fmt.Sprintf("unknown operator %q", n.Op))
	}
	// Combinator ops are not valid on leaf comparison nodes.
	switch n.Op {
	case OpAND, OpOR, OpNOT:
		return validationErr("INVALID_OPERATOR",
			fmt.Sprintf("operator %q is not valid on %s", n.Op, n.Kind))
	}

	switch n.Op {
	case OpEXISTS, OpISMissing:
		if n.Value != nil || len(n.Values) > 0 {
			// Representation of sentinel literals is allowed only as typed Value forms
			// elsewhere; EXISTS/IS_MISSING bind to attr presence, not literal operands.
			return validationErr("INVALID_NODE_SHAPE",
				fmt.Sprintf("%s must not carry value/values operands", n.Op))
		}
		return nil

	case OpIN, OpNOTIN:
		if n.Value != nil {
			return validationErr("INVALID_NODE_SHAPE",
				fmt.Sprintf("%s uses values[]; singular value is not allowed", n.Op))
		}
		if len(n.Values) == 0 {
			return validationErr("EMPTY_IN_SET", fmt.Sprintf("%s requires a non-empty values set", n.Op))
		}
		if len(n.Values) > MaxINSetSize {
			return validationErr("IN_SET_SIZE_EXCEEDED",
				fmt.Sprintf("%s values size %d exceeds max %d", n.Op, len(n.Values), MaxINSetSize))
		}
		base := n.Values[0].Type
		if !isComparableSetType(base) {
			return validationErr("INCOMPATIBLE_TYPES",
				fmt.Sprintf("%s element type %s is not allowed", n.Op, base))
		}
		for i := range n.Values {
			if err := validateValueBounds(&n.Values[i]); err != nil {
				return err
			}
			if n.Values[i].Type != base {
				return validationErr("INCOMPATIBLE_TYPES",
					fmt.Sprintf("%s set elements must share one type; got %s and %s", n.Op, base, n.Values[i].Type))
			}
			if isSentinelType(n.Values[i].Type) {
				return validationErr("INCOMPATIBLE_TYPES",
					fmt.Sprintf("%s set must not contain sentinel type %s", n.Op, n.Values[i].Type))
			}
		}
		return nil

	case OpBETWEEN:
		if n.Value != nil {
			return validationErr("INVALID_NODE_SHAPE", "BETWEEN uses values[2]; singular value is not allowed")
		}
		if len(n.Values) != 2 {
			return validationErr("INVALID_NODE_ARITY",
				fmt.Sprintf("BETWEEN requires exactly 2 values; got %d", len(n.Values)))
		}
		for i := range n.Values {
			if err := validateValueBounds(&n.Values[i]); err != nil {
				return err
			}
		}
		if n.Values[0].Type != n.Values[1].Type {
			return validationErr("INCOMPATIBLE_TYPES", "BETWEEN bounds must share one type")
		}
		if !isOrderedType(n.Values[0].Type) {
			return validationErr("INCOMPATIBLE_TYPES",
				fmt.Sprintf("BETWEEN does not allow type %s", n.Values[0].Type))
		}
		return nil

	case OpEQ, OpNE, OpGT, OpGTE, OpLT, OpLTE:
		if len(n.Values) != 0 {
			return validationErr("INVALID_NODE_SHAPE",
				fmt.Sprintf("%s uses singular value; values[] is not allowed", n.Op))
		}
		if n.Value == nil {
			return validationErr("MISSING_VALUE", fmt.Sprintf("%s requires value", n.Op))
		}
		if err := validateValueBounds(n.Value); err != nil {
			return err
		}
		if isSentinelType(n.Value.Type) {
			// Literals NULL/MISSING/NOT_AVAILABLE as comparison RHS are representable
			// but type-incompatible for ordered/equality commercial compares at validate.
			// Three-valued eval of attr-state is W1.3+; reject literal sentinel RHS here.
			return validationErr("INCOMPATIBLE_TYPES",
				fmt.Sprintf("%s literal operand type %s is not allowed", n.Op, n.Value.Type))
		}
		switch n.Op {
		case OpGT, OpGTE, OpLT, OpLTE:
			if !isOrderedType(n.Value.Type) {
				return validationErr("INCOMPATIBLE_TYPES",
					fmt.Sprintf("%s requires DECIMAL or TIMESTAMP; got %s", n.Op, n.Value.Type))
			}
		case OpEQ, OpNE:
			if !isEqualityType(n.Value.Type) {
				return validationErr("INCOMPATIBLE_TYPES",
					fmt.Sprintf("%s does not allow type %s", n.Op, n.Value.Type))
			}
		}
		return nil
	default:
		return validationErr("UNKNOWN_OPERATOR", fmt.Sprintf("unhandled operator %q", n.Op))
	}
}

func validateValueBounds(v *Value) error {
	if v == nil {
		return validationErr("NIL_VALUE", "value is nil")
	}
	if _, ok := allowedValueTypes[v.Type]; !ok {
		return validationErr("UNKNOWN_VALUE_TYPE", fmt.Sprintf("unknown value type %q", v.Type))
	}
	switch v.Type {
	case ValueTypeNull, ValueTypeMissing, ValueTypeNotAvailable:
		if v.String != nil || v.Boolean != nil || v.Enum != nil || v.Timestamp != nil || v.Decimal != nil {
			return validationErr("INVALID_VALUE_SHAPE",
				fmt.Sprintf("%s must not carry a payload field", v.Type))
		}
	case ValueTypeString:
		if v.String == nil {
			return validationErr("INVALID_VALUE_SHAPE", "STRING requires string")
		}
		return checkStringBound("string", *v.String)
	case ValueTypeEnum:
		if v.Enum == nil {
			return validationErr("INVALID_VALUE_SHAPE", "ENUM requires enum")
		}
		return checkStringBound("enum", *v.Enum)
	case ValueTypeTimestamp:
		if v.Timestamp == nil {
			return validationErr("INVALID_VALUE_SHAPE", "TIMESTAMP requires timestamp")
		}
		return checkStringBound("timestamp", *v.Timestamp)
	case ValueTypeBoolean:
		if v.Boolean == nil {
			return validationErr("INVALID_VALUE_SHAPE", "BOOLEAN requires boolean")
		}
	case ValueTypeDecimal:
		if v.Decimal == nil {
			return validationErr("INVALID_VALUE_SHAPE", "DECIMAL requires decimal")
		}
		return validateDecimalSafety(v.Decimal)
	}
	return nil
}

func validateWeight(w *Decimal) error {
	if w == nil {
		return validationErr("MISSING_WEIGHT", "weight is required")
	}
	if err := validateDecimalSafety(w); err != nil {
		return err
	}
	if w.IsNegative() {
		return validationErr("INVALID_WEIGHT", "SCORE weight must not be negative")
	}
	return nil
}

func validateDecimalSafety(d *Decimal) error {
	if d == nil {
		return validationErr("INVALID_DECIMAL", "decimal is nil")
	}
	// J-W1.2-G001: bound exponent / coefficient BEFORE any Decimal.String() formatting.
	exp := d.Exponent()
	if exp < 0 {
		exp = -exp
	}
	if exp > MaxDecimalExponentAbs {
		return validationErr("DECIMAL_EXPONENT_EXCEEDED",
			fmt.Sprintf("decimal exponent magnitude %d exceeds max %d", exp, MaxDecimalExponentAbs))
	}
	coeff := d.Coefficient()
	if coeff != nil {
		// Digit capacity via BitLen (no full decimal string materialization).
		// 10^n requires ~ n*log2(10) bits; refuse oversized coefficients first.
		maxBits := int(MaxDecimalCoefficientDigits)*3322/1000 + 2
		if coeff.BitLen() > maxBits {
			return validationErr("DECIMAL_PRECISION_EXCEEDED",
				fmt.Sprintf("decimal coefficient bit length exceeds safe bound for %d digits", MaxDecimalCoefficientDigits))
		}
	}
	// Reject non-finite float64 views if ever constructed via unsafe paths.
	if f, exact := d.Float64(); exact && (math.IsNaN(f) || math.IsInf(f, 0)) {
		return validationErr("INVALID_DECIMAL", "NaN/Infinity decimals are forbidden")
	}
	return nil
}

func checkStringBound(field, s string) error {
	if len(s) > MaxStringLength {
		return validationErr("STRING_LENGTH_EXCEEDED",
			fmt.Sprintf("%s length %d exceeds max %d", field, len(s), MaxStringLength))
	}
	return nil
}

func isSentinelType(t ValueType) bool {
	return t == ValueTypeNull || t == ValueTypeMissing || t == ValueTypeNotAvailable
}

func isOrderedType(t ValueType) bool {
	return t == ValueTypeDecimal || t == ValueTypeTimestamp
}

func isEqualityType(t ValueType) bool {
	switch t {
	case ValueTypeDecimal, ValueTypeString, ValueTypeBoolean, ValueTypeEnum, ValueTypeTimestamp:
		return true
	default:
		return false
	}
}

func isComparableSetType(t ValueType) bool {
	return isEqualityType(t)
}
