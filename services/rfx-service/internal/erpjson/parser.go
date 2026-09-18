package erpjson

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"unicode/utf8"

	"github.com/google/uuid"
	"golang.org/x/text/unicode/norm"

	"github.com/freight-platform/rfx-service/internal/domain"
)

type MappingLookup interface {
	ResolveCode(ctx context.Context, tenantID uuid.UUID, mappingType, externalCode string) (canonical string, mappingSet domain.ReferenceMappingSet, warning bool, err error)
}

func ParsePreview(ctx context.Context, tenantID uuid.UUID, expectedOp string, raw []byte, maps MappingLookup) (*ParsedPreview, error) {
	if len(raw) > MaxBodyBytes {
		return nil, fmt.Errorf("body too large")
	}
	if err := validateStructure(raw, MaxJSONDepth); err != nil {
		out := &ParsedPreview{Errors: IngestErrorToIssues(err)}
		return out, nil
	}
	if issues := CheckTopLevelAllowlist(raw); len(issues) > 0 {
		return &ParsedPreview{Errors: issues}, nil
	}
	var doc RawDocument
	if err := json.Unmarshal(raw, &doc); err != nil {
		return &ParsedPreview{Errors: IngestErrorToIssues(err)}, nil
	}

	out := &ParsedPreview{Operation: normalizeOperation(doc.RequestedOperation)}
	var issues []Issue
	var warnings []Issue

	if doc.SchemaVersion != domain.SchemaVersionERPJSONV1 {
		issues = append(issues, errIssue(MachineCodeUnsupportedSchema, "schema_version", "rfx.erp.unsupported_schema"))
	}
	op := out.Operation
	if op != expectedOp {
		issues = append(issues, errIssue(MachineCodeInvalidOperation, "requested_operation", "rfx.erp.invalid_requested_operation"))
	}
	if expectedOp == OperationCreateDraft {
		if doc.External == nil {
			issues = append(issues, errIssue(MachineCodeMissingRequired, "external", "rfx.erp.missing_external"))
		} else {
			issues = appendIssues(issues, validateExternal(doc.External)...)
		}
	}
	issues = appendIssues(issues, validateEvent(ctx, tenantID, &doc.Event, maps)...)
	freight := parseFreightExtensions(doc.Extensions)
	unitIssue, _, unitWarn := resolveMappedField(ctx, tenantID, maps, "UNIT", "extensions.freight.unit_code", freight.UnitCode)
	issues, warnings = appendMappingResults(issues, warnings, unitIssue, nil, unitWarn)
	cargoIssue, _, cargoWarn := resolveMappedField(ctx, tenantID, maps, "CARGO_TYPE", "extensions.freight.cargo_type", freight.CargoType)
	issues, warnings = appendMappingResults(issues, warnings, cargoIssue, nil, cargoWarn)
	if len(doc.Lots) > MaxLots {
		issues = append(issues, errIssue(MachineCodeTooManyLots, "lots", "rfx.erp.too_many_lots"))
	}
	for i, lot := range doc.Lots {
		issues = appendIssues(issues, validateLot(ctx, tenantID, lot, i, maps)...)
	}
	if len(doc.Questionnaire) > 0 {
		qIssues, qWarns := validateQuestionnaireJSON(doc.Questionnaire)
		issues = appendIssues(issues, qIssues...)
		warnings = append(warnings, qWarns...)
	}
	if len(doc.MappingContext) > 0 {
		issues = append(issues, errIssue(MachineCodeUnknownField, "mapping_context", "rfx.erp.unknown_field"))
	}

	out.Errors = issues
	out.Warnings = warnings
	out.External = doc.External
	out.Event = doc.Event
	out.ReadyToCommit = len(issues) == 0

	if !out.ReadyToCommit {
		return out, nil
	}

	canonical, pin, err := buildCanonicalPayload(doc, maps, ctx, tenantID)
	if err != nil {
		out.ReadyToCommit = false
		out.Errors = append(out.Errors, errIssue(MachineCodeMappingNotFound, "", err.Error()))
		return out, nil
	}
	hash, err := StableHash(canonical)
	if err != nil {
		return nil, err
	}
	out.CanonicalJSON = canonical
	out.CanonicalHash = hash
	out.MappingContext = pin
	return out, nil
}

func appendMappingResults(errors, warnings []Issue, issue Issue, _ *domain.ReferenceMappingSet, warnOnly bool) ([]Issue, []Issue) {
	if issue.MachineCode == "" {
		return errors, warnings
	}
	if warnOnly {
		issue.Severity = SeverityWarning
		return errors, append(warnings, issue)
	}
	return append(errors, issue), warnings
}

func appendIssues(base []Issue, extra ...Issue) []Issue {
	return append(base, extra...)
}

func validateExternal(ext *ExternalRef) []Issue {
	var issues []Issue
	sys := strings.ToUpper(strings.TrimSpace(ext.System))
	if sys == "" || len(sys) > 64 {
		issues = append(issues, errIssue(MachineCodeInvalidType, "external.system", "rfx.erp.invalid_external_system"))
	}
	id := norm.NFC.String(strings.TrimSpace(ext.ObjectID))
	if id == "" || utf8.RuneCountInString(id) > 256 {
		issues = append(issues, errIssue(MachineCodeInvalidType, "external.object_id", "rfx.erp.invalid_external_object_id"))
	}
	ext.System = sys
	ext.ObjectID = id
	return issues
}

func validateEvent(ctx context.Context, tenantID uuid.UUID, ev *EventPayload, maps MappingLookup) []Issue {
	var issues []Issue
	if strings.TrimSpace(ev.Title) == "" || utf8.RuneCountInString(ev.Title) > MaxStringTitle {
		issues = append(issues, errIssue(MachineCodeMissingRequired, "event.title", "rfx.erp.invalid_title"))
	}
	if strings.TrimSpace(ev.Type) == "" {
		issues = append(issues, errIssue(MachineCodeMissingRequired, "event.type", "rfx.erp.invalid_event_type"))
	}
	if maps != nil {
		if issue, _, _ := resolveMappedField(ctx, tenantID, maps, "CURRENCY", "event.currency", ev.Currency); issue.MachineCode != "" {
			issues = append(issues, issue)
		}
		if issue, _, _ := resolveMappedField(ctx, tenantID, maps, "TIMEZONE", "event.timezone", ev.Timezone); issue.MachineCode != "" {
			issues = append(issues, issue)
		}
	}
	return issues
}

func validateLot(ctx context.Context, tenantID uuid.UUID, lot LotPayload, index int, maps MappingLookup) []Issue {
	path := fmt.Sprintf("lots[%d]", index)
	var issues []Issue
	if strings.TrimSpace(lot.LotNumber) == "" {
		issues = append(issues, errIssue(MachineCodeMissingRequired, path+".lot_number", "rfx.erp.invalid_lot"))
	}
	if strings.TrimSpace(lot.Name) == "" {
		issues = append(issues, errIssue(MachineCodeMissingRequired, path+".name", "rfx.erp.invalid_lot"))
	}
	if maps != nil && strings.TrimSpace(lot.CurrencyCode) != "" {
		if issue, _, _ := resolveMappedField(ctx, tenantID, maps, "CURRENCY", path+".currency_code", lot.CurrencyCode); issue.MachineCode != "" {
			issues = append(issues, issue)
		}
	}
	return issues
}

type questionnaireJSON struct {
	Sections []struct {
		SectionCode string `json:"section_code"`
		Title       string `json:"title"`
	} `json:"sections"`
	Rules []questionnaireRuleJSON `json:"rules"`
}

type questionnaireRuleJSON struct {
	RuleCode           string          `json:"rule_code"`
	SourceQuestionCode   string          `json:"source_question_code"`
	TargetQuestionCode   string          `json:"target_question_code"`
	Action               string          `json:"action"`
	Condition            json.RawMessage `json:"condition"`
}

func validateQuestionnaireJSON(raw json.RawMessage) (errors, warnings []Issue) {
	var q questionnaireJSON
	if err := json.Unmarshal(raw, &q); err != nil {
		return []Issue{errIssue(MachineCodeInvalidType, "questionnaire", "rfx.erp.invalid_questionnaire")}, nil
	}
	sectionCodes := make(map[string]struct{})
	questionCodes := make(map[string]struct{})
	questionCodeByID := make(map[uuid.UUID]string)
	questionTypeByCode := make(map[string]string)

	for _, sec := range q.Sections {
		code := strings.TrimSpace(sec.SectionCode)
		if _, dup := sectionCodes[code]; dup {
			errors = append(errors, errIssue(MachineCodeDuplicateSection, "questionnaire.sections", "rfx.erp.duplicate_section_code"))
		}
		sectionCodes[code] = struct{}{}
	}

	codeToID := make(map[string]uuid.UUID)
	for _, rule := range q.Rules {
		if rule.SourceQuestionCode != "" {
			questionCodes[rule.SourceQuestionCode] = struct{}{}
		}
		if rule.TargetQuestionCode != "" {
			questionCodes[rule.TargetQuestionCode] = struct{}{}
		}
	}
	for code := range questionCodes {
		id := uuid.New()
		codeToID[code] = id
		questionCodeByID[id] = code
		questionTypeByCode[code] = domain.QuestionTypeText
	}

	domainRules := make([]domain.QuestionRule, 0, len(q.Rules))
	for _, rule := range q.Rules {
		targetID := codeToID[rule.TargetQuestionCode]
		cond := rule.Condition
		if len(cond) == 0 && rule.SourceQuestionCode != "" {
			cond = json.RawMessage(fmt.Sprintf(`{"operator":"IS_NOT_EMPTY","source_question_code":%q}`, rule.SourceQuestionCode))
		}
		domainRules = append(domainRules, domain.QuestionRule{
			RuleCode:         rule.RuleCode,
			Action:           rule.Action,
			TargetQuestionID: &targetID,
			ConditionJSON:    cond,
		})
	}
	if err := domain.ValidateRuleSet(domainRules, questionCodes, questionCodeByID, questionTypeByCode); err != nil {
		if strings.Contains(strings.ToLower(err.Error()), "cycle") || strings.Contains(err.Error(), "references itself") {
			errors = append(errors, errIssue(MachineCodeCyclicRule, "questionnaire.rules", "rfx.erp.cyclic_rule"))
		}
	}
	return errors, warnings
}

func buildCanonicalPayload(doc RawDocument, maps MappingLookup, ctx context.Context, tenantID uuid.UUID) ([]byte, MappingContextPin, error) {
	event := doc.Event
	pin := MappingContextPin{}
	typesApplied := []string{}

	if maps != nil && strings.TrimSpace(event.Currency) != "" {
		canonical, set, _, err := maps.ResolveCode(ctx, tenantID, "CURRENCY", event.Currency)
		if err != nil {
			return nil, pin, err
		}
		event.Currency = canonical
		typesApplied = appendUnique(typesApplied, "CURRENCY")
		pin = mergePin(pin, set)
	}
	if maps != nil && strings.TrimSpace(event.Timezone) != "" {
		canonical, set, _, err := maps.ResolveCode(ctx, tenantID, "TIMEZONE", event.Timezone)
		if err != nil {
			return nil, pin, err
		}
		event.Timezone = canonical
		typesApplied = appendUnique(typesApplied, "TIMEZONE")
		pin = mergePin(pin, set)
	}

	lots := make([]LotPayload, len(doc.Lots))
	copy(lots, doc.Lots)
	for i := range lots {
		if maps != nil && strings.TrimSpace(lots[i].CurrencyCode) != "" {
			canonical, set, _, err := maps.ResolveCode(ctx, tenantID, "CURRENCY", lots[i].CurrencyCode)
			if err != nil {
				return nil, pin, err
			}
			lots[i].CurrencyCode = canonical
			typesApplied = appendUnique(typesApplied, "CURRENCY")
			pin = mergePin(pin, set)
		}
	}

	extensions := doc.Extensions
	freight := parseFreightExtensions(doc.Extensions)
	if maps != nil {
		if strings.TrimSpace(freight.UnitCode) != "" {
			canonical, set, _, err := maps.ResolveCode(ctx, tenantID, "UNIT", freight.UnitCode)
			if err != nil {
				return nil, pin, err
			}
			freight.UnitCode = canonical
			typesApplied = appendUnique(typesApplied, "UNIT")
			pin = mergePin(pin, set)
			extensions = mergeFreightExtensions(extensions, freight)
		}
		if strings.TrimSpace(freight.CargoType) != "" {
			if canonical, set, _, err := maps.ResolveCode(ctx, tenantID, "CARGO_TYPE", freight.CargoType); err == nil {
				freight.CargoType = canonical
				typesApplied = appendUnique(typesApplied, "CARGO_TYPE")
				pin = mergePin(pin, set)
				extensions = mergeFreightExtensions(extensions, freight)
			}
		}
	}

	resolved := map[string]any{
		"schema_version":      doc.SchemaVersion,
		"requested_operation": normalizeOperation(doc.RequestedOperation),
		"event":               event,
	}
	if doc.External != nil {
		resolved["external"] = doc.External
	}
	if len(lots) > 0 {
		resolved["lots"] = lots
	}
	if len(doc.Questionnaire) > 0 {
		resolved["questionnaire"] = json.RawMessage(doc.Questionnaire)
	}
	if len(extensions) > 0 {
		resolved["extensions"] = json.RawMessage(extensions)
	}
	pin.MappingTypes = typesApplied
	resolved["mapping_context"] = pin
	normalized, err := json.Marshal(resolved)
	if err != nil {
		return nil, pin, err
	}
	out, err := NormalizeJSON(normalized)
	return out, pin, err
}

func mergeFreightExtensions(raw json.RawMessage, freight freightExtensions) json.RawMessage {
	root := map[string]any{}
	if len(raw) > 0 {
		_ = json.Unmarshal(raw, &root)
	}
	freightObj := map[string]any{}
	if existing, ok := root["freight"].(map[string]any); ok {
		freightObj = existing
	}
	if strings.TrimSpace(freight.UnitCode) != "" {
		freightObj["unit_code"] = freight.UnitCode
	}
	if strings.TrimSpace(freight.CargoType) != "" {
		freightObj["cargo_type"] = freight.CargoType
	}
	root["freight"] = freightObj
	out, _ := json.Marshal(root)
	return out
}

func mergePin(current MappingContextPin, set domain.ReferenceMappingSet) MappingContextPin {
	if current.MappingSetID == uuid.Nil {
		return MappingContextPin{MappingSetID: set.ID, MappingSetVersion: set.Version}
	}
	if current.MappingSetID != set.ID {
		return current
	}
	return current
}

func appendUnique(items []string, v string) []string {
	for _, existing := range items {
		if existing == v {
			return items
		}
	}
	return append(items, v)
}

func HasErrors(issues []Issue) bool {
	for _, i := range issues {
		if i.Severity == SeverityError {
			return true
		}
	}
	return false
}
