package erpjson

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"sort"
)

func StableHash(payload []byte) (string, error) {
	normalized, err := NormalizeJSON(payload)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(normalized)
	return hex.EncodeToString(sum[:]), nil
}

func NormalizeJSON(payload []byte) ([]byte, error) {
	var v any
	if err := json.Unmarshal(payload, &v); err != nil {
		return nil, err
	}
	sorted := sortJSONValue(v)
	return json.Marshal(sorted)
}

func sortJSONValue(v any) any {
	switch t := v.(type) {
	case map[string]any:
		keys := make([]string, 0, len(t))
		for k := range t {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		out := make(map[string]any, len(t))
		for _, k := range keys {
			out[k] = sortJSONValue(t[k])
		}
		return out
	case []any:
		out := make([]any, len(t))
		for i, item := range t {
			out[i] = sortJSONValue(item)
		}
		return out
	default:
		return v
	}
}
