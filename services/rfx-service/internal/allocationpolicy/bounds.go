package allocationpolicy

// Resource bounds for allocation_policy_ast.v1 (W1.2 / J-W1.1-M002).
// Eval step budget is intentionally NOT enforced here — no evaluator in W1.2.
const (
	MaxASTDepth     = 16
	MaxASTNodes     = 256
	MaxINSetSize    = 64
	MaxStringLength = 256

	// MaxDecimalExponentAbs is a safety bound on shopspring/decimal exponent
	// magnitude for weights and decimal literals. Not a commercial money scale;
	// reported to Agent J as a security constraint pending contract precision rules.
	MaxDecimalExponentAbs int32 = 18

	// MaxDecimalCoefficientDigits caps coefficient digit count for DoS safety.
	MaxDecimalCoefficientDigits = 30

	// MaxEvaluationSteps is the hard evaluator step budget (W1.3).
	MaxEvaluationSteps = 10000
)

// Counting rules (normative for W1.2 boundary tests):
//
// Depth:
//   - The root Node has depth 1.
//   - Each child in Args has depth = parentDepth + 1.
//   - MaxASTDepth=16 means the deepest node depth must be ≤ 16.
//   - Depth 17 is reject-closed (AST_DEPTH_EXCEEDED).
//
// Nodes:
//   - Every Node instance counts as exactly 1 (including root and combinators).
//   - Values / literal payloads do not count as nodes.
//   - MaxASTNodes=256 inclusive; 257th node ⇒ AST_NODE_LIMIT_EXCEEDED.
//
// IN-set:
//   - For Op IN / NOT_IN, len(Values) must be 1..MaxINSetSize inclusive.
//
// Strings:
//   - Attr, NodeID, and STRING/ENUM/TIMESTAMP payloads must be ≤ MaxStringLength runes...
//     W1.2 enforces ≤ MaxStringLength bytes (UTF-8 length) for deterministic bounds.
