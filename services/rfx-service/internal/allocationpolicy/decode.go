package allocationpolicy

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"strings"
)

// DecodeError is a fail-closed decode / schema-safety error.
type DecodeError struct {
	Code    string
	Message string
}

func (e *DecodeError) Error() string {
	return fmt.Sprintf("%s: %s", e.Code, e.Message)
}

func decodeErr(code, msg string) error {
	return &DecodeError{Code: code, Message: msg}
}

// DecodePolicy parses raw JSON into a typed Policy AST.
// Safety gates:
//   - raw payload size ≤ MaxJSONPayloadBytes before parse
//   - duplicate JSON keys rejected (J-W1.1-M003)
//   - unknown schema_version rejected
//   - unknown JSON fields rejected (DisallowUnknownFields)
//   - scoring_model allowlist ADDITIVE_WEIGHTS (J-W1.1-M001)
//   - share_policy_mode defaults to CAP_ONLY when omitted
//   - tie_break defaults to CARRIER_ID_ASC when omitted (J-W1.1-M004 / §8)
//   - forbidden SQL / script / HTTP / shell markers rejected
//   - DECIMAL never accepted as JSON number (float64)
// Full structural validation is ValidatePolicy (W1.2).
func DecodePolicy(raw []byte) (*Policy, error) {
	if len(raw) > MaxJSONPayloadBytes {
		return nil, decodeErr("PAYLOAD_TOO_LARGE",
			fmt.Sprintf("JSON payload size %d exceeds limit %d", len(raw), MaxJSONPayloadBytes))
	}
	if len(bytes.TrimSpace(raw)) == 0 {
		return nil, decodeErr("EMPTY_PAYLOAD", "JSON payload is empty")
	}

	if err := rejectForbiddenMarkers(raw); err != nil {
		return nil, err
	}
	if err := rejectDuplicateJSONKeys(raw); err != nil {
		return nil, err
	}

	var wire policyWire
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&wire); err != nil {
		return nil, mapJSONError(err)
	}
	if err := dec.Decode(&struct{}{}); err != io.EOF {
		if err == nil {
			return nil, decodeErr("TRAILING_DATA", "trailing data after top-level JSON value")
		}
		return nil, mapJSONError(err)
	}

	if wire.SchemaVersion != SchemaVersionV1 {
		return nil, decodeErr("UNKNOWN_SCHEMA_VERSION",
			fmt.Sprintf("unsupported schema_version %q; required %q", wire.SchemaVersion, SchemaVersionV1))
	}
	if strings.TrimSpace(wire.PolicyID) == "" {
		return nil, decodeErr("MISSING_POLICY_ID", "policy_id is required")
	}

	scoringModel, err := parseScoringModel(wire.ScoringModel)
	if err != nil {
		return nil, err
	}

	shareMode := SharePolicyModeDefault
	if wire.SharePolicyMode != nil {
		shareMode = *wire.SharePolicyMode
		if _, ok := allowedSharePolicyModes[shareMode]; !ok {
			return nil, decodeErr("UNKNOWN_SHARE_POLICY_MODE",
				fmt.Sprintf("unknown share_policy_mode %q", shareMode))
		}
	}

	tieBreak, err := parseTieBreak(wire.TieBreak)
	if err != nil {
		return nil, err
	}

	if wire.Root == nil {
		return nil, decodeErr("MISSING_ROOT", "root is required")
	}

	root, err := wire.Root.toNode()
	if err != nil {
		return nil, err
	}

	return &Policy{
		SchemaVersion:   wire.SchemaVersion,
		PolicyID:        wire.PolicyID,
		ScoringModel:    scoringModel,
		SharePolicyMode: shareMode,
		TieBreak:        tieBreak,
		Root:            *root,
	}, nil
}

// DecodeAndValidate decodes then runs W1.2 structural validation.
func DecodeAndValidate(raw []byte) (*Policy, error) {
	p, err := DecodePolicy(raw)
	if err != nil {
		return nil, err
	}
	if err := ValidatePolicy(p); err != nil {
		return nil, err
	}
	return p, nil
}

// policyWire is the JSON decode surface. Pointer/RawMessage fields distinguish omit vs set.
type policyWire struct {
	SchemaVersion   string           `json:"schema_version"`
	PolicyID        string           `json:"policy_id"`
	ScoringModel    json.RawMessage  `json:"scoring_model"`
	SharePolicyMode *SharePolicyMode `json:"share_policy_mode"`
	TieBreak        json.RawMessage  `json:"tie_break"`
	Root            *nodeWire        `json:"root"`
}

type tieBreakWire struct {
	Strategy TieBreakStrategy `json:"strategy"`
}

type nodeWire struct {
	Kind   NodeKind    `json:"kind"`
	Op     Operator    `json:"op"`
	Args   []nodeWire  `json:"args"`
	Attr   string      `json:"attr"`
	Value  *valueWire  `json:"value"`
	Values []valueWire `json:"values"`
	Weight *Decimal    `json:"weight"`
	NodeID string      `json:"node_id"`
}

type valueWire struct {
	Type      ValueType `json:"type"`
	String    *string   `json:"string"`
	Boolean   *bool     `json:"boolean"`
	Enum      *string   `json:"enum"`
	Timestamp *string   `json:"timestamp"`
	Decimal   *Decimal  `json:"decimal"`
}

func parseScoringModel(raw json.RawMessage) (string, error) {
	if len(raw) == 0 {
		return "", decodeErr("MISSING_SCORING_MODEL", "scoring_model is required")
	}
	trim := bytes.TrimSpace(raw)
	if bytes.Equal(trim, []byte("null")) {
		return "", decodeErr("NULL_SCORING_MODEL", "scoring_model must not be null")
	}
	var s string
	if err := json.Unmarshal(raw, &s); err != nil {
		return "", decodeErr("INVALID_SCORING_MODEL_TYPE", "scoring_model must be a JSON string")
	}
	if strings.TrimSpace(s) == "" {
		return "", decodeErr("MISSING_SCORING_MODEL", "scoring_model is required")
	}
	if s != ScoringModelAdditiveWeights {
		// Includes legacy 70/30 labels and any undocumented model names.
		return "", decodeErr("UNKNOWN_SCORING_MODEL",
			fmt.Sprintf("unsupported scoring_model %q; required %q", s, ScoringModelAdditiveWeights))
	}
	return s, nil
}

func parseTieBreak(raw json.RawMessage) (TieBreak, error) {
	// Omitted → documented default (W1_AST_SCHEMA §8 / J-W1.1-M004).
	if len(raw) == 0 {
		return TieBreak{Strategy: TieBreakCarrierIDAsc}, nil
	}
	trim := bytes.TrimSpace(raw)
	if bytes.Equal(trim, []byte("null")) {
		return TieBreak{}, decodeErr("NULL_TIE_BREAK", "tie_break must not be null")
	}
	if trim[0] != '{' {
		return TieBreak{}, decodeErr("INVALID_TIE_BREAK_TYPE", "tie_break must be a JSON object")
	}
	var tb tieBreakWire
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&tb); err != nil {
		return TieBreak{}, mapJSONError(err)
	}
	if tb.Strategy == "" {
		return TieBreak{}, decodeErr("MISSING_TIE_BREAK_STRATEGY", "tie_break.strategy is required when tie_break is present")
	}
	if _, ok := allowedTieBreakStrategies[tb.Strategy]; !ok {
		return TieBreak{}, decodeErr("UNKNOWN_TIE_BREAK",
			fmt.Sprintf("unknown tie_break.strategy %q", tb.Strategy))
	}
	return TieBreak{Strategy: tb.Strategy}, nil
}

func (n *nodeWire) toNode() (*Node, error) {
	if n == nil {
		return nil, decodeErr("NIL_NODE", "node is nil")
	}
	if _, ok := allowedNodeKinds[n.Kind]; !ok {
		return nil, decodeErr("UNKNOWN_NODE_KIND",
			fmt.Sprintf("unknown node kind %q", n.Kind))
	}
	if n.Op != "" {
		if _, ok := allowedOperators[n.Op]; !ok {
			return nil, decodeErr("UNKNOWN_OPERATOR",
				fmt.Sprintf("unknown operator %q", n.Op))
		}
	}

	out := &Node{
		Kind:   n.Kind,
		Op:     n.Op,
		Attr:   n.Attr,
		Weight: n.Weight,
		NodeID: n.NodeID,
	}

	if n.Value != nil {
		v, err := n.Value.toValue()
		if err != nil {
			return nil, err
		}
		out.Value = v
	}
	if len(n.Values) > 0 {
		out.Values = make([]Value, 0, len(n.Values))
		for i := range n.Values {
			v, err := n.Values[i].toValue()
			if err != nil {
				return nil, err
			}
			out.Values = append(out.Values, *v)
		}
	}
	if len(n.Args) > 0 {
		out.Args = make([]Node, 0, len(n.Args))
		for i := range n.Args {
			child, err := n.Args[i].toNode()
			if err != nil {
				return nil, err
			}
			out.Args = append(out.Args, *child)
		}
	}
	return out, nil
}

func (v *valueWire) toValue() (*Value, error) {
	if v == nil {
		return nil, decodeErr("NIL_VALUE", "value is nil")
	}
	if _, ok := allowedValueTypes[v.Type]; !ok {
		return nil, decodeErr("UNKNOWN_VALUE_TYPE",
			fmt.Sprintf("unknown value type %q", v.Type))
	}

	out := &Value{Type: v.Type}

	switch v.Type {
	case ValueTypeNull, ValueTypeMissing, ValueTypeNotAvailable:
		if v.String != nil || v.Boolean != nil || v.Enum != nil || v.Timestamp != nil || v.Decimal != nil {
			return nil, decodeErr("INVALID_VALUE_SHAPE",
				fmt.Sprintf("%s value must not carry a payload field", v.Type))
		}
	case ValueTypeString:
		if v.String == nil {
			return nil, decodeErr("INVALID_VALUE_SHAPE", "STRING value requires string field")
		}
		out.String = v.String
	case ValueTypeBoolean:
		if v.Boolean == nil {
			return nil, decodeErr("INVALID_VALUE_SHAPE", "BOOLEAN value requires boolean field")
		}
		out.Boolean = v.Boolean
	case ValueTypeEnum:
		if v.Enum == nil {
			return nil, decodeErr("INVALID_VALUE_SHAPE", "ENUM value requires enum field")
		}
		out.Enum = v.Enum
	case ValueTypeTimestamp:
		if v.Timestamp == nil {
			return nil, decodeErr("INVALID_VALUE_SHAPE", "TIMESTAMP value requires timestamp field")
		}
		out.Timestamp = v.Timestamp
	case ValueTypeDecimal:
		if v.Decimal == nil {
			return nil, decodeErr("INVALID_VALUE_SHAPE", "DECIMAL value requires decimal string field")
		}
		out.Decimal = v.Decimal
	default:
		return nil, decodeErr("UNKNOWN_VALUE_TYPE", fmt.Sprintf("unhandled value type %q", v.Type))
	}
	return out, nil
}

func rejectForbiddenMarkers(raw []byte) error {
	lower := strings.ToLower(string(raw))
	for _, marker := range forbiddenPayloadMarkers {
		needle := `"` + marker + `":`
		if strings.Contains(lower, needle) {
			return decodeErr("FORBIDDEN_SURFACE",
				fmt.Sprintf("forbidden AST surface marker %q is not allowed", marker))
		}
	}
	return nil
}

func mapJSONError(err error) error {
	if err == nil {
		return nil
	}
	msg := err.Error()
	switch {
	case strings.Contains(msg, "unknown field"):
		return decodeErr("UNKNOWN_FIELD", msg)
	case strings.Contains(msg, "decimal must be a JSON string"):
		return decodeErr("DECIMAL_FLOAT64_FORBIDDEN", msg)
	case strings.Contains(msg, "invalid decimal"):
		return decodeErr("INVALID_DECIMAL", msg)
	case strings.Contains(msg, "cannot unmarshal"):
		return decodeErr("INVALID_VALUE_TYPE", msg)
	default:
		return decodeErr("JSON_DECODE", msg)
	}
}
