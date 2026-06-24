package domain

import (
	"encoding/json"
	"strings"

	apperrors "github.com/freight-platform/low-code-service/internal/platform/errors"
)

// Import top-level keys allowed for preview/execute (export envelope shortcut included).
var allowedImportTopLevelKeys = map[string]struct{}{
	"schema_version":      {},
	"mode":                {},
	"conflict_strategy":   {},
	"target_code":         {},
	"allow_system_fields": {},
	"template":            {},
	"source_metadata":     {},
	"source":              {},
	"exported_at":         {},
	"metadata":            {},
}

// importTopLevelKeysRejected are never accepted even if nested elsewhere at top level.
var importTopLevelKeysRejected = map[string]struct{}{
	"custom_values": {},
	"audit_events":  {},
	"values":        {},
	"execute":       {},
	"script":        {},
	"publish":       {},
}

func validateImportTopLevelKeys(raw map[string]json.RawMessage) error {
	for key := range raw {
		if _, rejected := importTopLevelKeysRejected[key]; rejected {
			return apperrors.Validation(
				"import payload contains forbidden field",
				map[string]any{"field": key, "reason": "forbidden_import_key"},
			)
		}
		if _, ok := allowedImportTopLevelKeys[key]; !ok {
			return apperrors.Validation(
				"unsupported import field",
				map[string]any{"field": key, "reason": "unknown_top_level_key"},
			)
		}
	}
	return nil
}

// ChecksumImportWarning returns a preview warning when export checksum is present but mismatched.
// Missing checksum is allowed (manual payloads, legacy exports).
func ChecksumImportWarning(template ExportedFormTemplate, expectedChecksum string) (string, error) {
	expected := strings.TrimSpace(expectedChecksum)
	if expected == "" {
		return "", nil
	}
	actual, err := ComputeTemplateExportChecksum(template)
	if err != nil {
		return "", err
	}
	if strings.EqualFold(actual, expected) {
		return "", nil
	}
	return "export checksum mismatch: template content may have been modified after export", nil
}

func appendImportPreviewWarning(result *TemplateImportPreviewResult, warning string) {
	if strings.TrimSpace(warning) == "" {
		return
	}
	if result.Status == ImportPreviewStatusReady {
		result.Status = ImportPreviewStatusWarning
	}
	result.Warnings = append(result.Warnings, warning)
}
