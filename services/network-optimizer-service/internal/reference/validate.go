package reference

import "strings"

func ValidateRule(rule Rule) error {
	if strings.TrimSpace(rule.RuleCode) == "" {
		return errRule("rule_code is required")
	}
	if strings.TrimSpace(rule.ReasonCode) == "" {
		return errRule("reason_code is required")
	}
	switch rule.RuleKind {
	case "CARGO_CARGO", "CARGO_EQUIPMENT":
	default:
		return errRule("invalid rule_kind")
	}
	switch rule.Layer {
	case "REGULATORY", "PLATFORM", "TENANT":
	default:
		return errRule("invalid rule layer")
	}
	switch rule.Decision {
	case "ALLOW", "DENY", "REQUIRE_SEPARATION", "REQUIRE_CONDITION":
	default:
		return errRule("invalid decision")
	}
	if rule.Layer == "REGULATORY" && (rule.SourceReference == nil || strings.TrimSpace(*rule.SourceReference) == "") {
		return errRule("REGULATORY rule requires source_reference")
	}
	left := cargoSelectors
	right := cargoSelectors
	if rule.RuleKind == "CARGO_EQUIPMENT" {
		right = equipmentSelectors
	}
	if !validSelector(rule.LeftSelectorType, rule.LeftSelectorValue, left) {
		return errRule("invalid left selector")
	}
	if !validSelector(rule.RightSelectorType, rule.RightSelectorValue, right) {
		return errRule("invalid right selector")
	}
	return nil
}

func validSelector(kind, value string, allowed map[string]struct{}) bool {
	if _, ok := allowed[kind]; !ok {
		return false
	}
	if kind == "ANY" {
		return true
	}
	return strings.TrimSpace(value) != ""
}

var cargoSelectors = map[string]struct{}{
	"ANY": {}, "CARGO_TYPE": {}, "PARENT": {}, "TAG": {},
	"ODOR_EMISSION_CLASS": {}, "ODOR_SENSITIVE": {}, "CONTAMINATION_CLASS": {}, "HAZARD_CLASS": {},
}

var equipmentSelectors = map[string]struct{}{
	"ANY": {}, "BODY_TYPE": {}, "EQUIPMENT_TYPE": {}, "UNIT_KIND": {},
}

type ruleError string

func (e ruleError) Error() string { return string(e) }

func errRule(message string) error { return ruleError(message) }
