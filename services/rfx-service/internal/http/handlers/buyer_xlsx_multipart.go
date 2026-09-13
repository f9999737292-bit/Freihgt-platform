package handlers

import (
	"errors"
	"io"
	"mime"
	"mime/multipart"
	"net/http"
	"strings"

	apperrors "github.com/freight-platform/rfx-service/internal/platform/errors"
	"github.com/freight-platform/rfx-service/internal/xlsxsecurity"
)

const buyerXlsxImportFormField = "file"

// buyerXlsxMultipartOverheadBytes reserves space for boundaries and part headers above the 5 MiB file cap.
const buyerXlsxMultipartOverheadBytes = 1 << 20

// MaxBuyerXlsxImportRequestBytes is the maximum multipart request body size accepted by preview upload.
const MaxBuyerXlsxImportRequestBytes = xlsxsecurity.DefaultMaxUploadBytes + buyerXlsxMultipartOverheadBytes

// ReadBuyerXlsxImportFile reads exactly one multipart file field without temp-file persistence.
func ReadBuyerXlsxImportFile(w http.ResponseWriter, r *http.Request) ([]byte, error) {
	contentType := strings.TrimSpace(r.Header.Get("Content-Type"))
	if contentType == "" {
		return nil, apperrors.Validation("multipart content type is required", map[string]any{
			"field":        "content_type",
			"machine_code": xlsxexchangeMachineCode("invalid_multipart"),
		})
	}
	mediaType, params, err := mime.ParseMediaType(contentType)
	if err != nil || !strings.EqualFold(mediaType, "multipart/form-data") {
		return nil, apperrors.Validation("multipart form data is required", map[string]any{
			"field":        "content_type",
			"machine_code": xlsxexchangeMachineCode("invalid_multipart"),
		})
	}
	boundary, ok := params["boundary"]
	if !ok || strings.TrimSpace(boundary) == "" {
		return nil, apperrors.Validation("multipart boundary is required", map[string]any{
			"field":        "content_type",
			"machine_code": xlsxexchangeMachineCode("invalid_multipart"),
		})
	}

	r.Body = http.MaxBytesReader(w, r.Body, MaxBuyerXlsxImportRequestBytes)
	reader := multipart.NewReader(r.Body, boundary)

	var fileBytes []byte
	var fileParts int
	for {
		part, partErr := reader.NextPart()
		if errors.Is(partErr, io.EOF) {
			break
		}
		if partErr != nil {
			if isRequestBodyTooLarge(partErr) {
				return nil, apperrors.RequestBodyTooLarge("request body is too large")
			}
			return nil, apperrors.Validation("invalid multipart payload", map[string]any{
				"field":        "file",
				"machine_code": xlsxexchangeMachineCode("invalid_multipart"),
			})
		}

		name := part.FormName()
		switch name {
		case buyerXlsxImportFormField:
			fileParts++
			if fileParts > 1 {
				_ = part.Close()
				return nil, apperrors.Validation("duplicate file part", map[string]any{
					"field":        "file",
					"machine_code": xlsxexchangeMachineCode("invalid_multipart"),
				})
			}
			limited := io.LimitReader(part, xlsxsecurity.DefaultMaxUploadBytes+1)
			data, readErr := io.ReadAll(limited)
			_ = part.Close()
			if readErr != nil {
				if isRequestBodyTooLarge(readErr) {
					return nil, apperrors.RequestBodyTooLarge("request body is too large")
				}
				return nil, apperrors.Validation("failed to read uploaded file", map[string]any{
					"field":        "file",
					"machine_code": xlsxexchangeMachineCode("invalid_multipart"),
				})
			}
			if int64(len(data)) > xlsxsecurity.DefaultMaxUploadBytes {
				return nil, apperrors.RequestBodyTooLarge("uploaded file is too large")
			}
			fileBytes = data
		default:
			_ = part.Close()
			return nil, apperrors.Validation("unexpected multipart field", map[string]any{
				"field":        name,
				"machine_code": xlsxexchangeMachineCode("invalid_multipart"),
			})
		}
	}

	if fileParts == 0 {
		return nil, apperrors.Validation("file part is required", map[string]any{
			"field":        "file",
			"machine_code": xlsxexchangeMachineCode("invalid_multipart"),
		})
	}
	if len(fileBytes) == 0 {
		return nil, apperrors.Validation("uploaded file is empty", map[string]any{
			"field":        "file",
			"machine_code": xlsxexchangeMachineCode("invalid_multipart"),
		})
	}
	return fileBytes, nil
}

func isRequestBodyTooLarge(err error) bool {
	var maxErr *http.MaxBytesError
	return errors.As(err, &maxErr)
}

func xlsxexchangeMachineCode(code string) string {
	return code
}
