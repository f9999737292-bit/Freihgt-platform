package erpjson

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"strings"
)

// IngestJSON validates size, depth, duplicate keys, then decodes into v.
func IngestJSON(raw []byte, v any) error {
	if len(raw) > MaxBodyBytes {
		return fmt.Errorf("body too large")
	}
	if err := validateStructure(raw, MaxJSONDepth); err != nil {
		return err
	}
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.DisallowUnknownFields()
	if err := dec.Decode(v); err != nil {
		return err
	}
	if err := dec.Decode(&struct{}{}); err != io.EOF {
		return fmt.Errorf("trailing json")
	}
	return nil
}

func validateStructure(raw []byte, maxDepth int) error {
	dec := json.NewDecoder(bytes.NewReader(raw))
	return walk(dec, 0, maxDepth)
}

func walk(dec *json.Decoder, depth, maxDepth int) error {
	tok, err := dec.Token()
	if err != nil {
		return err
	}
	switch delim := tok.(type) {
	case json.Delim:
		switch delim {
		case '{':
			if depth >= maxDepth {
				return fmt.Errorf("json depth exceeded")
			}
			keys := make(map[string]struct{})
			for dec.More() {
				keyTok, keyErr := dec.Token()
				if keyErr != nil {
					return keyErr
				}
				key, ok := keyTok.(string)
				if !ok {
					return fmt.Errorf("invalid object key")
				}
				if _, dup := keys[key]; dup {
					return fmt.Errorf("duplicate key %q", key)
				}
				keys[key] = struct{}{}
				if err := walk(dec, depth+1, maxDepth); err != nil {
					return err
				}
			}
			if _, err := dec.Token(); err != nil {
				return err
			}
		case '[':
			if depth >= maxDepth {
				return fmt.Errorf("json depth exceeded")
			}
			for dec.More() {
				if err := walk(dec, depth+1, maxDepth); err != nil {
					return err
				}
			}
			if _, err := dec.Token(); err != nil {
				return err
			}
		}
	}
	return nil
}

func IngestErrorToIssues(err error) []Issue {
	if err == nil {
		return nil
	}
	msg := err.Error()
	switch {
	case msg == "body too large":
		return []Issue{errIssue(MachineCodeInvalidType, "", "rfx.erp.payload_too_large")}
	case msg == "json depth exceeded":
		return []Issue{errIssue(MachineCodeJSONDepthExceeded, "", "rfx.erp.json_depth_exceeded")}
	case len(msg) >= 14 && msg[:14] == "duplicate key ":
		path := msg[14:]
		if len(path) >= 2 && path[0] == '"' {
			path = path[1 : len(path)-1]
		}
		return []Issue{errIssue(MachineCodeDuplicateField, path, "rfx.erp.duplicate_field")}
	case msg == "trailing json":
		return []Issue{errIssue(MachineCodeInvalidType, "", "rfx.erp.invalid_json")}
	case strings.HasPrefix(msg, `json: unknown field "`):
		field := strings.TrimSuffix(strings.TrimPrefix(msg, `json: unknown field "`), `"`)
		return []Issue{errIssue(MachineCodeUnknownField, field, "rfx.erp.unknown_field")}
	default:
		return []Issue{errIssue(MachineCodeInvalidType, "", "rfx.erp.invalid_json")}
	}
}

func prefixIssuePaths(issues []Issue, prefix string) []Issue {
	if prefix == "" {
		return issues
	}
	out := make([]Issue, len(issues))
	copy(out, issues)
	for i := range out {
		if out[i].Path == "" {
			out[i].Path = prefix
			continue
		}
		out[i].Path = prefix + "." + out[i].Path
	}
	return out
}
