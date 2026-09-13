package xlsxexchange

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/google/uuid"

	"github.com/freight-platform/rfx-service/internal/domain"
)

type canonicalImportPayload struct {
	SchemaName            string                   `json:"schema_name"`
	SchemaVersion         string                   `json:"schema_version"`
	Mode                  string                   `json:"mode"`
	TargetEventID         uuid.UUID                `json:"target_event_id"`
	TargetDraftVersionID  uuid.UUID                `json:"target_draft_version_id"`
	TargetVersionNumber   int                      `json:"target_version_number"`
	EventRowVersion       int                      `json:"event_row_version"`
	DraftRowVersion       int                      `json:"draft_row_version"`
	Lots                  []canonicalLot           `json:"lots"`
	Questionnaire         canonicalQuestionnaire   `json:"questionnaire"`
	QuestionnaireDiffHash string                   `json:"questionnaire_diff_hash"`
	LotsDiffHash          string                   `json:"lots_diff_hash"`
	Counts                canonicalImportCounts    `json:"counts"`
	CommitAffectingWarns  []canonicalCommitWarning `json:"commit_affecting_warnings,omitempty"`
}

type canonicalImportCounts struct {
	Errors    int `json:"errors"`
	Warnings  int `json:"warnings"`
	Lots      int `json:"lots"`
	Sections  int `json:"sections"`
	Questions int `json:"questions"`
	Options   int `json:"options"`
	Rules     int `json:"rules"`
}

type canonicalCommitWarning struct {
	MachineCode string `json:"machine_code"`
	Sheet       string `json:"sheet,omitempty"`
	Row         int    `json:"row,omitempty"`
	Column      string `json:"column,omitempty"`
	StableCode  string `json:"stable_code,omitempty"`
}

type canonicalLot struct {
	LotNumber      string   `json:"lot_number"`
	Name           string   `json:"name"`
	Description    string   `json:"description,omitempty"`
	Category       string   `json:"category,omitempty"`
	EstimatedValue *float64 `json:"estimated_value,omitempty"`
	CurrencyCode   string   `json:"currency_code,omitempty"`
	Status         string   `json:"status"`
}

type canonicalQuestionnaire struct {
	Sections  []canonicalSection  `json:"sections"`
	Questions []canonicalQuestion `json:"questions"`
	Options   []canonicalOption   `json:"options"`
	Rules     []canonicalRule     `json:"rules"`
}

type canonicalSection struct {
	SectionCode string `json:"section_code"`
	Title       string `json:"title"`
	Description string `json:"description,omitempty"`
	SortOrder   int    `json:"sort_order"`
}

type canonicalQuestion struct {
	SectionCode    string          `json:"section_code"`
	QuestionCode   string          `json:"question_code"`
	QuestionType   string          `json:"question_type"`
	Label          string          `json:"label"`
	HelpText       string          `json:"help_text,omitempty"`
	Required       bool            `json:"required"`
	SortOrder      int             `json:"sort_order"`
	ValidationJSON json.RawMessage `json:"validation_json,omitempty"`
}

type canonicalOption struct {
	QuestionCode string `json:"question_code"`
	OptionCode   string `json:"option_code"`
	Label        string `json:"label"`
	SortOrder    int    `json:"sort_order"`
}

type canonicalRule struct {
	RuleCode           string          `json:"rule_code"`
	SourceQuestionCode string          `json:"source_question_code"`
	ConditionJSON      json.RawMessage `json:"condition_json"`
	TargetQuestionCode string          `json:"target_question_code"`
	Action             string          `json:"action"`
	SortOrder          int             `json:"sort_order"`
}

func computeCanonicalPayloadHash(
	target TargetDraftBaseline,
	proposal BuyerImportProposal,
	qDiff domain.CompareVersionsResult,
	lDiff LotsCompareResult,
	errors, warnings []BuyerImportIssue,
) (string, error) {
	payload := buildCanonicalImportPayload(target, proposal, qDiff, lDiff, errors, warnings)
	raw, err := marshalCanonical(payload)
	if err != nil {
		return "", err
	}
	_, hash, err := StableStoredPayload(raw)
	return hash, err
}

// CanonicalImportPayloadJSON returns deterministic JSON bytes for immutable preview persistence.
func CanonicalImportPayloadJSON(preview BuyerImportPreview, target TargetDraftBaseline, proposal BuyerImportProposal) ([]byte, error) {
	payload := buildCanonicalImportPayload(target, proposal, preview.QuestionnaireDiff, preview.LotsDiff, preview.Errors, preview.Warnings)
	raw, err := marshalCanonical(payload)
	if err != nil {
		return nil, err
	}
	stored, _, err := StableStoredPayload(raw)
	return stored, err
}

// StableStoredPayload re-canonicalizes import payload bytes for JSONB-stable hashing and persistence.
func StableStoredPayload(payloadJSON []byte) ([]byte, string, error) {
	pgLike, err := normalizeJSONDocument(payloadJSON)
	if err != nil {
		return nil, "", err
	}
	var payload canonicalImportPayload
	if err := json.Unmarshal(pgLike, &payload); err != nil {
		return nil, "", err
	}
	if err := normalizeCanonicalImportPayload(&payload); err != nil {
		return nil, "", err
	}
	normalized, err := marshalCanonical(payload)
	if err != nil {
		return nil, "", err
	}
	sum := sha256.Sum256(normalized)
	return normalized, hex.EncodeToString(sum[:]), nil
}

func normalizeJSONDocument(payloadJSON []byte) ([]byte, error) {
	if len(bytes.TrimSpace(payloadJSON)) == 0 {
		return payloadJSON, nil
	}
	var decoded any
	if err := json.Unmarshal(payloadJSON, &decoded); err != nil {
		return nil, err
	}
	return json.Marshal(decoded)
}

func normalizeJSONBytes(raw json.RawMessage) (json.RawMessage, error) {
	if len(bytes.TrimSpace(raw)) == 0 {
		return nil, nil
	}
	var decoded any
	if err := json.Unmarshal(raw, &decoded); err != nil {
		return nil, err
	}
	out, err := json.Marshal(decoded)
	if err != nil {
		return nil, err
	}
	return json.RawMessage(out), nil
}

func normalizeCanonicalImportPayload(payload *canonicalImportPayload) error {
	for idx := range payload.Questionnaire.Questions {
		normalized, err := normalizeJSONBytes(payload.Questionnaire.Questions[idx].ValidationJSON)
		if err != nil {
			return err
		}
		payload.Questionnaire.Questions[idx].ValidationJSON = normalized
	}
	for idx := range payload.Questionnaire.Rules {
		normalized, err := normalizeJSONBytes(payload.Questionnaire.Rules[idx].ConditionJSON)
		if err != nil {
			return err
		}
		payload.Questionnaire.Rules[idx].ConditionJSON = normalized
	}
	return nil
}

// StableStoredPayloadHash returns the JSONB-stable SHA-256 hex digest for canonical payload bytes.
func StableStoredPayloadHash(payloadJSON []byte) (string, error) {
	_, hash, err := StableStoredPayload(payloadJSON)
	return hash, err
}

// VerifyStoredCanonicalPayloadHash checks hash against JSONB-round-trip-safe canonical bytes.
func VerifyStoredCanonicalPayloadHash(payloadJSON []byte, hash string) error {
	_, computed, err := StableStoredPayload(payloadJSON)
	if err != nil {
		return err
	}
	if strings.ToLower(strings.TrimSpace(hash)) != computed {
		return fmt.Errorf("canonical hash mismatch")
	}
	return nil
}

func buildCanonicalImportPayload(
	target TargetDraftBaseline,
	proposal BuyerImportProposal,
	qDiff domain.CompareVersionsResult,
	lDiff LotsCompareResult,
	errors, warnings []BuyerImportIssue,
) canonicalImportPayload {
	sections, questions, options := flattenCanonicalQuestionnaire(proposal.Questionnaire)
	return canonicalImportPayload{
		SchemaName:           domain.SchemaVersionBuyerXLSXV1,
		SchemaVersion:        schemaVersionNumber,
		Mode:                 BuyerImportModeUpdateDraft,
		TargetEventID:        target.EventID,
		TargetDraftVersionID: target.DraftVersionID,
		TargetVersionNumber:  target.DraftVersionNumber,
		EventRowVersion:      target.EventRowVersion,
		DraftRowVersion:      target.DraftRowVersion,
		Lots:                 canonicalLots(proposal.Lots),
		Questionnaire: canonicalQuestionnaire{
			Sections:  sections,
			Questions: questions,
			Options:   options,
			Rules:     canonicalRules(proposal.Questionnaire.Rules, proposal.Questionnaire.Sections),
		},
		QuestionnaireDiffHash: qDiff.CanonicalDiffHash,
		LotsDiffHash:          lDiff.CanonicalDiffHash,
		Counts: canonicalImportCounts{
			Errors:    len(errors),
			Warnings:  len(warnings),
			Lots:      len(proposal.Lots),
			Sections:  len(sections),
			Questions: len(questions),
			Options:   len(options),
			Rules:     len(proposal.Questionnaire.Rules),
		},
		CommitAffectingWarns: commitAffectingWarnings(warnings),
	}
}

func commitAffectingWarnings(warnings []BuyerImportIssue) []canonicalCommitWarning {
	out := make([]canonicalCommitWarning, 0)
	for _, warn := range warnings {
		if warn.MachineCode != MachineCodeI18NMonolingualMismatch && warn.MachineCode != MachineCodeHiddenContentWarning {
			continue
		}
		out = append(out, canonicalCommitWarning{
			MachineCode: warn.MachineCode,
			Sheet:       warn.Sheet,
			Row:         warn.Row,
			Column:      warn.Column,
			StableCode:  warn.StableCode,
		})
	}
	return out
}

func canonicalLots(lots []domain.RfxLot) []canonicalLot {
	sorted := append([]domain.RfxLot(nil), lots...)
	sortLots(sorted)
	out := make([]canonicalLot, 0, len(sorted))
	for _, lot := range sorted {
		out = append(out, canonicalLot{
			LotNumber:      lot.LotNumber,
			Name:           lot.Name,
			Description:    optionalString(lot.Description),
			Category:       optionalString(lot.Category),
			EstimatedValue: lot.EstimatedValue,
			CurrencyCode:   optionalString(lot.CurrencyCode),
			Status:         lot.Status,
		})
	}
	return out
}

func flattenCanonicalQuestionnaire(def domain.QuestionnaireDefinition) ([]canonicalSection, []canonicalQuestion, []canonicalOption) {
	sections := make([]canonicalSection, 0)
	questions := make([]canonicalQuestion, 0)
	options := make([]canonicalOption, 0)
	sorted := append([]domain.SectionWithQuestions(nil), def.Sections...)
	sortSections(sorted)
	for _, swq := range sorted {
		sections = append(sections, canonicalSection{
			SectionCode: swq.Section.SectionCode,
			Title:       swq.Section.Title,
			Description: optionalString(swq.Section.Description),
			SortOrder:   swq.Section.SortOrder,
		})
		for _, q := range swq.Questions {
			validationJSON, err := normalizeJSONBytes(q.ValidationRuleJSON)
			if err != nil {
				validationJSON = q.ValidationRuleJSON
			}
			questions = append(questions, canonicalQuestion{
				SectionCode:    swq.Section.SectionCode,
				QuestionCode:   q.QuestionCode,
				QuestionType:   q.QuestionType,
				Label:          q.Label,
				HelpText:       optionalString(q.HelpText),
				Required:       q.Required,
				SortOrder:      q.SortOrder,
				ValidationJSON: validationJSON,
			})
			for _, opt := range q.Options {
				options = append(options, canonicalOption{
					QuestionCode: q.QuestionCode,
					OptionCode:   opt.OptionCode,
					Label:        opt.Label,
					SortOrder:    opt.SortOrder,
				})
			}
		}
	}
	return sections, questions, options
}

func canonicalRules(rules []domain.QuestionRule, sections []domain.SectionWithQuestions) []canonicalRule {
	codeByID := make(map[uuid.UUID]string)
	for _, swq := range sections {
		for _, q := range swq.Questions {
			codeByID[q.ID] = q.QuestionCode
		}
	}
	sorted := append([]domain.QuestionRule(nil), rules...)
	sortRules(sorted)
	out := make([]canonicalRule, 0, len(sorted))
	for _, rule := range sorted {
		targetCode := ""
		if rule.TargetQuestionID != nil {
			targetCode = codeByID[*rule.TargetQuestionID]
		}
		sourceCode := ExtractSourceQuestionCode(rule.ConditionJSON)
		conditionJSON, err := normalizeJSONBytes(rule.ConditionJSON)
		if err != nil {
			conditionJSON = rule.ConditionJSON
		}
		out = append(out, canonicalRule{
			RuleCode:           rule.RuleCode,
			SourceQuestionCode: sourceCode,
			ConditionJSON:      conditionJSON,
			TargetQuestionCode: targetCode,
			Action:             rule.Action,
			SortOrder:          rule.SortOrder,
		})
	}
	return out
}
