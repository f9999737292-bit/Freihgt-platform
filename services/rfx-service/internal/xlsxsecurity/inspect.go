package xlsxsecurity

import (
	"archive/zip"
	"bytes"
	"encoding/xml"
	"fmt"
	"io"
	"path"
	"strings"

	apperrors "github.com/freight-platform/rfx-service/internal/platform/errors"
)

const (
	DefaultMaxUploadBytes   = 5 << 20 // 5 MiB
	DefaultMaxZipEntries    = 256
	DefaultMaxExpandedBytes = 50 << 20 // 50 MiB
	DefaultMaxZipRatio      = 100.0
)

var xlsxZipSignature = []byte{0x50, 0x4B, 0x03, 0x04}

var forbiddenEntrySubstrings = []string{
	"vbaProject.bin",
	"xl/externalLinks/",
	"externalLinks/",
	"xl/embeddings/",
	"embeddings/",
	"activeX/",
	"customUI/",
	"macrosheets/",
}

type Limits struct {
	MaxUploadBytes   int64
	MaxZipEntries    int
	MaxExpandedBytes int64
	MaxZipRatio      float64
}

func DefaultLimits() Limits {
	return Limits{
		MaxUploadBytes:   DefaultMaxUploadBytes,
		MaxZipEntries:    DefaultMaxZipEntries,
		MaxExpandedBytes: DefaultMaxExpandedBytes,
		MaxZipRatio:      DefaultMaxZipRatio,
	}
}

type InspectionResult struct {
	EntryCount     int
	ExpandedBytes  int64
	WorksheetCount int
}

func InspectUpload(contentType string, data []byte, limits Limits) (InspectionResult, error) {
	if limits.MaxUploadBytes <= 0 {
		limits = DefaultLimits()
	}
	if int64(len(data)) > limits.MaxUploadBytes {
		return InspectionResult{}, apperrors.Validation("xlsx file exceeds max upload size", map[string]any{
			"field": "file", "max_bytes": limits.MaxUploadBytes,
		})
	}
	if !isXLSXContentType(contentType) {
		return InspectionResult{}, apperrors.Validation("unsupported content type for xlsx upload", map[string]any{
			"field": "content_type", "value": contentType,
		})
	}
	if len(data) < len(xlsxZipSignature) || !bytes.Equal(data[:len(xlsxZipSignature)], xlsxZipSignature) {
		return InspectionResult{}, apperrors.Validation("file is not a valid xlsx zip package", map[string]any{
			"field": "file", "reason": "invalid_zip_signature",
		})
	}
	reader, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return InspectionResult{}, apperrors.Validation("malformed xlsx zip package", map[string]any{
			"field": "file", "reason": "zip_open_failed",
		})
	}
	if len(reader.File) == 0 {
		return InspectionResult{}, apperrors.Validation("empty xlsx zip package", map[string]any{"field": "file"})
	}
	if len(reader.File) > limits.MaxZipEntries {
		return InspectionResult{}, apperrors.Validation("xlsx zip entry count exceeds limit", map[string]any{
			"field": "file", "entry_count": len(reader.File), "max_entries": limits.MaxZipEntries,
		})
	}

	seen := make(map[string]struct{}, len(reader.File))
	var expanded int64
	worksheetCount := 0
	hasContentTypes := false

	for _, file := range reader.File {
		normalized, err := normalizeZipEntryName(file.Name)
		if err != nil {
			return InspectionResult{}, err
		}
		if _, ok := seen[normalized]; ok {
			return InspectionResult{}, apperrors.Validation("duplicate zip entry", map[string]any{
				"field": "file", "entry": normalized,
			})
		}
		seen[normalized] = struct{}{}
		if err := rejectForbiddenEntry(normalized); err != nil {
			return InspectionResult{}, err
		}
		if strings.HasPrefix(normalized, "xl/worksheets/sheet") && strings.HasSuffix(normalized, ".xml") {
			worksheetCount++
		}
		if normalized == "[Content_Types].xml" {
			hasContentTypes = true
		}

		uncompressed := file.UncompressedSize64
		if uncompressed == 0 {
			uncompressed = uint64(file.UncompressedSize)
		}
		expanded += int64(uncompressed)
		if expanded > limits.MaxExpandedBytes {
			return InspectionResult{}, apperrors.Validation("xlsx expanded size exceeds limit", map[string]any{
				"field": "file", "expanded_bytes": expanded, "max_expanded_bytes": limits.MaxExpandedBytes,
			})
		}
		if file.CompressedSize64 > 0 && float64(uncompressed)/float64(file.CompressedSize64) > limits.MaxZipRatio {
			return InspectionResult{}, apperrors.Validation("xlsx compression ratio exceeds limit", map[string]any{
				"field": "file", "entry": normalized, "max_ratio": limits.MaxZipRatio,
			})
		}

		if strings.HasPrefix(normalized, "xl/worksheets/") && strings.HasSuffix(normalized, ".xml") {
			if err := inspectWorksheetForFormulas(file); err != nil {
				return InspectionResult{}, err
			}
		}
		if normalized == "[Content_Types].xml" {
			if err := inspectContentTypes(file); err != nil {
				return InspectionResult{}, err
			}
		}
		if normalized == "xl/workbook.xml" {
			if err := inspectWorkbookDefinedNames(file); err != nil {
				return InspectionResult{}, err
			}
		}
	}

	if !hasContentTypes {
		return InspectionResult{}, apperrors.Validation("xlsx missing [Content_Types].xml", map[string]any{"field": "file"})
	}

	return InspectionResult{
		EntryCount:     len(reader.File),
		ExpandedBytes:  expanded,
		WorksheetCount: worksheetCount,
	}, nil
}

func isXLSXContentType(contentType string) bool {
	ct := strings.ToLower(strings.TrimSpace(strings.Split(contentType, ";")[0]))
	switch ct {
	case "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet",
		"application/octet-stream",
		"application/zip",
		"":
		return true
	default:
		return false
	}
}

func normalizeZipEntryName(name string) (string, error) {
	clean := path.Clean(strings.ReplaceAll(name, "\\", "/"))
	clean = strings.TrimPrefix(clean, "./")
	if clean == "." || clean == ".." {
		return "", apperrors.Validation("invalid zip entry path", map[string]any{"field": "file", "entry": name})
	}
	if strings.HasPrefix(clean, "../") || strings.Contains(clean, "/../") {
		return "", apperrors.Validation("zip entry path traversal rejected", map[string]any{"field": "file", "entry": name})
	}
	if strings.HasPrefix(clean, "/") {
		return "", apperrors.Validation("absolute zip entry path rejected", map[string]any{"field": "file", "entry": name})
	}
	return clean, nil
}

func rejectForbiddenEntry(normalized string) error {
	lower := strings.ToLower(normalized)
	for _, forbidden := range forbiddenEntrySubstrings {
		if strings.Contains(lower, strings.ToLower(forbidden)) {
			return apperrors.Validation("forbidden xlsx package content", map[string]any{
				"field": "file", "entry": normalized, "reason": forbidden,
			})
		}
	}
	return nil
}

func inspectWorksheetForFormulas(file *zip.File) error {
	rc, err := file.Open()
	if err != nil {
		return apperrors.Validation("unable to read worksheet xml", map[string]any{"field": "file", "entry": file.Name})
	}
	defer rc.Close()
	decoder := xml.NewDecoder(io.LimitReader(rc, 1<<20))
	for {
		token, err := decoder.Token()
		if err == io.EOF {
			return nil
		}
		if err != nil {
			return apperrors.Validation("malformed worksheet xml", map[string]any{"field": "file", "entry": file.Name})
		}
		if start, ok := token.(xml.StartElement); ok && start.Name.Local == "f" {
			return apperrors.Validation("xlsx formulas are not allowed", map[string]any{
				"field": "file", "entry": file.Name,
			})
		}
	}
}

func inspectContentTypes(file *zip.File) error {
	rc, err := file.Open()
	if err != nil {
		return apperrors.Validation("unable to read [Content_Types].xml", map[string]any{"field": "file"})
	}
	defer rc.Close()
	body, err := io.ReadAll(io.LimitReader(rc, 1<<20))
	if err != nil {
		return apperrors.Validation("malformed [Content_Types].xml", map[string]any{"field": "file"})
	}
	lower := strings.ToLower(string(body))
	for _, marker := range []string{"vba", "macro", "activex", "oleobject", "externalLink"} {
		if strings.Contains(lower, marker) {
			return apperrors.Validation("forbidden xlsx content type", map[string]any{
				"field": "file", "reason": marker,
			})
		}
	}
	return nil
}

func inspectWorkbookDefinedNames(file *zip.File) error {
	rc, err := file.Open()
	if err != nil {
		return apperrors.Validation("unable to read workbook xml", map[string]any{"field": "file", "entry": file.Name})
	}
	defer rc.Close()
	body, err := io.ReadAll(io.LimitReader(rc, 1<<20))
	if err != nil {
		return apperrors.Validation("malformed workbook xml", map[string]any{"field": "file", "entry": file.Name})
	}
	lower := strings.ToLower(string(body))
	if !strings.Contains(lower, "definedname") {
		return nil
	}
	decoder := xml.NewDecoder(bytes.NewReader(body))
	for {
		token, err := decoder.Token()
		if err == io.EOF {
			return nil
		}
		if err != nil {
			return apperrors.Validation("malformed workbook xml", map[string]any{"field": "file", "entry": file.Name})
		}
		if start, ok := token.(xml.StartElement); ok && strings.EqualFold(start.Name.Local, "definedName") {
			var value string
			if err := decoder.DecodeElement(&value, &start); err != nil {
				return apperrors.Validation("malformed defined name", map[string]any{"field": "file", "entry": file.Name})
			}
			trimmed := strings.TrimSpace(value)
			if trimmed == "" {
				continue
			}
			if strings.HasPrefix(trimmed, "=") || (strings.Contains(trimmed, "[") && strings.Contains(trimmed, "]")) {
				return apperrors.Validation("workbook defined names with formulas or external references are not allowed", map[string]any{
					"field": "file", "entry": file.Name, "reason": "defined_name_formula_or_external",
				})
			}
		}
	}
}

func TempFilePattern(workbookType string) string {
	return fmt.Sprintf("bintrans-rfx-%s-*.xlsx", strings.ToLower(strings.TrimSpace(workbookType)))
}
