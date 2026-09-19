package handlers

import (
	"errors"
	"net/http"
	"strings"
	"testing"

	"github.com/google/uuid"

	"github.com/freight-platform/rfx-service/internal/erpjson"
	apperrors "github.com/freight-platform/rfx-service/internal/platform/errors"
)

func TestRequireERPJSONContentTypeAcceptsJSONAndUTF8(t *testing.T) {
	t.Parallel()
	for _, ct := range []string{
		"application/json",
		"application/json; charset=utf-8",
		"application/json; charset=UTF-8",
		"Application/JSON; charset=Utf-8",
	} {
		req := httptestRequest(ct)
		if err := requireERPJSONContentType(req); err != nil {
			t.Fatalf("content-type %q should be accepted: %v", ct, err)
		}
	}
}

func TestRequireERPJSONContentTypeRejectsInvalidMediaTypes(t *testing.T) {
	t.Parallel()
	for _, ct := range []string{
		"",
		"application/json;;",
		"text/plain",
		"application/x-www-form-urlencoded",
		"multipart/form-data; boundary=x",
		"application/json; charset=utf-16",
	} {
		req := httptestRequest(ct)
		err := requireERPJSONContentType(req)
		if err == nil {
			t.Fatalf("content-type %q should be rejected", ct)
		}
		var appErr *apperrors.AppError
		if !errors.As(err, &appErr) || appErr.Code != apperrors.CodeValidation {
			t.Fatalf("content-type %q: want VALIDATION_ERROR, got %v", ct, err)
		}
		if appErr.Details["field"] != "content_type" || appErr.Details["machine_code"] != erpjson.MachineCodeUnsupportedMediaType {
			t.Fatalf("content-type %q: details=%v", ct, appErr.Details)
		}
		if appErr.Details["message_key"] != "rfx.erp.unsupported_media_type" {
			t.Fatalf("content-type %q: message_key=%v", ct, appErr.Details["message_key"])
		}
		if strings.Contains(strings.ToLower(appErr.Message), "{") || strings.Contains(appErr.Error(), "schema_version") {
			t.Fatalf("content-type %q leaked payload: %s", ct, appErr.Error())
		}
	}
}

func TestReadERPCreateCommitBodyRequiresSingleJSONValue(t *testing.T) {
	t.Parallel()
	analysisID := uuid.New()
	valid := `{"analysis_id":"` + analysisID.String() + `"}`
	cases := []struct {
		name    string
		body    string
		wantErr bool
	}{
		{name: "valid", body: valid, wantErr: false},
		{name: "trailing_whitespace", body: valid + "  \n\t", wantErr: false},
		{name: "second_object", body: valid + `{"extra":1}`, wantErr: true},
		{name: "second_scalar", body: valid + `1`, wantErr: true},
		{name: "corrupt_remainder", body: valid + `{`, wantErr: true},
		{name: "unknown_field", body: `{"analysis_id":"` + analysisID.String() + `","extra":1}`, wantErr: true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			req, err := http.NewRequest(http.MethodPost, "/v1/integrations/erp/rfx/drafts/commit", strings.NewReader(tc.body))
			if err != nil {
				t.Fatalf("new request: %v", err)
			}
			req.Header.Set("Content-Type", "application/json")
			in, readErr := readERPCreateCommitBody(req)
			if tc.wantErr {
				if readErr == nil {
					t.Fatal("expected validation error")
				}
				var appErr *apperrors.AppError
				if !errors.As(readErr, &appErr) || appErr.Code != apperrors.CodeValidation {
					t.Fatalf("want VALIDATION_ERROR, got %v", readErr)
				}
				return
			}
			if readErr != nil {
				t.Fatalf("unexpected error: %v", readErr)
			}
			if in.AnalysisID != analysisID {
				t.Fatalf("analysis_id=%s", in.AnalysisID)
			}
		})
	}
}

func httptestRequest(contentType string) *http.Request {
	req, _ := http.NewRequest(http.MethodPost, "/v1/integrations/erp/rfx/drafts/preview", strings.NewReader(`{"schema_version":"BINTRANS_RFX_ERP_JSON_V1"}`))
	if contentType != "" {
		req.Header.Set("Content-Type", contentType)
	}
	return req
}
