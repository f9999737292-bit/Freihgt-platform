package handlers

import (
	"errors"
	"io"
	"mime"
	"mime/multipart"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"

	apperrors "github.com/freight-platform/rfx-service/internal/platform/errors"
	"github.com/freight-platform/rfx-service/internal/service"
	"github.com/freight-platform/rfx-service/internal/xlsxsecurity"
)

var buyerXlsxCreateAllowedFields = map[string]struct{}{
	"file":              {},
	"owner_company_id":  {},
	"rfx_number":        {},
	"title":             {},
	"rfx_type":          {},
	"category":          {},
	"description":       {},
	"response_deadline": {},
	"currency_code":     {},
}

var buyerXlsxCreateRequiredFields = []string{
	"file",
	"owner_company_id",
	"rfx_number",
	"title",
	"rfx_type",
	"category",
}

// ReadBuyerXlsxCreatePreviewRequest reads CREATE multipart fields without temp-file persistence.
func ReadBuyerXlsxCreatePreviewRequest(w http.ResponseWriter, r *http.Request) (service.BuyerXlsxCreatePreviewInput, error) {
	var in service.BuyerXlsxCreatePreviewInput
	contentType := strings.TrimSpace(r.Header.Get("Content-Type"))
	if contentType == "" {
		return in, apperrors.Validation("multipart content type is required", map[string]any{
			"field":        "content_type",
			"machine_code": xlsxexchangeMachineCode("invalid_multipart"),
		})
	}
	mediaType, params, err := mime.ParseMediaType(contentType)
	if err != nil || !strings.EqualFold(mediaType, "multipart/form-data") {
		return in, apperrors.Validation("multipart form data is required", map[string]any{
			"field":        "content_type",
			"machine_code": xlsxexchangeMachineCode("invalid_multipart"),
		})
	}
	boundary, ok := params["boundary"]
	if !ok || strings.TrimSpace(boundary) == "" {
		return in, apperrors.Validation("multipart boundary is required", map[string]any{
			"field":        "content_type",
			"machine_code": xlsxexchangeMachineCode("invalid_multipart"),
		})
	}

	r.Body = http.MaxBytesReader(w, r.Body, MaxBuyerXlsxImportRequestBytes)
	reader := multipart.NewReader(r.Body, boundary)
	seen := make(map[string]int)
	var fileBytes []byte

	for {
		part, partErr := reader.NextPart()
		if errors.Is(partErr, io.EOF) {
			break
		}
		if partErr != nil {
			if isRequestBodyTooLarge(partErr) {
				return in, apperrors.RequestBodyTooLarge("request body is too large")
			}
			return in, apperrors.Validation("invalid multipart payload", map[string]any{
				"field":        "file",
				"machine_code": xlsxexchangeMachineCode("invalid_multipart"),
			})
		}
		name := part.FormName()
		if _, allowed := buyerXlsxCreateAllowedFields[name]; !allowed {
			_ = part.Close()
			return in, apperrors.Validation("unexpected multipart field", map[string]any{
				"field":        name,
				"machine_code": xlsxexchangeMachineCode("invalid_multipart"),
			})
		}
		seen[name]++
		if seen[name] > 1 {
			_ = part.Close()
			return in, apperrors.Validation("duplicate multipart field", map[string]any{
				"field":        name,
				"machine_code": xlsxexchangeMachineCode("invalid_multipart"),
			})
		}
		if name == buyerXlsxImportFormField {
			limited := io.LimitReader(part, xlsxsecurity.DefaultMaxUploadBytes+1)
			data, readErr := io.ReadAll(limited)
			_ = part.Close()
			if readErr != nil {
				if isRequestBodyTooLarge(readErr) {
					return in, apperrors.RequestBodyTooLarge("request body is too large")
				}
				return in, apperrors.Validation("failed to read uploaded file", map[string]any{
					"field":        "file",
					"machine_code": xlsxexchangeMachineCode("invalid_multipart"),
				})
			}
			if int64(len(data)) > xlsxsecurity.DefaultMaxUploadBytes {
				return in, apperrors.RequestBodyTooLarge("uploaded file is too large")
			}
			fileBytes = data
			continue
		}
		value, readErr := io.ReadAll(io.LimitReader(part, 4096))
		_ = part.Close()
		if readErr != nil {
			return in, apperrors.Validation("invalid multipart payload", map[string]any{
				"field":        name,
				"machine_code": xlsxexchangeMachineCode("invalid_multipart"),
			})
		}
		text := strings.TrimSpace(string(value))
		switch name {
		case "owner_company_id":
			parsed, parseErr := uuid.Parse(text)
			if parseErr != nil || parsed == uuid.Nil {
				return in, apperrors.Validation("invalid owner_company_id", map[string]any{"field": "owner_company_id"})
			}
			in.OwnerCompanyID = parsed
		case "rfx_number":
			in.RfxNumber = text
		case "title":
			in.Title = text
		case "rfx_type":
			in.RfxType = text
		case "category":
			in.Category = text
		case "description":
			if text != "" {
				in.Description = &text
			}
		case "response_deadline":
			if text == "" {
				continue
			}
			deadline, parseErr := time.Parse(time.RFC3339, text)
			if parseErr != nil {
				return in, apperrors.Validation("invalid response_deadline", map[string]any{"field": "response_deadline"})
			}
			utc := deadline.UTC()
			in.ResponseDeadline = &utc
		case "currency_code":
			if text != "" {
				in.CurrencyCode = &text
			}
		}
	}

	for _, field := range buyerXlsxCreateRequiredFields {
		if seen[field] == 0 {
			return in, apperrors.Validation(field+" is required", map[string]any{
				"field":        field,
				"machine_code": xlsxexchangeMachineCode("invalid_multipart"),
			})
		}
	}
	if len(fileBytes) == 0 {
		return in, apperrors.Validation("uploaded file is empty", map[string]any{
			"field":        "file",
			"machine_code": xlsxexchangeMachineCode("invalid_multipart"),
		})
	}
	in.WorkbookBytes = fileBytes
	return in, nil
}
