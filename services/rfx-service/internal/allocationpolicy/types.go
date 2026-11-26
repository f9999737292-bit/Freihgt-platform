package allocationpolicy

import (
	"encoding/json"
	"fmt"

	"github.com/shopspring/decimal"
)

// Policy is the top-level typed allocation policy AST document.
type Policy struct {
	SchemaVersion   string         `json:"schema_version"`
	PolicyID        string         `json:"policy_id"`
	ScoringModel    string         `json:"scoring_model"`
	SharePolicyMode SharePolicyMode `json:"share_policy_mode"`
	TieBreak        TieBreak       `json:"tie_break"`
	Root            Node           `json:"root"`
}

// TieBreak holds the deterministic post-PREFERENCE ordering strategy.
type TieBreak struct {
	Strategy TieBreakStrategy `json:"strategy"`
}

// Node is a typed AST node. Combinators use Kind AND|OR|NOT with Args.
// Block kinds (FILTER, SCORE, …) carry Attr / Op / Value / Weight as needed.
// ACTION is structural only — there is no universal ACTION executor in W1.1.
type Node struct {
	Kind   NodeKind  `json:"kind"`
	Op     Operator  `json:"op,omitempty"`
	Args   []Node    `json:"args,omitempty"`
	Attr   string    `json:"attr,omitempty"`
	Value  *Value    `json:"value,omitempty"`
	Values []Value   `json:"values,omitempty"` // IN / NOT_IN / BETWEEN operands
	Weight *Decimal  `json:"weight,omitempty"` // SCORE weights — decimal, never float64
	NodeID string    `json:"node_id,omitempty"`
}

// Value is a strictly typed AST literal. Sentinel states NULL / MISSING /
// NOT_AVAILABLE are representable without evaluating EXISTS(NULL) semantics
// (evaluation deferred pending Agent J confirmation).
type Value struct {
	Type      ValueType `json:"type"`
	String    *string   `json:"string,omitempty"`
	Boolean   *bool     `json:"boolean,omitempty"`
	Enum      *string   `json:"enum,omitempty"`
	Timestamp *string   `json:"timestamp,omitempty"` // RFC3339 textual; parse deferred to W1.2+
	Decimal   *Decimal  `json:"decimal,omitempty"`
}

// Decimal wraps shopspring/decimal so JSON money/weights/scores never use float64.
type Decimal struct {
	decimal.Decimal
}

// NewDecimalFromString parses a decimal string. Rejects float-origin paths.
func NewDecimalFromString(s string) (Decimal, error) {
	d, err := decimal.NewFromString(s)
	if err != nil {
		return Decimal{}, fmt.Errorf("invalid decimal %q: %w", s, err)
	}
	return Decimal{Decimal: d}, nil
}

// MarshalJSON encodes Decimal as a JSON string (never a JSON number).
func (d Decimal) MarshalJSON() ([]byte, error) {
	return []byte(`"` + d.String() + `"`), nil
}

// UnmarshalJSON accepts only a JSON string. JSON numbers (float64 path) are rejected.
func (d *Decimal) UnmarshalJSON(b []byte) error {
	if len(b) == 0 {
		return fmt.Errorf("empty decimal")
	}
	if b[0] != '"' {
		return fmt.Errorf("decimal must be a JSON string, not a JSON number (float64 forbidden)")
	}
	var s string
	if err := json.Unmarshal(b, &s); err != nil {
		return fmt.Errorf("invalid decimal JSON string: %w", err)
	}
	parsed, err := NewDecimalFromString(s)
	if err != nil {
		return err
	}
	*d = parsed
	return nil
}

// IsZero reports whether the decimal is zero.
func (d Decimal) IsZero() bool {
	return d.Decimal.IsZero()
}
