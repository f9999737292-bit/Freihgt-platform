package allocationpolicy

// Schema and vocabulary constants for allocation_policy_ast.v1.
// Semantic evaluation belongs to later W1 waves; this package only
// defines the typed surface and decode-time safety gates.

const (
	// SchemaVersionV1 is the only schema version accepted by decode.
	SchemaVersionV1 = "allocation_policy_ast.v1"

	// MaxJSONPayloadBytes is the hard pre-parse limit for raw policy JSON.
	MaxJSONPayloadBytes = 65536

	// ScoringModelAdditiveWeights is the only scoring_model allowed in W1
	// (J-W1.1-M001). Legacy 70/30 remains outside this package.
	ScoringModelAdditiveWeights = "ADDITIVE_WEIGHTS"
)

// Allowed ACTION attr flags per W1_AST_SCHEMA §2 (structural only; no executor).
var allowedActionAttrs = map[string]struct{}{
	"shortlist": {},
	"reserve":   {},
	"exception": {},
}

// SharePolicyMode controls how preliminary share targets interact with caps.
// Preliminary shares are not legal obligations.
type SharePolicyMode string

const (
	SharePolicyModeCapOnly        SharePolicyMode = "CAP_ONLY"
	SharePolicyModeTargetWithCap  SharePolicyMode = "TARGET_WITH_CAP"
	SharePolicyModeDefault        SharePolicyMode = SharePolicyModeCapOnly
)

// TieBreakStrategy is the only approved deterministic tie-break for v1.
type TieBreakStrategy string

const (
	TieBreakCarrierIDAsc TieBreakStrategy = "CARRIER_ID_ASC"
)

// NodeKind enumerates typed AST block kinds.
type NodeKind string

const (
	NodeKindFilter     NodeKind = "FILTER"
	NodeKindScore      NodeKind = "SCORE"
	NodeKindPreference NodeKind = "PREFERENCE"
	NodeKindCap        NodeKind = "CAP"
	NodeKindFloor      NodeKind = "FLOOR"
	NodeKindFallback   NodeKind = "FALLBACK"
	NodeKindApproval   NodeKind = "APPROVAL"
	NodeKindAction     NodeKind = "ACTION"
	NodeKindAnd        NodeKind = "AND"
	NodeKindOr         NodeKind = "OR"
	NodeKindNot        NodeKind = "NOT"
)

// Operator enumerates comparison and logical operators.
type Operator string

const (
	OpEQ        Operator = "EQ"
	OpNE        Operator = "NE"
	OpGT        Operator = "GT"
	OpGTE       Operator = "GTE"
	OpLT        Operator = "LT"
	OpLTE       Operator = "LTE"
	OpIN        Operator = "IN"
	OpNOTIN     Operator = "NOT_IN"
	OpBETWEEN   Operator = "BETWEEN"
	OpEXISTS    Operator = "EXISTS"
	OpISMissing Operator = "IS_MISSING"
	OpAND       Operator = "AND"
	OpOR        Operator = "OR"
	OpNOT       Operator = "NOT"
)

// ValueType enumerates typed AST value kinds.
// DECIMAL must never be represented as float64.
type ValueType string

const (
	ValueTypeDecimal      ValueType = "DECIMAL"
	ValueTypeString       ValueType = "STRING"
	ValueTypeBoolean      ValueType = "BOOLEAN"
	ValueTypeEnum         ValueType = "ENUM"
	ValueTypeTimestamp    ValueType = "TIMESTAMP"
	ValueTypeNull         ValueType = "NULL"
	ValueTypeMissing      ValueType = "MISSING"
	ValueTypeNotAvailable ValueType = "NOT_AVAILABLE"
)

var allowedSharePolicyModes = map[SharePolicyMode]struct{}{
	SharePolicyModeCapOnly:       {},
	SharePolicyModeTargetWithCap: {},
}

var allowedTieBreakStrategies = map[TieBreakStrategy]struct{}{
	TieBreakCarrierIDAsc: {},
}

var allowedNodeKinds = map[NodeKind]struct{}{
	NodeKindFilter:     {},
	NodeKindScore:      {},
	NodeKindPreference: {},
	NodeKindCap:        {},
	NodeKindFloor:      {},
	NodeKindFallback:   {},
	NodeKindApproval:   {},
	NodeKindAction:     {},
	NodeKindAnd:        {},
	NodeKindOr:         {},
	NodeKindNot:        {},
}

var allowedOperators = map[Operator]struct{}{
	OpEQ:        {},
	OpNE:        {},
	OpGT:        {},
	OpGTE:       {},
	OpLT:        {},
	OpLTE:       {},
	OpIN:        {},
	OpNOTIN:     {},
	OpBETWEEN:   {},
	OpEXISTS:    {},
	OpISMissing: {},
	OpAND:       {},
	OpOR:        {},
	OpNOT:       {},
}

var allowedValueTypes = map[ValueType]struct{}{
	ValueTypeDecimal:      {},
	ValueTypeString:       {},
	ValueTypeBoolean:      {},
	ValueTypeEnum:         {},
	ValueTypeTimestamp:    {},
	ValueTypeNull:         {},
	ValueTypeMissing:      {},
	ValueTypeNotAvailable: {},
}

// Forbidden AST surface markers. Presence in raw payload is reject-closed.
// These are not executable hooks — decode refuses them outright.
var forbiddenPayloadMarkers = []string{
	"sql",
	"raw_sql",
	"query_sql",
	"script",
	"javascript",
	"js_eval",
	"http",
	"http_call",
	"webhook",
	"shell",
	"exec",
	"os_exec",
	"command",
}
