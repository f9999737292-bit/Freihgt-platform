package erpjson

import (
	"encoding/json"
	"strings"
)

var allowedTopLevel = map[string]struct{}{
	"schema_version":      {},
	"requested_operation": {},
	"external":            {},
	"event":               {},
	"lots":                {},
	"questionnaire":       {},
	"template_reference":  {},
	"extensions":          {},
}

var rejectedTopLevel = map[string]struct{}{
	"lanes":   {},
	"cargo":   {},
	"routes":  {},
	"mapping_context": {},
}

func CheckTopLevelAllowlist(raw json.RawMessage) []Issue {
	var top map[string]json.RawMessage
	if err := json.Unmarshal(raw, &top); err != nil {
		return IngestErrorToIssues(err)
	}
	var issues []Issue
	for key := range top {
		if _, rejected := rejectedTopLevel[key]; rejected {
			issues = append(issues, errIssue(MachineCodeUnsupportedFieldV1, key, "rfx.erp.unsupported_field_v1"))
			continue
		}
		if _, ok := allowedTopLevel[key]; !ok {
			issues = append(issues, errIssue(MachineCodeUnknownField, key, "rfx.erp.unknown_field"))
		}
	}
	return issues
}

func normalizeOperation(op string) string {
	return strings.ToUpper(strings.TrimSpace(op))
}
