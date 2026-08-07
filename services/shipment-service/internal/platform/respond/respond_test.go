package respond

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	apperrors "github.com/freight-platform/shipment-service/internal/platform/errors"
)

func TestErrorHTTPMapping(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name   string
		err    error
		status int
		code   apperrors.Code
	}{
		{name: "validation", err: apperrors.Validation("bad input", nil), status: http.StatusBadRequest, code: apperrors.CodeValidation},
		{name: "not found", err: apperrors.NotFound("missing"), status: http.StatusNotFound, code: apperrors.CodeNotFound},
		{name: "conflict", err: apperrors.Conflict("conflict", nil), status: http.StatusConflict, code: apperrors.CodeConflict},
		{name: "unauthorized", err: apperrors.Unauthorized("tenant context is required"), status: http.StatusUnauthorized, code: apperrors.CodeUnauthorized},
		{name: "internal", err: apperrors.Internal("db", errors.New("boom")), status: http.StatusInternalServerError, code: apperrors.CodeInternal},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			rec := httptest.NewRecorder()
			Error(rec, tc.err)
			if rec.Code != tc.status {
				t.Fatalf("status=%d want %d body=%s", rec.Code, tc.status, rec.Body.String())
			}
			var payload struct {
				Error struct {
					Code    string `json:"code"`
					Message string `json:"message"`
				} `json:"error"`
			}
			if err := json.NewDecoder(rec.Body).Decode(&payload); err != nil {
				t.Fatalf("decode: %v", err)
			}
			if payload.Error.Code != string(tc.code) {
				t.Fatalf("code=%q want %q", payload.Error.Code, tc.code)
			}
		})
	}
}
