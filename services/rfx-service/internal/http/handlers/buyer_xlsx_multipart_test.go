package handlers

import (
	"bytes"
	"errors"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"testing"

	apperrors "github.com/freight-platform/rfx-service/internal/platform/errors"
	"github.com/freight-platform/rfx-service/internal/xlsxsecurity"
)

func TestReadBuyerXlsxImportFileSuccess(t *testing.T) {
	t.Parallel()
	payload := []byte("PK\x03\x04test")
	body, contentType := buildMultipartBody(t, "file", payload, false, false)

	req := httptest.NewRequest(http.MethodPost, "/preview", body)
	req.Header.Set("Content-Type", contentType)
	rec := httptest.NewRecorder()

	got, err := ReadBuyerXlsxImportFile(rec, req)
	if err != nil {
		t.Fatalf("read file: %v", err)
	}
	if !bytes.Equal(got, payload) {
		t.Fatalf("payload mismatch")
	}
}

func TestReadBuyerXlsxImportFileMissingAndDuplicate(t *testing.T) {
	t.Parallel()
	payload := []byte("PK\x03\x04test")

	t.Run("missing", func(t *testing.T) {
		body := &bytes.Buffer{}
		writer := multipart.NewWriter(body)
		_ = writer.Close()
		req := httptest.NewRequest(http.MethodPost, "/preview", body)
		req.Header.Set("Content-Type", writer.FormDataContentType())
		rec := httptest.NewRecorder()
		if _, err := ReadBuyerXlsxImportFile(rec, req); err == nil {
			t.Fatal("expected missing file error")
		}
	})

	t.Run("duplicate", func(t *testing.T) {
		body, contentType := buildMultipartBody(t, "file", payload, true, false)
		req := httptest.NewRequest(http.MethodPost, "/preview", body)
		req.Header.Set("Content-Type", contentType)
		rec := httptest.NewRecorder()
		if _, err := ReadBuyerXlsxImportFile(rec, req); err == nil {
			t.Fatal("expected duplicate file error")
		}
	})
}

func TestReadBuyerXlsxImportFileOversized413(t *testing.T) {
	t.Parallel()
	payload := bytes.Repeat([]byte("A"), int(xlsxsecurity.DefaultMaxUploadBytes)+1)
	body, contentType := buildMultipartBody(t, "file", payload, false, false)
	req := httptest.NewRequest(http.MethodPost, "/preview", body)
	req.Header.Set("Content-Type", contentType)
	rec := httptest.NewRecorder()

	_, err := ReadBuyerXlsxImportFile(rec, req)
	if err == nil {
		t.Fatal("expected oversize error")
	}
	var appErr *apperrors.AppError
	if !errors.As(err, &appErr) || appErr.Code != apperrors.CodeRequestBodyTooLarge {
		t.Fatalf("expected request body too large, got %v", err)
	}
}

func buildMultipartBody(t *testing.T, field string, payload []byte, duplicate, extra bool) (*bytes.Buffer, string) {
	t.Helper()
	var buf bytes.Buffer
	writer := multipart.NewWriter(&buf)
	if extra {
		if err := writer.WriteField("unexpected", "x"); err != nil {
			t.Fatalf("write field: %v", err)
		}
	}
	writeFile := func() {
		part, err := writer.CreateFormFile(field, "import.xlsx")
		if err != nil {
			t.Fatalf("create form file: %v", err)
		}
		if _, err := part.Write(payload); err != nil {
			t.Fatalf("write payload: %v", err)
		}
	}
	writeFile()
	if duplicate {
		writeFile()
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("close writer: %v", err)
	}
	return &buf, writer.FormDataContentType()
}
