package xlsxexchange

import (
	"bytes"
	"encoding/json"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/xuri/excelize/v2"

	"github.com/freight-platform/rfx-service/internal/domain"
	"github.com/freight-platform/rfx-service/internal/xlsxsecurity"
)

const (
	sheetInstructions = "Instructions"
	sheetMetadata     = "Metadata"
	sheetLots         = "Lots"
	sheetSections     = "Sections"
	sheetQuestions    = "Questions"
	sheetOptions      = "Options"
	sheetRules        = "Rules"
)

var buyerSheetOrder = []string{
	sheetInstructions,
	sheetMetadata,
	sheetLots,
	sheetSections,
	sheetQuestions,
	sheetOptions,
	sheetRules,
}

var instructionRows = [][3]string{
	{
		"Схема: BINTRANS_RFX_BUYER_XLSX_V1",
		"Schema: BINTRANS_RFX_BUYER_XLSX_V1",
		"架构: BINTRANS_RFX_BUYER_XLSX_V1",
	},
	{
		"Версия схемы: 1",
		"Schema version: 1",
		"架构版本: 1",
	},
	{
		"Назначение: экспорт черновика тендера заказчика для офлайн-редактирования.",
		"Purpose: export buyer draft tender for offline editing.",
		"用途: 导出买方草稿招标以供离线编辑。",
	},
	{
		"Экспорт является снимком активного DRAFT и не публикует тендер.",
		"Export is a snapshot of the active DRAFT and does not publish the tender.",
		"导出是活动 DRAFT 的快照，不会发布招标。",
	},
	{
		"Импорт из этого файла будет отдельной операцией.",
		"Import from this file is a separate operation.",
		"从此文件导入是单独的操作。",
	},
	{
		"Не добавляйте формулы, макросы или внешние ссылки.",
		"Do not add formulas, macros, or external links.",
		"请勿添加公式、宏或外部链接。",
	},
}

// GenerateBuyerDraftWorkbook renders snapshot into a secure BUYER XLSX V1 workbook.
func GenerateBuyerDraftWorkbook(snapshot BuyerDraftSnapshot) ([]byte, error) {
	f := excelize.NewFile()
	defer func() {
		_ = f.Close()
	}()

	if err := f.SetSheetName("Sheet1", sheetInstructions); err != nil {
		return nil, fmt.Errorf("rename instructions sheet: %w", err)
	}
	for _, name := range buyerSheetOrder[1:] {
		if _, err := f.NewSheet(name); err != nil {
			return nil, fmt.Errorf("create sheet %s: %w", name, err)
		}
	}

	if err := writeInstructionsSheet(f); err != nil {
		return nil, err
	}
	if err := writeMetadataSheet(f, snapshot.Metadata); err != nil {
		return nil, err
	}
	if err := writeLotsSheet(f, snapshot.Lots); err != nil {
		return nil, err
	}
	if err := writeSectionsSheet(f, snapshot.Sections); err != nil {
		return nil, err
	}
	if err := writeQuestionsSheet(f, snapshot.Questions); err != nil {
		return nil, err
	}
	if err := writeOptionsSheet(f, snapshot.Options); err != nil {
		return nil, err
	}
	if err := writeRulesSheet(f, snapshot.Rules); err != nil {
		return nil, err
	}

	var buf bytes.Buffer
	if err := f.Write(&buf); err != nil {
		return nil, fmt.Errorf("write workbook: %w", err)
	}
	data := buf.Bytes()
	if int64(len(data)) > xlsxsecurity.DefaultMaxUploadBytes {
		return nil, fmt.Errorf("workbook exceeds max upload size")
	}
	return data, nil
}

func writeInstructionsSheet(f *excelize.File) error {
	for rowIdx, cols := range instructionRows {
		row := rowIdx + 1
		for colIdx, value := range cols {
			cell, err := excelize.CoordinatesToCellName(colIdx+1, row)
			if err != nil {
				return err
			}
			if err := setTextCell(f, sheetInstructions, cell, value); err != nil {
				return err
			}
		}
	}
	return nil
}

func writeMetadataSheet(f *excelize.File, md BuyerDraftMetadata) error {
	rows := [][2]string{
		{"schema_name", domain.SchemaVersionBuyerXLSXV1},
		{"schema_version", schemaVersionNumber},
		{"exported_at_utc", md.ExportedAtUTC.UTC().Format(time.RFC3339)},
		{"tenant_id", md.TenantID.String()},
		{"rfx_event_id", md.RfxEventID.String()},
		{"rfx_version_id", md.RfxVersionID.String()},
		{"version_number", strconv.Itoa(md.VersionNumber)},
		{"version_status", md.VersionStatus},
		{"event_row_version", strconv.Itoa(md.EventRowVersion)},
		{"version_row_version", strconv.Itoa(md.VersionRowVersion)},
		{"creation_channel", md.CreationChannel},
		{"source_template_version_id", optionalUUID(md.SourceTemplateVersionID)},
		{"source_template_version_number", optionalInt(md.SourceTemplateVersionNumber)},
	}
	for rowIdx, row := range rows {
		rowNum := rowIdx + 1
		if err := setTextCell(f, sheetMetadata, fmt.Sprintf("A%d", rowNum), row[0]); err != nil {
			return err
		}
		if err := setTextCell(f, sheetMetadata, fmt.Sprintf("B%d", rowNum), row[1]); err != nil {
			return err
		}
	}
	return nil
}

func writeLotsSheet(f *excelize.File, lots []domain.RfxLot) error {
	headers := []string{"lot_number", "name", "description", "category", "estimated_value", "currency_code", "status"}
	if err := writeHeaderRow(f, sheetLots, headers); err != nil {
		return err
	}
	sortedLots := append([]domain.RfxLot(nil), lots...)
	sort.SliceStable(sortedLots, func(i, j int) bool {
		li, lj := sortedLots[i].LotNumber, sortedLots[j].LotNumber
		if li != lj {
			return li < lj
		}
		return sortedLots[i].Name < sortedLots[j].Name
	})
	for rowIdx, lot := range sortedLots {
		rowNum := rowIdx + 2
		values := []string{
			lot.LotNumber,
			lot.Name,
			optionalString(lot.Description),
			optionalString(lot.Category),
			optionalFloat(lot.EstimatedValue),
			optionalString(lot.CurrencyCode),
			lot.Status,
		}
		if err := writeDataRow(f, sheetLots, rowNum, values); err != nil {
			return err
		}
	}
	return nil
}

func writeSectionsSheet(f *excelize.File, sections []domain.Section) error {
	headers := []string{"section_code", "title_ru", "title_en", "title_zh", "description_ru", "description_en", "description_zh", "sort_order"}
	if err := writeHeaderRow(f, sheetSections, headers); err != nil {
		return err
	}
	sortedSections := append([]domain.Section(nil), sections...)
	sort.SliceStable(sortedSections, func(i, j int) bool {
		if sortedSections[i].SortOrder != sortedSections[j].SortOrder {
			return sortedSections[i].SortOrder < sortedSections[j].SortOrder
		}
		return sortedSections[i].SectionCode < sortedSections[j].SectionCode
	})
	for rowIdx, section := range sortedSections {
		rowNum := rowIdx + 2
		desc := optionalString(section.Description)
		values := []string{
			section.SectionCode,
			section.Title, section.Title, section.Title,
			desc, desc, desc,
			strconv.Itoa(section.SortOrder),
		}
		if err := writeDataRow(f, sheetSections, rowNum, values); err != nil {
			return err
		}
	}
	return nil
}

func writeQuestionsSheet(f *excelize.File, questions []BuyerDraftQuestion) error {
	headers := []string{
		"section_code", "question_code", "question_type",
		"title_ru", "title_en", "title_zh",
		"description_ru", "description_en", "description_zh",
		"required", "sort_order", "validation_json",
	}
	if err := writeHeaderRow(f, sheetQuestions, headers); err != nil {
		return err
	}
	sortedQuestions := append([]BuyerDraftQuestion(nil), questions...)
	sort.SliceStable(sortedQuestions, func(i, j int) bool {
		if sortedQuestions[i].Question.SortOrder != sortedQuestions[j].Question.SortOrder {
			return sortedQuestions[i].Question.SortOrder < sortedQuestions[j].Question.SortOrder
		}
		return sortedQuestions[i].Question.QuestionCode < sortedQuestions[j].Question.QuestionCode
	})
	for rowIdx, item := range sortedQuestions {
		rowNum := rowIdx + 2
		q := item.Question
		help := optionalString(q.HelpText)
		validationJSON, err := formatCanonicalJSON(q.ValidationRuleJSON)
		if err != nil {
			return fmt.Errorf("canonical validation_json for %s: %w", q.QuestionCode, err)
		}
		required := "false"
		if q.Required {
			required = "true"
		}
		values := []string{
			item.SectionCode,
			q.QuestionCode,
			q.QuestionType,
			q.Label, q.Label, q.Label,
			help, help, help,
			required,
			strconv.Itoa(q.SortOrder),
			validationJSON,
		}
		if err := writeDataRow(f, sheetQuestions, rowNum, values); err != nil {
			return err
		}
	}
	return nil
}

func writeOptionsSheet(f *excelize.File, options []BuyerDraftOption) error {
	headers := []string{"question_code", "option_code", "label_ru", "label_en", "label_zh", "sort_order"}
	if err := writeHeaderRow(f, sheetOptions, headers); err != nil {
		return err
	}
	sortedOptions := append([]BuyerDraftOption(nil), options...)
	sort.SliceStable(sortedOptions, func(i, j int) bool {
		if sortedOptions[i].Option.SortOrder != sortedOptions[j].Option.SortOrder {
			return sortedOptions[i].Option.SortOrder < sortedOptions[j].Option.SortOrder
		}
		if sortedOptions[i].QuestionCode != sortedOptions[j].QuestionCode {
			return sortedOptions[i].QuestionCode < sortedOptions[j].QuestionCode
		}
		return sortedOptions[i].Option.OptionCode < sortedOptions[j].Option.OptionCode
	})
	for rowIdx, item := range sortedOptions {
		rowNum := rowIdx + 2
		values := []string{
			item.QuestionCode,
			item.Option.OptionCode,
			item.Option.Label, item.Option.Label, item.Option.Label,
			strconv.Itoa(item.Option.SortOrder),
		}
		if err := writeDataRow(f, sheetOptions, rowNum, values); err != nil {
			return err
		}
	}
	return nil
}

func writeRulesSheet(f *excelize.File, rules []BuyerDraftRule) error {
	headers := []string{"rule_code", "source_question_code", "condition", "target_question_code", "action", "sort_order"}
	if err := writeHeaderRow(f, sheetRules, headers); err != nil {
		return err
	}
	sortedRules := append([]BuyerDraftRule(nil), rules...)
	sort.SliceStable(sortedRules, func(i, j int) bool {
		if sortedRules[i].SortOrder != sortedRules[j].SortOrder {
			return sortedRules[i].SortOrder < sortedRules[j].SortOrder
		}
		return sortedRules[i].RuleCode < sortedRules[j].RuleCode
	})
	for rowIdx, rule := range sortedRules {
		rowNum := rowIdx + 2
		conditionJSON, err := formatCanonicalJSON(rule.ConditionJSON)
		if err != nil {
			return fmt.Errorf("canonical condition for %s: %w", rule.RuleCode, err)
		}
		values := []string{
			rule.RuleCode,
			rule.SourceQuestionCode,
			conditionJSON,
			rule.TargetQuestionCode,
			rule.Action,
			strconv.Itoa(rule.SortOrder),
		}
		if err := writeDataRow(f, sheetRules, rowNum, values); err != nil {
			return err
		}
	}
	return nil
}

func writeHeaderRow(f *excelize.File, sheet string, headers []string) error {
	for colIdx, header := range headers {
		cell, err := excelize.CoordinatesToCellName(colIdx+1, 1)
		if err != nil {
			return err
		}
		if err := setTextCell(f, sheet, cell, header); err != nil {
			return err
		}
	}
	return nil
}

func writeDataRow(f *excelize.File, sheet string, rowNum int, values []string) error {
	for colIdx, value := range values {
		cell, err := excelize.CoordinatesToCellName(colIdx+1, rowNum)
		if err != nil {
			return err
		}
		if err := setTextCell(f, sheet, cell, value); err != nil {
			return err
		}
	}
	return nil
}

func setTextCell(f *excelize.File, sheet, cell, value string) error {
	return f.SetCellStr(sheet, cell, sanitizeCellValue(value))
}

func sanitizeCellValue(value string) string {
	if value == "" {
		return value
	}
	switch value[0] {
	case '=', '+', '-', '@':
		return "'" + value
	default:
		return value
	}
}

func formatCanonicalJSON(raw json.RawMessage) (string, error) {
	trimmed := strings.TrimSpace(string(raw))
	if trimmed == "" || trimmed == "null" {
		return "", nil
	}
	var value any
	if err := json.Unmarshal(raw, &value); err != nil {
		return "", err
	}
	out, err := marshalCanonical(value)
	if err != nil {
		return "", err
	}
	return string(out), nil
}

func marshalCanonical(value any) ([]byte, error) {
	switch typed := value.(type) {
	case map[string]any:
		keys := make([]string, 0, len(typed))
		for key := range typed {
			keys = append(keys, key)
		}
		sort.Strings(keys)
		var buf bytes.Buffer
		buf.WriteByte('{')
		for idx, key := range keys {
			if idx > 0 {
				buf.WriteByte(',')
			}
			keyJSON, err := json.Marshal(key)
			if err != nil {
				return nil, err
			}
			buf.Write(keyJSON)
			buf.WriteByte(':')
			valJSON, err := marshalCanonical(typed[key])
			if err != nil {
				return nil, err
			}
			buf.Write(valJSON)
		}
		buf.WriteByte('}')
		return buf.Bytes(), nil
	case []any:
		items := make([][]byte, 0, len(typed))
		for _, item := range typed {
			itemJSON, err := marshalCanonical(item)
			if err != nil {
				return nil, err
			}
			items = append(items, itemJSON)
		}
		return []byte("[" + joinJSON(items) + "]"), nil
	default:
		return json.Marshal(typed)
	}
}

func joinJSON(items [][]byte) string {
	parts := make([]string, len(items))
	for i, item := range items {
		parts[i] = string(item)
	}
	return strings.Join(parts, ",")
}

func optionalString(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}

func optionalUUID(value *uuid.UUID) string {
	if value == nil || *value == uuid.Nil {
		return ""
	}
	return value.String()
}

func optionalInt(value *int) string {
	if value == nil {
		return ""
	}
	return strconv.Itoa(*value)
}

func optionalFloat(value *float64) string {
	if value == nil {
		return ""
	}
	return strconv.FormatFloat(*value, 'f', -1, 64)
}

// ExtractSourceQuestionCode returns the first source_question_code from condition JSON.
func ExtractSourceQuestionCode(raw json.RawMessage) string {
	codes, err := domain.CollectConditionSourceQuestionCodes(raw)
	if err != nil || len(codes) == 0 {
		return ""
	}
	return codes[0]
}
