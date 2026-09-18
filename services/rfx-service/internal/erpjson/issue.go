package erpjson

const (
	SeverityError   = "error"
	SeverityWarning = "warning"

	MachineCodeUnknownField        = "unknown_field"
	MachineCodeUnsupportedFieldV1  = "unsupported_field_v1"
	MachineCodeDuplicateField      = "duplicate_field"
	MachineCodeJSONDepthExceeded   = "json_depth_exceeded"
	MachineCodeUnsupportedSchema   = "unsupported_schema"
	MachineCodeInvalidType         = "invalid_type"
	MachineCodeMissingRequired     = "missing_required_value"
	MachineCodeDuplicateSection    = "duplicate_section_code"
	MachineCodeCyclicRule          = "cyclic_rule"
	MachineCodeTooManyLots         = "too_many_lots"
	MachineCodeInvalidOperation    = "invalid_requested_operation"
	MachineCodeMappingNotFound     = "mapping_not_found"
	MachineCodeMappingRetired      = "mapping_retired"
	MachineCodeMappingAmbiguous    = "mapping_ambiguous"
)

type Issue struct {
	Severity       string         `json:"severity"`
	MachineCode    string         `json:"machine_code"`
	Path           string         `json:"path,omitempty"`
	MessageKey     string         `json:"message_key,omitempty"`
	ExternalSource string         `json:"external_source,omitempty"`
	Params         map[string]any `json:"params,omitempty"`
}

func errIssue(code, path, messageKey string) Issue {
	return Issue{
		Severity:    SeverityError,
		MachineCode: code,
		Path:        path,
		MessageKey:  messageKey,
	}
}

func warnIssue(code, path, messageKey string) Issue {
	return Issue{
		Severity:    SeverityWarning,
		MachineCode: code,
		Path:        path,
		MessageKey:  messageKey,
	}
}
