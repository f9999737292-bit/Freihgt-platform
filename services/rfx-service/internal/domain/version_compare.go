package domain

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"github.com/google/uuid"

	apperrors "github.com/freight-platform/rfx-service/internal/platform/errors"
)

const (
	VersionDiffAdded     = "ADDED"
	VersionDiffRemoved   = "REMOVED"
	VersionDiffChanged   = "CHANGED"
	VersionDiffReordered = "REORDERED"
	VersionDiffUnchanged = "UNCHANGED"
)

type CompareVersionsInput struct {
	SourceVersionID uuid.UUID `json:"source_version_id"`
	TargetVersionID uuid.UUID `json:"target_version_id"`
}

type VersionCompareMetadata struct {
	ID            uuid.UUID `json:"id"`
	VersionNumber int       `json:"version_number"`
	Status        string    `json:"status"`
}

type CompareSummary struct {
	AddedCount     int `json:"added_count"`
	RemovedCount   int `json:"removed_count"`
	ChangedCount   int `json:"changed_count"`
	ReorderedCount int `json:"reordered_count"`
	UnchangedCount int `json:"unchanged_count"`
}

type CompareFieldDiff struct {
	Field  string `json:"field"`
	Before any    `json:"before,omitempty"`
	After  any    `json:"after,omitempty"`
}

type CompareItemDiff struct {
	EntityType    string             `json:"entity_type"`
	Change        string             `json:"change"`
	SectionCode   string             `json:"section_code,omitempty"`
	QuestionCode  string             `json:"question_code,omitempty"`
	OptionCode    string             `json:"option_code,omitempty"`
	RuleCode      string             `json:"rule_code,omitempty"`
	CriterionCode string             `json:"criterion_code,omitempty"`
	Fields        []string           `json:"fields,omitempty"`
	FieldDiffs    []CompareFieldDiff `json:"field_diffs,omitempty"`
}

type CompareScoringDiff struct {
	Criteria []CompareItemDiff `json:"criteria"`
	Bindings []CompareItemDiff `json:"bindings"`
	Model    *CompareItemDiff  `json:"model,omitempty"`
}

type CompareVersionsResult struct {
	SourceVersion       VersionCompareMetadata `json:"source_version"`
	TargetVersion       VersionCompareMetadata `json:"target_version"`
	SourceVersionNumber int                    `json:"source_version_number"`
	TargetVersionNumber int                    `json:"target_version_number"`
	Summary             CompareSummary         `json:"summary"`
	CanonicalDiffHash   string                 `json:"canonical_diff_hash"`
	Differences         []CompareItemDiff      `json:"differences"`
	Sections            []CompareItemDiff      `json:"sections"`
	Questions           []CompareItemDiff      `json:"questions"`
	Options             []CompareItemDiff      `json:"options"`
	Rules               []CompareItemDiff      `json:"rules"`
	Scoring             CompareScoringDiff     `json:"scoring"`
}

type ScoringCompareSnapshot struct {
	ModelVersion int
	Status       string
	Criteria     []ScoreCriterion
	Bindings     []ScoreBindingCompareEntry
}

type ScoreBindingCompareEntry struct {
	CriterionCode    string
	QuestionCode     string
	BindingType      string
	ScoringRuleJSON  json.RawMessage
	KnockoutRuleJSON json.RawMessage
}

type VersionCompareSnapshot struct {
	Version       RfxVersion
	Questionnaire QuestionnaireDefinition
	Scoring       *ScoringCompareSnapshot
}

func ValidateCompareVersionsInput(in CompareVersionsInput) error {
	if in.SourceVersionID == uuid.Nil {
		return apperrors.Validation("source_version_id is required", map[string]any{"field": "source_version_id"})
	}
	if in.TargetVersionID == uuid.Nil {
		return apperrors.Validation("target_version_id is required", map[string]any{"field": "target_version_id"})
	}
	if in.SourceVersionID == in.TargetVersionID {
		return apperrors.Validation("source_version_id and target_version_id must differ", map[string]any{"field": "target_version_id"})
	}
	return nil
}

func CompareVersionSnapshots(source, target VersionCompareSnapshot) CompareVersionsResult {
	questionCodeByIDSource := questionCodeIndex(source.Questionnaire)
	questionCodeByIDTarget := questionCodeIndex(target.Questionnaire)

	var sections, questions, options, rules []CompareItemDiff
	sections = compareSections(source.Questionnaire, target.Questionnaire)
	questions = compareQuestions(source.Questionnaire, target.Questionnaire)
	options = compareOptions(source.Questionnaire, target.Questionnaire)
	rules = compareRules(source.Questionnaire, target.Questionnaire, questionCodeByIDSource, questionCodeByIDTarget)

	scoring := CompareScoringDiff{}
	if source.Scoring != nil || target.Scoring != nil {
		scoring = compareScoring(source.Scoring, target.Scoring)
	}

	enabledDiff := compareQuestionnaireEnabled(source.Version, target.Version)
	if enabledDiff != nil {
		sections = append(sections, *enabledDiff)
	}

	all := make([]CompareItemDiff, 0, len(sections)+len(questions)+len(options)+len(rules)+len(scoring.Criteria)+len(scoring.Bindings)+1)
	all = append(all, sections...)
	all = append(all, questions...)
	all = append(all, options...)
	all = append(all, rules...)
	all = append(all, scoring.Criteria...)
	all = append(all, scoring.Bindings...)
	if scoring.Model != nil {
		all = append(all, *scoring.Model)
	}
	sortCompareItems(all)

	summary := summarizeCompareItems(all)
	hash := canonicalDiffHash(all)

	return CompareVersionsResult{
		SourceVersion: VersionCompareMetadata{
			ID:            source.Version.ID,
			VersionNumber: source.Version.VersionNumber,
			Status:        source.Version.Status,
		},
		TargetVersion: VersionCompareMetadata{
			ID:            target.Version.ID,
			VersionNumber: target.Version.VersionNumber,
			Status:        target.Version.Status,
		},
		SourceVersionNumber: source.Version.VersionNumber,
		TargetVersionNumber: target.Version.VersionNumber,
		Summary:             summary,
		CanonicalDiffHash:   hash,
		Differences:         all,
		Sections:            filterByEntityType(all, "section"),
		Questions:           filterByEntityType(all, "question"),
		Options:             filterByEntityType(all, "option"),
		Rules:               filterByEntityType(all, "rule"),
		Scoring:             scoring,
	}
}

func compareQuestionnaireEnabled(source, target RfxVersion) *CompareItemDiff {
	if source.QuestionnaireEnabled == target.QuestionnaireEnabled {
		item := CompareItemDiff{
			EntityType: "questionnaire_enabled",
			Change:     VersionDiffUnchanged,
			Fields:     []string{"questionnaire_enabled"},
			FieldDiffs: []CompareFieldDiff{{
				Field:  "questionnaire_enabled",
				Before: source.QuestionnaireEnabled,
				After:  target.QuestionnaireEnabled,
			}},
		}
		return &item
	}
	return &CompareItemDiff{
		EntityType: "questionnaire_enabled",
		Change:     VersionDiffChanged,
		Fields:     []string{"questionnaire_enabled"},
		FieldDiffs: []CompareFieldDiff{{
			Field:  "questionnaire_enabled",
			Before: source.QuestionnaireEnabled,
			After:  target.QuestionnaireEnabled,
		}},
	}
}

func compareSections(source, target QuestionnaireDefinition) []CompareItemDiff {
	sourceMap := mapSections(source)
	targetMap := mapSections(target)
	codes := sortedStringKeys(sourceMap, targetMap)
	out := make([]CompareItemDiff, 0, len(codes))
	for _, code := range codes {
		left, leftOK := sourceMap[code]
		right, rightOK := targetMap[code]
		switch {
		case leftOK && !rightOK:
			out = append(out, CompareItemDiff{
				EntityType:  "section",
				Change:      VersionDiffRemoved,
				SectionCode: code,
				FieldDiffs:  sectionFieldDiffs(&left, nil),
			})
		case !leftOK && rightOK:
			out = append(out, CompareItemDiff{
				EntityType:  "section",
				Change:      VersionDiffAdded,
				SectionCode: code,
				FieldDiffs:  sectionFieldDiffs(nil, &right),
			})
		default:
			item := diffSection(code, left, right)
			out = append(out, item)
		}
	}
	return out
}

func diffSection(code string, left, right sectionSnapshot) CompareItemDiff {
	fields, diffs := diffScalars(
		[]fieldPair{
			{"title", left.title, right.title},
			{"description", left.description, right.description},
			{"sort_order", left.sortOrder, right.sortOrder},
		},
	)
	change := classifyChange(fields, left.sortOrder != right.sortOrder)
	return CompareItemDiff{
		EntityType:  "section",
		Change:      change,
		SectionCode: code,
		Fields:      fields,
		FieldDiffs:  diffs,
	}
}

func compareQuestions(source, target QuestionnaireDefinition) []CompareItemDiff {
	sourceMap := mapQuestions(source)
	targetMap := mapQuestions(target)
	keys := sortedStringKeys(sourceMap, targetMap)
	out := make([]CompareItemDiff, 0, len(keys))
	for _, key := range keys {
		left, leftOK := sourceMap[key]
		right, rightOK := targetMap[key]
		sectionCode, questionCode := splitQuestionKey(key)
		switch {
		case leftOK && !rightOK:
			out = append(out, CompareItemDiff{
				EntityType:   "question",
				Change:       VersionDiffRemoved,
				SectionCode:  sectionCode,
				QuestionCode: questionCode,
				FieldDiffs:   questionFieldDiffs(&left, nil),
			})
		case !leftOK && rightOK:
			out = append(out, CompareItemDiff{
				EntityType:   "question",
				Change:       VersionDiffAdded,
				SectionCode:  sectionCode,
				QuestionCode: questionCode,
				FieldDiffs:   questionFieldDiffs(nil, &right),
			})
		default:
			item := diffQuestion(sectionCode, questionCode, left, right)
			out = append(out, item)
		}
	}
	return out
}

func diffQuestion(sectionCode, questionCode string, left, right questionSnapshot) CompareItemDiff {
	fields, diffs := diffScalars(
		[]fieldPair{
			{"question_type", left.questionType, right.questionType},
			{"label", left.label, right.label},
			{"help_text", left.helpText, right.helpText},
			{"required", left.required, right.required},
			{"validation_rule_json", normalizeJSON(left.validationJSON), normalizeJSON(right.validationJSON)},
			{"sort_order", left.sortOrder, right.sortOrder},
		},
	)
	change := classifyChange(fields, left.sortOrder != right.sortOrder)
	return CompareItemDiff{
		EntityType:   "question",
		Change:       change,
		SectionCode:  sectionCode,
		QuestionCode: questionCode,
		Fields:       fields,
		FieldDiffs:   diffs,
	}
}

func compareOptions(source, target QuestionnaireDefinition) []CompareItemDiff {
	sourceMap := mapOptions(source)
	targetMap := mapOptions(target)
	keys := sortedStringKeys(sourceMap, targetMap)
	out := make([]CompareItemDiff, 0, len(keys))
	for _, key := range keys {
		left, leftOK := sourceMap[key]
		right, rightOK := targetMap[key]
		sectionCode, questionCode, optionCode := splitOptionKey(key)
		switch {
		case leftOK && !rightOK:
			out = append(out, CompareItemDiff{
				EntityType:   "option",
				Change:       VersionDiffRemoved,
				SectionCode:  sectionCode,
				QuestionCode: questionCode,
				OptionCode:   optionCode,
				FieldDiffs:   optionFieldDiffs(&left, nil),
			})
		case !leftOK && rightOK:
			out = append(out, CompareItemDiff{
				EntityType:   "option",
				Change:       VersionDiffAdded,
				SectionCode:  sectionCode,
				QuestionCode: questionCode,
				OptionCode:   optionCode,
				FieldDiffs:   optionFieldDiffs(nil, &right),
			})
		default:
			item := diffOption(sectionCode, questionCode, optionCode, left, right)
			out = append(out, item)
		}
	}
	return out
}

func diffOption(sectionCode, questionCode, optionCode string, left, right optionSnapshot) CompareItemDiff {
	fields, diffs := diffScalars(
		[]fieldPair{
			{"label", left.label, right.label},
			{"sort_order", left.sortOrder, right.sortOrder},
		},
	)
	change := classifyChange(fields, left.sortOrder != right.sortOrder)
	return CompareItemDiff{
		EntityType:   "option",
		Change:       change,
		SectionCode:  sectionCode,
		QuestionCode: questionCode,
		OptionCode:   optionCode,
		Fields:       fields,
		FieldDiffs:   diffs,
	}
}

func compareRules(source, target QuestionnaireDefinition, sourceQuestionCodes, targetQuestionCodes map[uuid.UUID]string) []CompareItemDiff {
	sourceMap := mapRules(source, sourceQuestionCodes)
	targetMap := mapRules(target, targetQuestionCodes)
	codes := sortedStringKeys(sourceMap, targetMap)
	out := make([]CompareItemDiff, 0, len(codes))
	for _, code := range codes {
		left, leftOK := sourceMap[code]
		right, rightOK := targetMap[code]
		switch {
		case leftOK && !rightOK:
			out = append(out, CompareItemDiff{
				EntityType: "rule",
				Change:     VersionDiffRemoved,
				RuleCode:   code,
				FieldDiffs: ruleFieldDiffs(&left, nil),
			})
		case !leftOK && rightOK:
			out = append(out, CompareItemDiff{
				EntityType: "rule",
				Change:     VersionDiffAdded,
				RuleCode:   code,
				FieldDiffs: ruleFieldDiffs(nil, &right),
			})
		default:
			item := diffRule(code, left, right)
			out = append(out, item)
		}
	}
	return out
}

func diffRule(code string, left, right ruleSnapshot) CompareItemDiff {
	fields, diffs := diffScalars(
		[]fieldPair{
			{"action", left.action, right.action},
			{"condition_json", normalizeJSON(left.conditionJSON), normalizeJSON(right.conditionJSON)},
			{"target_question_code", left.targetQuestionCode, right.targetQuestionCode},
			{"sort_order", left.sortOrder, right.sortOrder},
		},
	)
	change := classifyChange(fields, left.sortOrder != right.sortOrder)
	return CompareItemDiff{
		EntityType: "rule",
		Change:     change,
		RuleCode:   code,
		Fields:     fields,
		FieldDiffs: diffs,
	}
}

func compareScoring(source, target *ScoringCompareSnapshot) CompareScoringDiff {
	var leftModelVersion int
	var leftStatus string
	if source != nil {
		leftModelVersion = source.ModelVersion
		leftStatus = source.Status
	}
	var rightModelVersion int
	var rightStatus string
	if target != nil {
		rightModelVersion = target.ModelVersion
		rightStatus = target.Status
	}

	out := CompareScoringDiff{}
	if leftModelVersion != rightModelVersion || leftStatus != rightStatus {
		fields, diffs := diffScalars([]fieldPair{
			{"model_version", leftModelVersion, rightModelVersion},
			{"status", leftStatus, rightStatus},
		})
		out.Model = &CompareItemDiff{
			EntityType: "score_model",
			Change:     VersionDiffChanged,
			Fields:     fields,
			FieldDiffs: diffs,
		}
	} else if source != nil || target != nil {
		out.Model = &CompareItemDiff{
			EntityType: "score_model",
			Change:     VersionDiffUnchanged,
			Fields:     []string{"model_version", "status"},
			FieldDiffs: []CompareFieldDiff{
				{Field: "model_version", Before: leftModelVersion, After: rightModelVersion},
				{Field: "status", Before: leftStatus, After: rightStatus},
			},
		}
	}

	sourceCriteria := mapCriteria(source)
	targetCriteria := mapCriteria(target)
	for _, code := range sortedStringKeys(sourceCriteria, targetCriteria) {
		left, leftOK := sourceCriteria[code]
		right, rightOK := targetCriteria[code]
		switch {
		case leftOK && !rightOK:
			out.Criteria = append(out.Criteria, CompareItemDiff{
				EntityType:    "score_criterion",
				Change:        VersionDiffRemoved,
				CriterionCode: code,
				FieldDiffs:    criterionFieldDiffs(&left, nil),
			})
		case !leftOK && rightOK:
			out.Criteria = append(out.Criteria, CompareItemDiff{
				EntityType:    "score_criterion",
				Change:        VersionDiffAdded,
				CriterionCode: code,
				FieldDiffs:    criterionFieldDiffs(nil, &right),
			})
		default:
			out.Criteria = append(out.Criteria, diffCriterion(code, left, right))
		}
	}
	sortCompareItems(out.Criteria)

	sourceBindings := mapBindings(source)
	targetBindings := mapBindings(target)
	for _, key := range sortedStringKeys(sourceBindings, targetBindings) {
		left, leftOK := sourceBindings[key]
		right, rightOK := targetBindings[key]
		criterionCode, questionCode := splitBindingKey(key)
		switch {
		case leftOK && !rightOK:
			out.Bindings = append(out.Bindings, CompareItemDiff{
				EntityType:    "score_binding",
				Change:        VersionDiffRemoved,
				CriterionCode: criterionCode,
				QuestionCode:  questionCode,
				FieldDiffs:    bindingFieldDiffs(&left, nil),
			})
		case !leftOK && rightOK:
			out.Bindings = append(out.Bindings, CompareItemDiff{
				EntityType:    "score_binding",
				Change:        VersionDiffAdded,
				CriterionCode: criterionCode,
				QuestionCode:  questionCode,
				FieldDiffs:    bindingFieldDiffs(nil, &right),
			})
		default:
			out.Bindings = append(out.Bindings, diffBinding(criterionCode, questionCode, left, right))
		}
	}
	sortCompareItems(out.Bindings)
	return out
}

func diffCriterion(code string, left, right criterionSnapshot) CompareItemDiff {
	fields, diffs := diffScalars([]fieldPair{
		{"weight", left.weight, right.weight},
		{"normalization_json", normalizeJSON(left.normalizationJSON), normalizeJSON(right.normalizationJSON)},
	})
	change := classifyChange(fields, false)
	return CompareItemDiff{
		EntityType:    "score_criterion",
		Change:        change,
		CriterionCode: code,
		Fields:        fields,
		FieldDiffs:    diffs,
	}
}

func diffBinding(criterionCode, questionCode string, left, right bindingSnapshot) CompareItemDiff {
	fields, diffs := diffScalars([]fieldPair{
		{"knockout_rule_json", normalizeJSON(left.knockoutJSON), normalizeJSON(right.knockoutJSON)},
		{"scoring_rule_json", normalizeJSON(left.scoringJSON), normalizeJSON(right.scoringJSON)},
		{"binding_type", left.bindingType, right.bindingType},
	})
	change := classifyChange(fields, false)
	return CompareItemDiff{
		EntityType:    "score_binding",
		Change:        change,
		CriterionCode: criterionCode,
		QuestionCode:  questionCode,
		Fields:        fields,
		FieldDiffs:    diffs,
	}
}

type sectionSnapshot struct {
	title       string
	description *string
	sortOrder   int
}

type questionSnapshot struct {
	questionType   string
	label          string
	helpText       *string
	required       bool
	validationJSON json.RawMessage
	sortOrder      int
}

type optionSnapshot struct {
	label     string
	sortOrder int
}

type ruleSnapshot struct {
	action             string
	conditionJSON      json.RawMessage
	targetQuestionCode string
	sortOrder          int
}

type criterionSnapshot struct {
	weight            float64
	normalizationJSON json.RawMessage
}

type bindingSnapshot struct {
	knockoutJSON json.RawMessage
	scoringJSON  json.RawMessage
	bindingType  string
}

type fieldPair struct {
	name   string
	before any
	after  any
}

func mapSections(def QuestionnaireDefinition) map[string]sectionSnapshot {
	out := make(map[string]sectionSnapshot, len(def.Sections))
	for _, item := range def.Sections {
		out[item.Section.SectionCode] = sectionSnapshot{
			title:       item.Section.Title,
			description: item.Section.Description,
			sortOrder:   item.Section.SortOrder,
		}
	}
	return out
}

func mapQuestions(def QuestionnaireDefinition) map[string]questionSnapshot {
	out := make(map[string]questionSnapshot)
	for _, section := range def.Sections {
		for _, question := range section.Questions {
			key := questionKey(section.Section.SectionCode, question.QuestionCode)
			out[key] = questionSnapshot{
				questionType:   question.QuestionType,
				label:          question.Label,
				helpText:       question.HelpText,
				required:       question.Required,
				validationJSON: question.ValidationRuleJSON,
				sortOrder:      question.SortOrder,
			}
		}
	}
	return out
}

func mapOptions(def QuestionnaireDefinition) map[string]optionSnapshot {
	out := make(map[string]optionSnapshot)
	for _, section := range def.Sections {
		for _, question := range section.Questions {
			for _, option := range question.Options {
				key := optionKey(section.Section.SectionCode, question.QuestionCode, option.OptionCode)
				out[key] = optionSnapshot{
					label:     option.Label,
					sortOrder: option.SortOrder,
				}
			}
		}
	}
	return out
}

func mapRules(def QuestionnaireDefinition, questionCodes map[uuid.UUID]string) map[string]ruleSnapshot {
	out := make(map[string]ruleSnapshot, len(def.Rules))
	for _, rule := range def.Rules {
		targetCode := ""
		if rule.TargetQuestionID != nil {
			targetCode = questionCodes[*rule.TargetQuestionID]
		}
		out[rule.RuleCode] = ruleSnapshot{
			action:             rule.Action,
			conditionJSON:      rule.ConditionJSON,
			targetQuestionCode: targetCode,
			sortOrder:          rule.SortOrder,
		}
	}
	return out
}

func mapCriteria(snapshot *ScoringCompareSnapshot) map[string]criterionSnapshot {
	out := make(map[string]criterionSnapshot)
	if snapshot == nil {
		return out
	}
	for _, item := range snapshot.Criteria {
		out[item.CriterionCode] = criterionSnapshot{
			weight:            item.Weight,
			normalizationJSON: item.NormalizationJSON,
		}
	}
	return out
}

func mapBindings(snapshot *ScoringCompareSnapshot) map[string]bindingSnapshot {
	out := make(map[string]bindingSnapshot)
	if snapshot == nil {
		return out
	}
	for _, item := range snapshot.Bindings {
		key := bindingKey(item.CriterionCode, item.QuestionCode)
		out[key] = bindingSnapshot{
			knockoutJSON: item.KnockoutRuleJSON,
			scoringJSON:  item.ScoringRuleJSON,
			bindingType:  item.BindingType,
		}
	}
	return out
}

func questionCodeIndex(def QuestionnaireDefinition) map[uuid.UUID]string {
	out := make(map[uuid.UUID]string)
	for _, section := range def.Sections {
		for _, question := range section.Questions {
			out[question.ID] = question.QuestionCode
		}
	}
	return out
}

func questionKey(sectionCode, questionCode string) string {
	return sectionCode + "\x00" + questionCode
}

func optionKey(sectionCode, questionCode, optionCode string) string {
	return sectionCode + "\x00" + questionCode + "\x00" + optionCode
}

func bindingKey(criterionCode, questionCode string) string {
	return criterionCode + "\x00" + questionCode
}

func splitQuestionKey(key string) (string, string) {
	parts := strings.SplitN(key, "\x00", 2)
	if len(parts) != 2 {
		return key, ""
	}
	return parts[0], parts[1]
}

func splitOptionKey(key string) (string, string, string) {
	parts := strings.SplitN(key, "\x00", 3)
	if len(parts) != 3 {
		return key, "", ""
	}
	return parts[0], parts[1], parts[2]
}

func splitBindingKey(key string) (string, string) {
	return splitQuestionKey(key)
}

func sortedStringKeys[T any](source, target map[string]T) []string {
	keys := make(map[string]struct{})
	for key := range source {
		keys[key] = struct{}{}
	}
	for key := range target {
		keys[key] = struct{}{}
	}
	out := make([]string, 0, len(keys))
	for key := range keys {
		out = append(out, key)
	}
	sort.Strings(out)
	return out
}

func diffScalars(pairs []fieldPair) ([]string, []CompareFieldDiff) {
	fields := make([]string, 0, len(pairs))
	diffs := make([]CompareFieldDiff, 0, len(pairs))
	for _, pair := range pairs {
		if normalizeCompareValue(pair.before) == normalizeCompareValue(pair.after) {
			continue
		}
		fields = append(fields, pair.name)
		diffs = append(diffs, CompareFieldDiff{
			Field:  pair.name,
			Before: pair.before,
			After:  pair.after,
		})
	}
	sort.Strings(fields)
	sort.Slice(diffs, func(i, j int) bool { return diffs[i].Field < diffs[j].Field })
	return fields, diffs
}

func classifyChange(changedFields []string, sortOrderChanged bool) string {
	if len(changedFields) == 0 {
		return VersionDiffUnchanged
	}
	nonOrderChanges := make([]string, 0, len(changedFields))
	for _, field := range changedFields {
		if field != "sort_order" {
			nonOrderChanges = append(nonOrderChanges, field)
		}
	}
	if len(nonOrderChanges) == 0 && sortOrderChanged {
		return VersionDiffReordered
	}
	return VersionDiffChanged
}

func normalizeCompareValue(value any) string {
	switch v := value.(type) {
	case nil:
		return ""
	case string:
		return v
	case bool:
		if v {
			return "true"
		}
		return "false"
	case int:
		return fmt.Sprintf("%d", v)
	case int64:
		return fmt.Sprintf("%d", v)
	case float64:
		return fmt.Sprintf("%.6f", v)
	default:
		payload, err := json.Marshal(v)
		if err != nil {
			return fmt.Sprintf("%v", v)
		}
		return string(payload)
	}
}

func normalizeJSON(raw json.RawMessage) string {
	if len(raw) == 0 {
		return ""
	}
	var decoded any
	if err := json.Unmarshal(raw, &decoded); err != nil {
		return string(raw)
	}
	payload, err := json.Marshal(decoded)
	if err != nil {
		return string(raw)
	}
	return string(payload)
}

func sectionFieldDiffs(left, right *sectionSnapshot) []CompareFieldDiff {
	var before, after sectionSnapshot
	if left != nil {
		before = *left
	}
	if right != nil {
		after = *right
	}
	_, diffs := diffScalars([]fieldPair{
		{"title", before.title, after.title},
		{"description", before.description, after.description},
		{"sort_order", before.sortOrder, after.sortOrder},
	})
	return diffs
}

func questionFieldDiffs(left, right *questionSnapshot) []CompareFieldDiff {
	var before, after questionSnapshot
	if left != nil {
		before = *left
	}
	if right != nil {
		after = *right
	}
	_, diffs := diffScalars([]fieldPair{
		{"question_type", before.questionType, after.questionType},
		{"label", before.label, after.label},
		{"help_text", before.helpText, after.helpText},
		{"required", before.required, after.required},
		{"validation_rule_json", normalizeJSON(before.validationJSON), normalizeJSON(after.validationJSON)},
		{"sort_order", before.sortOrder, after.sortOrder},
	})
	return diffs
}

func optionFieldDiffs(left, right *optionSnapshot) []CompareFieldDiff {
	var before, after optionSnapshot
	if left != nil {
		before = *left
	}
	if right != nil {
		after = *right
	}
	_, diffs := diffScalars([]fieldPair{
		{"label", before.label, after.label},
		{"sort_order", before.sortOrder, after.sortOrder},
	})
	return diffs
}

func ruleFieldDiffs(left, right *ruleSnapshot) []CompareFieldDiff {
	var before, after ruleSnapshot
	if left != nil {
		before = *left
	}
	if right != nil {
		after = *right
	}
	_, diffs := diffScalars([]fieldPair{
		{"action", before.action, after.action},
		{"condition_json", normalizeJSON(before.conditionJSON), normalizeJSON(after.conditionJSON)},
		{"target_question_code", before.targetQuestionCode, after.targetQuestionCode},
		{"sort_order", before.sortOrder, after.sortOrder},
	})
	return diffs
}

func criterionFieldDiffs(left, right *criterionSnapshot) []CompareFieldDiff {
	var before, after criterionSnapshot
	if left != nil {
		before = *left
	}
	if right != nil {
		after = *right
	}
	_, diffs := diffScalars([]fieldPair{
		{"weight", before.weight, after.weight},
		{"normalization_json", normalizeJSON(before.normalizationJSON), normalizeJSON(after.normalizationJSON)},
	})
	return diffs
}

func bindingFieldDiffs(left, right *bindingSnapshot) []CompareFieldDiff {
	var before, after bindingSnapshot
	if left != nil {
		before = *left
	}
	if right != nil {
		after = *right
	}
	_, diffs := diffScalars([]fieldPair{
		{"knockout_rule_json", normalizeJSON(before.knockoutJSON), normalizeJSON(after.knockoutJSON)},
		{"scoring_rule_json", normalizeJSON(before.scoringJSON), normalizeJSON(after.scoringJSON)},
		{"binding_type", before.bindingType, after.bindingType},
	})
	return diffs
}

func summarizeCompareItems(items []CompareItemDiff) CompareSummary {
	var summary CompareSummary
	for _, item := range items {
		switch item.Change {
		case VersionDiffAdded:
			summary.AddedCount++
		case VersionDiffRemoved:
			summary.RemovedCount++
		case VersionDiffChanged:
			summary.ChangedCount++
		case VersionDiffReordered:
			summary.ReorderedCount++
		case VersionDiffUnchanged:
			summary.UnchangedCount++
		}
	}
	return summary
}

func filterByEntityType(items []CompareItemDiff, entityType string) []CompareItemDiff {
	out := make([]CompareItemDiff, 0)
	for _, item := range items {
		if item.EntityType == entityType {
			out = append(out, item)
		}
	}
	return out
}

func sortCompareItems(items []CompareItemDiff) {
	sort.Slice(items, func(i, j int) bool {
		left := compareSortKey(items[i])
		right := compareSortKey(items[j])
		return left < right
	})
}

func compareSortKey(item CompareItemDiff) string {
	return strings.Join([]string{
		item.EntityType,
		item.SectionCode,
		item.QuestionCode,
		item.OptionCode,
		item.RuleCode,
		item.CriterionCode,
	}, "\x00")
}

type canonicalHashEntry struct {
	EntityType    string             `json:"entity_type"`
	Change        string             `json:"change"`
	SectionCode   string             `json:"section_code,omitempty"`
	QuestionCode  string             `json:"question_code,omitempty"`
	OptionCode    string             `json:"option_code,omitempty"`
	RuleCode      string             `json:"rule_code,omitempty"`
	CriterionCode string             `json:"criterion_code,omitempty"`
	Fields        []string           `json:"fields,omitempty"`
	FieldDiffs    []CompareFieldDiff `json:"field_diffs,omitempty"`
}

func canonicalDiffHash(items []CompareItemDiff) string {
	entries := make([]canonicalHashEntry, len(items))
	for i, item := range items {
		fields := append([]string(nil), item.Fields...)
		sort.Strings(fields)
		fieldDiffs := append([]CompareFieldDiff(nil), item.FieldDiffs...)
		sort.Slice(fieldDiffs, func(a, b int) bool { return fieldDiffs[a].Field < fieldDiffs[b].Field })
		entries[i] = canonicalHashEntry{
			EntityType:    item.EntityType,
			Change:        item.Change,
			SectionCode:   item.SectionCode,
			QuestionCode:  item.QuestionCode,
			OptionCode:    item.OptionCode,
			RuleCode:      item.RuleCode,
			CriterionCode: item.CriterionCode,
			Fields:        fields,
			FieldDiffs:    fieldDiffs,
		}
	}
	sort.Slice(entries, func(i, j int) bool {
		left, _ := json.Marshal(entries[i])
		right, _ := json.Marshal(entries[j])
		return bytes.Compare(left, right) < 0
	})
	payload, err := json.Marshal(entries)
	if err != nil {
		return ""
	}
	sum := sha256.Sum256(payload)
	return hex.EncodeToString(sum[:])
}
