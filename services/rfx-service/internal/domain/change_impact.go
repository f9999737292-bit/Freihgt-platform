package domain

import (
	"sort"
	"strings"
	"time"

	"github.com/google/uuid"

	apperrors "github.com/freight-platform/rfx-service/internal/platform/errors"
)

const (
	ChangeImpactClassNonMaterial                 = "NON_MATERIAL"
	ChangeImpactClassMaterialNoResponses         = "MATERIAL_NO_RESPONSES"
	ChangeImpactClassMaterialWithDraftResponses  = "MATERIAL_WITH_DRAFT_RESPONSES"
	ChangeImpactClassMaterialWithSubmitted       = "MATERIAL_WITH_SUBMITTED_RESPONSES"
	ChangeImpactClassScoringAffecting              = "SCORING_AFFECTING"
	ChangeImpactClassKnockoutAffecting             = "KNOCKOUT_AFFECTING"

	ChangeImpactPreviewTTL = 15 * time.Minute

	VersionLifecycleMachineCodeChangeImpactExpired     = "CHANGE_IMPACT_ANALYSIS_EXPIRED"
	VersionLifecycleMachineCodeChangeImpactConsumed    = "CHANGE_IMPACT_ANALYSIS_CONSUMED"
	VersionLifecycleMachineCodeChangeImpactStaleDiff   = "CHANGE_IMPACT_STALE_DIFF"
)

var changeImpactClassOrder = []string{
	ChangeImpactClassNonMaterial,
	ChangeImpactClassMaterialNoResponses,
	ChangeImpactClassMaterialWithDraftResponses,
	ChangeImpactClassMaterialWithSubmitted,
	ChangeImpactClassScoringAffecting,
	ChangeImpactClassKnockoutAffecting,
}

var allowedChangeImpactClasses = map[string]struct{}{
	ChangeImpactClassNonMaterial:                {},
	ChangeImpactClassMaterialNoResponses:        {},
	ChangeImpactClassMaterialWithDraftResponses: {},
	ChangeImpactClassMaterialWithSubmitted:      {},
	ChangeImpactClassScoringAffecting:           {},
	ChangeImpactClassKnockoutAffecting:          {},
}

type PreviewChangeImpactInput struct {
	CandidateVersionID uuid.UUID `json:"candidate_version_id"`
}

type ChangeImpactAnalysis struct {
	ID                            uuid.UUID
	TenantID                      uuid.UUID
	EventID                       uuid.UUID
	SourceVersionID               *uuid.UUID
	CandidateVersionID            uuid.UUID
	ActorID                       uuid.UUID
	CanonicalDiffHash             string
	ImpactClasses                 []string
	AffectedDraftResponseCount    int
	AffectedSubmittedResponseCount int
	ScoringAffecting              bool
	KnockoutAffecting             bool
	CreatedAt                     time.Time
	ExpiresAt                     time.Time
	ConsumedAt                    *time.Time
}

type ChangeImpactClassification struct {
	ImpactClasses                  []string
	AffectedDraftResponseCount     int
	AffectedSubmittedResponseCount int
	ScoringAffecting               bool
	KnockoutAffecting              bool
	AffectedQuestionCodes          []string
}

// ResponseImpactScope drives affected response counting for change-impact preview.
type ResponseImpactScope struct {
	CountAllResponsesOnVersion bool
	QuestionCodesWithAnswers   []string
}

func ValidatePreviewChangeImpactInput(in PreviewChangeImpactInput) error {
	if in.CandidateVersionID == uuid.Nil {
		return apperrors.Validation("candidate_version_id is required", map[string]any{"field": "candidate_version_id"})
	}
	return nil
}

func ValidateImpactClasses(classes []string) error {
	seen := make(map[string]struct{}, len(classes))
	for _, class := range classes {
		if _, ok := allowedChangeImpactClasses[class]; !ok {
			return apperrors.Internal("invalid impact class persisted: "+class, nil)
		}
		if _, dup := seen[class]; dup {
			return apperrors.Internal("duplicate impact class persisted: "+class, nil)
		}
		seen[class] = struct{}{}
	}
	return nil
}

func CanonicalizeImpactClasses(classes []string) []string {
	return SortImpactClasses(classes)
}

func SortImpactClasses(classes []string) []string {
	if len(classes) == 0 {
		return nil
	}
	present := make(map[string]struct{}, len(classes))
	for _, class := range classes {
		present[class] = struct{}{}
	}
	out := make([]string, 0, len(present))
	for _, class := range changeImpactClassOrder {
		if _, ok := present[class]; ok {
			out = append(out, class)
		}
	}
	return out
}

func ClassifyChangeImpact(diff CompareVersionsResult, affectedDraftCount, affectedSubmittedCount int) ChangeImpactClassification {
	hasKnockout := false
	hasScoring := false
	hasStructural := false
	hasNonMaterialOnly := true
	affectedCodes := extractAffectedQuestionCodes(diff)

	for _, item := range diff.Differences {
		switch item.EntityType {
		case "score_binding":
			if item.Change == VersionDiffUnchanged {
				continue
			}
			if bindingChangeAffectsKnockout(item) {
				hasKnockout = true
			}
			if bindingChangeAffectsScoring(item) {
				hasScoring = true
			}
		case "score_criterion", "score_model":
			if item.Change != VersionDiffUnchanged {
				hasScoring = true
			}
		default:
			if item.Change == VersionDiffUnchanged {
				continue
			}
			if isNonMaterialDiffItem(item) {
				continue
			}
			hasStructural = true
			hasNonMaterialOnly = false
		}
	}

	if hasKnockout || hasScoring || hasStructural {
		hasNonMaterialOnly = false
	}

	classes := make([]string, 0, 6)
	if hasNonMaterialOnly && !hasKnockout && !hasScoring && !hasStructural {
		classes = append(classes, ChangeImpactClassNonMaterial)
	}
	if hasStructural && affectedDraftCount == 0 && affectedSubmittedCount == 0 {
		classes = append(classes, ChangeImpactClassMaterialNoResponses)
	}
	if hasStructural && affectedDraftCount > 0 {
		classes = append(classes, ChangeImpactClassMaterialWithDraftResponses)
	}
	if hasStructural && affectedSubmittedCount > 0 {
		classes = append(classes, ChangeImpactClassMaterialWithSubmitted)
	}
	if hasScoring {
		classes = append(classes, ChangeImpactClassScoringAffecting)
	}
	if hasKnockout {
		classes = append(classes, ChangeImpactClassKnockoutAffecting)
	}
	if len(classes) == 0 {
		classes = append(classes, ChangeImpactClassNonMaterial)
	}

	return ChangeImpactClassification{
		ImpactClasses:                  SortImpactClasses(classes),
		AffectedDraftResponseCount:     affectedDraftCount,
		AffectedSubmittedResponseCount: affectedSubmittedCount,
		ScoringAffecting:               hasScoring,
		KnockoutAffecting:              hasKnockout,
		AffectedQuestionCodes:          affectedCodes,
	}
}

func ComputeRescoringRequired(classes []string) bool {
	for _, class := range classes {
		if class == ChangeImpactClassScoringAffecting || class == ChangeImpactClassKnockoutAffecting {
			return true
		}
	}
	return false
}

func isNonMaterialDiffItem(item CompareItemDiff) bool {
	switch item.Change {
	case VersionDiffUnchanged:
		return true
	case VersionDiffReordered:
		return true
	case VersionDiffChanged:
		return isNonMaterialFieldSet(item.Fields)
	case VersionDiffAdded, VersionDiffRemoved:
		return false
	default:
		return false
	}
}

func isNonMaterialFieldSet(fields []string) bool {
	if len(fields) == 0 {
		return true
	}
	for _, field := range fields {
		switch field {
		case "label", "help_text", "sort_order":
		default:
			return false
		}
	}
	return true
}

func bindingChangeAffectsKnockout(item CompareItemDiff) bool {
	switch item.Change {
	case VersionDiffAdded:
		return bindingKnockoutRulePresent(item.FieldDiffs, true)
	case VersionDiffRemoved:
		return bindingKnockoutRulePresent(item.FieldDiffs, false)
	case VersionDiffChanged:
		for _, field := range item.Fields {
			if field == "knockout_rule_json" {
				return true
			}
		}
	}
	return false
}

func bindingKnockoutRulePresent(diffs []CompareFieldDiff, useAfter bool) bool {
	for _, diff := range diffs {
		if diff.Field != "knockout_rule_json" {
			continue
		}
		value := diff.Before
		if useAfter {
			value = diff.After
		}
		return isMeaningfulKnockoutRule(normalizeCompareValue(value))
	}
	return false
}

func isMeaningfulKnockoutRule(normalized string) bool {
	return normalized != "" && normalized != "null" && normalized != "{}"
}

func bindingChangeAffectsScoring(item CompareItemDiff) bool {
	if item.Change == VersionDiffAdded || item.Change == VersionDiffRemoved {
		return true
	}
	for _, field := range item.Fields {
		switch field {
		case "scoring_rule_json", "binding_type":
			return true
		}
	}
	return false
}

func extractAffectedQuestionCodes(diff CompareVersionsResult) []string {
	scope := BuildResponseImpactScope(diff)
	codes := make(map[string]struct{}, len(scope.QuestionCodesWithAnswers))
	for _, code := range scope.QuestionCodesWithAnswers {
		codes[code] = struct{}{}
	}
	if scope.CountAllResponsesOnVersion {
		for _, q := range diff.Questions {
			if q.QuestionCode != "" {
				codes[q.QuestionCode] = struct{}{}
			}
		}
	}
	out := make([]string, 0, len(codes))
	for code := range codes {
		out = append(out, code)
	}
	sort.Strings(out)
	return out
}

func BuildResponseImpactScope(diff CompareVersionsResult) ResponseImpactScope {
	scope := ResponseImpactScope{}
	questionCodesAnswered := make(map[string]struct{})

	for _, item := range diff.Differences {
		if item.Change == VersionDiffUnchanged || isNonMaterialDiffItem(item) {
			continue
		}
		switch item.EntityType {
		case "question":
			switch item.Change {
			case VersionDiffAdded:
				if questionDiffTargetRequired(item) {
					scope.CountAllResponsesOnVersion = true
				}
			case VersionDiffRemoved:
				if item.QuestionCode != "" {
					questionCodesAnswered[item.QuestionCode] = struct{}{}
				}
			case VersionDiffChanged:
				if questionDiffBecameRequired(item) {
					scope.CountAllResponsesOnVersion = true
				} else if item.QuestionCode != "" {
					questionCodesAnswered[item.QuestionCode] = struct{}{}
				}
			}
		case "option":
			if item.QuestionCode != "" {
				questionCodesAnswered[item.QuestionCode] = struct{}{}
			}
		case "section", "questionnaire_enabled":
			scope.CountAllResponsesOnVersion = true
		case "rule":
			scope.CountAllResponsesOnVersion = true
		}
	}

	scope.QuestionCodesWithAnswers = make([]string, 0, len(questionCodesAnswered))
	for code := range questionCodesAnswered {
		scope.QuestionCodesWithAnswers = append(scope.QuestionCodesWithAnswers, code)
	}
	sort.Strings(scope.QuestionCodesWithAnswers)
	return scope
}

func questionDiffTargetRequired(item CompareItemDiff) bool {
	for _, diff := range item.FieldDiffs {
		if diff.Field == "required" && normalizeCompareValue(diff.After) == "true" {
			return true
		}
	}
	return false
}

func questionDiffBecameRequired(item CompareItemDiff) bool {
	for _, diff := range item.FieldDiffs {
		if diff.Field != "required" {
			continue
		}
		if normalizeCompareValue(diff.Before) == "false" && normalizeCompareValue(diff.After) == "true" {
			return true
		}
	}
	return false
}

func ChangeImpactConfirmationRequired() error {
	return apperrors.Validation("change impact confirmation is required before republishing this questionnaire", map[string]any{
		"code": VersionLifecycleMachineCodeChangeImpactRequired,
	})
}

func ChangeImpactAnalysisExpired() error {
	return apperrors.Validation("change impact analysis has expired", map[string]any{
		"code": VersionLifecycleMachineCodeChangeImpactExpired,
	})
}

func ChangeImpactAnalysisConsumed() error {
	return apperrors.Conflict("change impact analysis was already consumed", map[string]any{
		"code": VersionLifecycleMachineCodeChangeImpactConsumed,
	})
}

func ChangeImpactStaleDiff() error {
	return apperrors.Conflict("candidate questionnaire changed after impact preview", map[string]any{
		"code":  VersionLifecycleMachineCodeChangeImpactStaleDiff,
		"field": "canonical_diff_hash",
	})
}

func ChangeImpactAnalysisNotFound() error {
	return apperrors.NotFound("change impact analysis not found")
}

func ValidateImpactConfirmationFields(in PublishQuestionnaireInput) error {
	if in.ImpactAnalysisID == nil || *in.ImpactAnalysisID == uuid.Nil {
		return apperrors.Validation("impact_analysis_id is required", map[string]any{"field": "impact_analysis_id"})
	}
	if strings.TrimSpace(in.CanonicalDiffHash) == "" {
		return apperrors.Validation("canonical_diff_hash is required", map[string]any{"field": "canonical_diff_hash"})
	}
	return nil
}
