package allocationpolicy

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
)

// rejectDuplicateJSONKeys walks the JSON token stream and fails closed when any
// object repeats a key (J-W1.1-M003). encoding/json last-key-wins is not trusted.
// Keys are compared as decoded Unicode strings (Token unescapes), so equivalent
// escaped spellings of the same name also collide.
func rejectDuplicateJSONKeys(raw []byte) error {
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.UseNumber()
	if err := walkJSONValue(dec); err != nil {
		return err
	}
	// Ensure single top-level value (trailing handled later by DecodePolicy).
	return nil
}

func walkJSONValue(dec *json.Decoder) error {
	tok, err := dec.Token()
	if err != nil {
		if err == io.EOF {
			return decodeErr("JSON_DECODE", "unexpected EOF while scanning JSON")
		}
		return decodeErr("JSON_DECODE", err.Error())
	}
	switch t := tok.(type) {
	case json.Delim:
		switch t {
		case '{':
			seen := make(map[string]struct{})
			for dec.More() {
				keyTok, err := dec.Token()
				if err != nil {
					return decodeErr("JSON_DECODE", err.Error())
				}
				key, ok := keyTok.(string)
				if !ok {
					return decodeErr("JSON_DECODE", "object key must be a string")
				}
				if _, dup := seen[key]; dup {
					return decodeErr("DUPLICATE_JSON_KEY",
						fmt.Sprintf("duplicate JSON object key %q", key))
				}
				seen[key] = struct{}{}
				if err := walkJSONValue(dec); err != nil {
					return err
				}
			}
			end, err := dec.Token()
			if err != nil {
				return decodeErr("JSON_DECODE", err.Error())
			}
			if end != json.Delim('}') {
				return decodeErr("JSON_DECODE", "expected closing '}'")
			}
		case '[':
			for dec.More() {
				if err := walkJSONValue(dec); err != nil {
					return err
				}
			}
			end, err := dec.Token()
			if err != nil {
				return decodeErr("JSON_DECODE", err.Error())
			}
			if end != json.Delim(']') {
				return decodeErr("JSON_DECODE", "expected closing ']'")
			}
		default:
			return decodeErr("JSON_DECODE", fmt.Sprintf("unexpected delimiter %q", t))
		}
	case bool, float64, json.Number, string, nil:
		// Scalar leaf — OK.
		return nil
	default:
		return decodeErr("JSON_DECODE", fmt.Sprintf("unexpected token %T", tok))
	}
	return nil
}
