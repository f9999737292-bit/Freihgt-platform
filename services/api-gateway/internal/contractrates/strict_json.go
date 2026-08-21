package contractrates

import (
	"bytes"
	"encoding/json"
	"io"

	apperrors "github.com/freight-platform/api-gateway/internal/platform/errors"
)

func decodeStrictJSON(raw []byte, target any) error {
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.DisallowUnknownFields()
	if err := dec.Decode(target); err != nil {
		return apperrors.Validation("invalid request body", map[string]any{"field": "body"})
	}
	if err := dec.Decode(&struct{}{}); err != io.EOF {
		return apperrors.Validation("invalid request body", map[string]any{"field": "body"})
	}
	return nil
}

func requireNonEmptyBody(raw []byte) error {
	if len(bytes.TrimSpace(raw)) == 0 {
		return apperrors.Validation("request body is required", map[string]any{"field": "body"})
	}
	return nil
}

func rejectNonEmptyBody(raw []byte) error {
	if len(bytes.TrimSpace(raw)) == 0 {
		return nil
	}
	var empty struct{}
	if err := decodeStrictJSON(raw, &empty); err != nil {
		return err
	}
	return nil
}

func marshalSanitized(v any) ([]byte, error) {
	out, err := json.Marshal(v)
	if err != nil {
		return nil, apperrors.Validation("invalid request body", map[string]any{"field": "body"})
	}
	return out, nil
}
