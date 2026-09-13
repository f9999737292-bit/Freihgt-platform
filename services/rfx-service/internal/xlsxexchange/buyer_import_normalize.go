package xlsxexchange

import (
	"encoding/json"
	"sort"
	"strconv"
	"strings"

	"github.com/google/uuid"

	"github.com/freight-platform/rfx-service/internal/domain"
)

const buyerImportUUIDNamespace = "BINTRANS_RFX_BUYER_XLSX_IMPORT_V1"

func stableImportUUID(kind, code string) uuid.UUID {
	return uuid.NewSHA1(uuid.NameSpaceOID, []byte(buyerImportUUIDNamespace+":"+kind+":"+code))
}

func normalizeI18NValue(sheet, column, stableCode string, row int, ru, en, zh string, issues *issueCollector) string {
	ru = stringsTrimSpace(ru)
	en = stringsTrimSpace(en)
	zh = stringsTrimSpace(zh)
	if en == ru && zh == ru {
		return ru
	}
	issues.addWarning(issueWarning(
		MachineCodeI18NMonolingualMismatch,
		"rfx.buyer_xlsx_import.i18n_monolingual_mismatch",
		sheet, column, stableCode, row,
		map[string]any{"chosen": "ru", "ru": ru, "en": en, "zh": zh},
	))
	return ru
}

func normalizeOptionalString(value string) *string {
	trimmed := stringsTrimSpace(value)
	if trimmed == "" {
		return nil
	}
	return &trimmed
}

func parseRequiredBool(value, sheet, column, stableCode string, row int, issues *issueCollector) (bool, bool) {
	trimmed := strings.ToLower(stringsTrimSpace(value))
	switch trimmed {
	case "true":
		return true, true
	case "false":
		return false, true
	default:
		issues.addError(issueError(
			MachineCodeInvalidType,
			"rfx.buyer_xlsx_import.invalid_bool",
			sheet, column, stableCode, row,
			map[string]any{"value": value},
		))
		return false, false
	}
}

func parseRequiredInt(value, sheet, column, stableCode string, row int, issues *issueCollector) (int, bool) {
	trimmed := stringsTrimSpace(value)
	if trimmed == "" {
		issues.addError(issueError(
			MachineCodeMissingRequiredValue,
			"rfx.buyer_xlsx_import.missing_required_value",
			sheet, column, stableCode, row, nil,
		))
		return 0, false
	}
	parsed, err := strconv.Atoi(trimmed)
	if err != nil {
		issues.addError(issueError(
			MachineCodeInvalidType,
			"rfx.buyer_xlsx_import.invalid_int",
			sheet, column, stableCode, row,
			map[string]any{"value": value},
		))
		return 0, false
	}
	return parsed, true
}

func parseOptionalFloat(value, sheet, column, stableCode string, row int, issues *issueCollector) (*float64, bool) {
	trimmed := stringsTrimSpace(value)
	if trimmed == "" {
		return nil, true
	}
	parsed, err := strconv.ParseFloat(trimmed, 64)
	if err != nil {
		issues.addError(issueError(
			MachineCodeInvalidType,
			"rfx.buyer_xlsx_import.invalid_float",
			sheet, column, stableCode, row,
			map[string]any{"value": value},
		))
		return nil, false
	}
	return &parsed, true
}

func parseCanonicalJSONField(raw, sheet, column, stableCode string, row int, issues *issueCollector) (json.RawMessage, bool) {
	trimmed := stringsTrimSpace(raw)
	if trimmed == "" || trimmed == "null" {
		return nil, true
	}
	canonical, err := formatCanonicalJSON(json.RawMessage(trimmed))
	if err != nil {
		issues.addError(issueError(
			MachineCodeInvalidType,
			"rfx.buyer_xlsx_import.invalid_json",
			sheet, column, stableCode, row,
			map[string]any{"value": raw},
		))
		return nil, false
	}
	return json.RawMessage(canonical), true
}

func enforceStringLength(value, sheet, column, stableCode string, row, maxLen int, issues *issueCollector) bool {
	if len([]rune(value)) > maxLen {
		issues.addError(issueError(
			MachineCodeInvalidType,
			"rfx.buyer_xlsx_import.string_too_long",
			sheet, column, stableCode, row,
			map[string]any{"max_length": maxLen},
		))
		return false
	}
	return true
}

func sortLots(lots []domain.RfxLot) {
	sort.SliceStable(lots, func(i, j int) bool {
		if lots[i].LotNumber != lots[j].LotNumber {
			return lots[i].LotNumber < lots[j].LotNumber
		}
		return lots[i].Name < lots[j].Name
	})
}

func sortSections(sections []domain.SectionWithQuestions) {
	sort.SliceStable(sections, func(i, j int) bool {
		if sections[i].Section.SortOrder != sections[j].Section.SortOrder {
			return sections[i].Section.SortOrder < sections[j].Section.SortOrder
		}
		return sections[i].Section.SectionCode < sections[j].Section.SectionCode
	})
	for idx := range sections {
		sort.SliceStable(sections[idx].Questions, func(i, j int) bool {
			if sections[idx].Questions[i].SortOrder != sections[idx].Questions[j].SortOrder {
				return sections[idx].Questions[i].SortOrder < sections[idx].Questions[j].SortOrder
			}
			return sections[idx].Questions[i].QuestionCode < sections[idx].Questions[j].QuestionCode
		})
		for qIdx := range sections[idx].Questions {
			sort.SliceStable(sections[idx].Questions[qIdx].Options, func(i, j int) bool {
				if sections[idx].Questions[qIdx].Options[i].SortOrder != sections[idx].Questions[qIdx].Options[j].SortOrder {
					return sections[idx].Questions[qIdx].Options[i].SortOrder < sections[idx].Questions[qIdx].Options[j].SortOrder
				}
				return sections[idx].Questions[qIdx].Options[i].OptionCode < sections[idx].Questions[qIdx].Options[j].OptionCode
			})
		}
	}
}

func sortRules(rules []domain.QuestionRule) {
	sort.SliceStable(rules, func(i, j int) bool {
		if rules[i].SortOrder != rules[j].SortOrder {
			return rules[i].SortOrder < rules[j].SortOrder
		}
		return rules[i].RuleCode < rules[j].RuleCode
	})
}

func proposalToQuestionnaireDefinition(target TargetDraftBaseline, proposal BuyerImportProposal) domain.QuestionnaireDefinition {
	return domain.QuestionnaireDefinition{
		EventID:              target.EventID,
		RfxVersionID:         target.DraftVersionID,
		VersionNumber:        target.DraftVersionNumber,
		QuestionnaireEnabled: target.Questionnaire.QuestionnaireEnabled,
		VersionStatus:        domain.RfxVersionStatusDraft,
		Sections:             append([]domain.SectionWithQuestions(nil), proposal.Questionnaire.Sections...),
		Rules:                append([]domain.QuestionRule(nil), proposal.Questionnaire.Rules...),
	}
}

func baselineCompareSnapshot(target TargetDraftBaseline) domain.VersionCompareSnapshot {
	return domain.VersionCompareSnapshot{
		Version:       target.Questionnaire.Version(),
		Questionnaire: target.Questionnaire,
	}
}

func proposalCompareSnapshot(target TargetDraftBaseline, proposal BuyerImportProposal) domain.VersionCompareSnapshot {
	return domain.VersionCompareSnapshot{
		Version: domain.RfxVersion{
			ID:                   target.DraftVersionID,
			RfxEventID:           target.EventID,
			VersionNumber:        target.DraftVersionNumber,
			Status:               domain.RfxVersionStatusDraft,
			QuestionnaireEnabled: target.Questionnaire.QuestionnaireEnabled,
		},
		Questionnaire: proposalToQuestionnaireDefinition(target, proposal),
	}
}
