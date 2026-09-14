package xlsxexchange

import (
	"bytes"
	"encoding/json"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/xuri/excelize/v2"

	"github.com/freight-platform/rfx-service/internal/domain"
	"github.com/freight-platform/rfx-service/internal/xlsxsecurity"
)

const (
	sheetAnswers    = "Answers"
	sheetOfferLines = "OfferLines"
)

var carrierSheetOrder = []string{
	sheetInstructions,
	sheetMetadata,
	sheetLots,
	sheetQuestions,
	sheetOptions,
	sheetRules,
	sheetAnswers,
	sheetOfferLines,
}

var carrierInstructionRows = [][3]string{
	{
		"Схема: BINTRANS_RFX_CARRIER_XLSX_V1",
		"Schema: BINTRANS_RFX_CARRIER_XLSX_V1",
		"架构: BINTRANS_RFX_CARRIER_XLSX_V1",
	},
	{
		"Версия схемы: 1",
		"Schema version: 1",
		"架构版本: 1",
	},
	{
		"Назначение: экспорт собственного ответа перевозчика для офлайн-редактирования.",
		"Purpose: export own carrier response for offline editing.",
		"用途: 导出承运人自己的响应以供离线编辑。",
	},
	{
		"Экспорт является снимком собственного ответа; импорт не выполняет submit.",
		"Export is a snapshot of your own response; import does not submit.",
		"导出是您自己响应的快照；导入不会提交。",
	},
	{
		"Не добавляйте формулы, макросы или внешние ссылки.",
		"Do not add formulas, macros, or external links.",
		"请勿添加公式、宏或外部链接。",
	},
}

var submittedReadonlyBannerRows = [][3]string{
	{
		"РЕЖИМ ТОЛЬКО ЧТЕНИЕ: ответ SUBMITTED; импорт/commit запрещён.",
		"READ-ONLY MODE: response is SUBMITTED; import/commit is disabled.",
		"只读模式：响应已 SUBMITTED；禁止导入/提交。",
	},
}

// GenerateCarrierResponseWorkbook renders snapshot into a secure CARRIER XLSX V1 workbook.
func GenerateCarrierResponseWorkbook(snapshot CarrierResponseSnapshot) ([]byte, error) {
	f := excelize.NewFile()
	defer func() {
		_ = f.Close()
	}()

	if err := f.SetSheetName("Sheet1", sheetInstructions); err != nil {
		return nil, fmt.Errorf("rename instructions sheet: %w", err)
	}
	for _, name := range carrierSheetOrder[1:] {
		if _, err := f.NewSheet(name); err != nil {
			return nil, fmt.Errorf("create sheet %s: %w", name, err)
		}
	}
	cw, err := newCellWriter(f)
	if err != nil {
		return nil, err
	}

	if err := writeCarrierInstructionsSheet(cw, snapshot.Metadata.ExportMode); err != nil {
		return nil, err
	}
	if err := writeCarrierMetadataSheet(cw, snapshot.Metadata); err != nil {
		return nil, err
	}
	if err := writeCarrierLotsSheet(cw, snapshot.Lots); err != nil {
		return nil, err
	}
	if err := writeCarrierQuestionsSheet(cw, snapshot.Questions); err != nil {
		return nil, err
	}
	if err := writeCarrierOptionsSheet(cw, snapshot.Options); err != nil {
		return nil, err
	}
	if err := writeCarrierRulesSheet(cw, snapshot.Rules); err != nil {
		return nil, err
	}
	if err := writeCarrierAnswersSheet(cw, snapshot.Answers); err != nil {
		return nil, err
	}
	if err := writeCarrierOfferLinesSheet(cw, snapshot.OfferLines); err != nil {
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

func writeCarrierInstructionsSheet(cw *cellWriter, exportMode string) error {
	rows := carrierInstructionRows
	if exportMode == CarrierExportModeSubmittedReadonly {
		rows = append(append([][3]string(nil), carrierInstructionRows...), submittedReadonlyBannerRows...)
	}
	for rowIdx, cols := range rows {
		row := rowIdx + 1
		for colIdx, value := range cols {
			cell, err := excelize.CoordinatesToCellName(colIdx+1, row)
			if err != nil {
				return err
			}
			if err := cw.setTextCell(sheetInstructions, cell, value); err != nil {
				return err
			}
		}
	}
	return nil
}

func writeCarrierMetadataSheet(cw *cellWriter, md CarrierResponseMetadata) error {
	rows := [][2]string{
		{"schema_name", domain.SchemaVersionCarrierXLSXV1},
		{"schema_version", schemaVersionNumber},
		{"exported_at_utc", md.ExportedAtUTC.UTC().Format(time.RFC3339)},
		{"tenant_id", md.TenantID.String()},
		{"rfx_event_id", md.RfxEventID.String()},
		{"rfx_response_id", md.RfxResponseID.String()},
		{"carrier_company_id", md.CarrierCompanyID.String()},
		{"rfx_version_id", md.RfxVersionID.String()},
		{"questionnaire_version_number", strconv.Itoa(md.QuestionnaireVersionNumber)},
		{"response_save_version", strconv.FormatInt(md.ResponseSaveVersion, 10)},
		{"response_status", md.ResponseStatus},
		{"event_row_version", strconv.Itoa(md.EventRowVersion)},
		{"available_lots_fingerprint", md.AvailableLotsFingerprint},
		{"answers_fingerprint", md.AnswersFingerprint},
		{"offer_lines_fingerprint", md.OfferLinesFingerprint},
		{"export_mode", md.ExportMode},
	}
	for rowIdx, row := range rows {
		rowNum := rowIdx + 1
		if err := cw.setTextCell(sheetMetadata, fmt.Sprintf("A%d", rowNum), row[0]); err != nil {
			return err
		}
		if err := cw.setTextCell(sheetMetadata, fmt.Sprintf("B%d", rowNum), row[1]); err != nil {
			return err
		}
	}
	return nil
}

func writeCarrierLotsSheet(cw *cellWriter, lots []domain.RfxLot) error {
	headers := []string{"lot_number", "name", "description", "category", "currency_code", "status"}
	if err := writeHeaderRow(cw, sheetLots, headers); err != nil {
		return err
	}
	sortedLots := append([]domain.RfxLot(nil), lots...)
	sortLots(sortedLots)
	for rowIdx, lot := range sortedLots {
		rowNum := rowIdx + 2
		values := []string{
			lot.LotNumber,
			lot.Name,
			optionalString(lot.Description),
			optionalString(lot.Category),
			optionalString(lot.CurrencyCode),
			lot.Status,
		}
		if err := writeDataRow(cw, sheetLots, rowNum, values); err != nil {
			return err
		}
	}
	return nil
}

func writeCarrierQuestionsSheet(cw *cellWriter, questions []CarrierQuestion) error {
	headers := []string{
		"section_code", "question_code", "question_type",
		"title_ru", "title_en", "title_zh",
		"required", "sort_order", "validation_json",
	}
	if err := writeHeaderRow(cw, sheetQuestions, headers); err != nil {
		return err
	}
	sortedQuestions := append([]CarrierQuestion(nil), questions...)
	sort.SliceStable(sortedQuestions, func(i, j int) bool {
		if sortedQuestions[i].Question.SortOrder != sortedQuestions[j].Question.SortOrder {
			return sortedQuestions[i].Question.SortOrder < sortedQuestions[j].Question.SortOrder
		}
		return sortedQuestions[i].Question.QuestionCode < sortedQuestions[j].Question.QuestionCode
	})
	for rowIdx, item := range sortedQuestions {
		rowNum := rowIdx + 2
		q := item.Question
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
			required,
			strconv.Itoa(q.SortOrder),
			validationJSON,
		}
		if err := writeDataRow(cw, sheetQuestions, rowNum, values); err != nil {
			return err
		}
	}
	return nil
}

func writeCarrierOptionsSheet(cw *cellWriter, options []CarrierOption) error {
	headers := []string{"question_code", "option_code", "label_ru", "label_en", "label_zh", "sort_order"}
	if err := writeHeaderRow(cw, sheetOptions, headers); err != nil {
		return err
	}
	sortedOptions := append([]CarrierOption(nil), options...)
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
		if err := writeDataRow(cw, sheetOptions, rowNum, values); err != nil {
			return err
		}
	}
	return nil
}

func writeCarrierRulesSheet(cw *cellWriter, rules []CarrierRule) error {
	headers := []string{"rule_code", "source_question_code", "condition", "target_question_code", "action", "sort_order"}
	if err := writeHeaderRow(cw, sheetRules, headers); err != nil {
		return err
	}
	sortedRules := append([]CarrierRule(nil), rules...)
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
		if err := writeDataRow(cw, sheetRules, rowNum, values); err != nil {
			return err
		}
	}
	return nil
}

func writeCarrierAnswersSheet(cw *cellWriter, answers []CarrierAnswerRow) error {
	headers := []string{"question_code", "answer_value"}
	if err := writeHeaderRow(cw, sheetAnswers, headers); err != nil {
		return err
	}
	sortedAnswers := append([]CarrierAnswerRow(nil), answers...)
	sort.Slice(sortedAnswers, func(i, j int) bool {
		return sortedAnswers[i].QuestionCode < sortedAnswers[j].QuestionCode
	})
	for rowIdx, answer := range sortedAnswers {
		rowNum := rowIdx + 2
		if err := writeDataRow(cw, sheetAnswers, rowNum, []string{answer.QuestionCode, answer.AnswerValue}); err != nil {
			return err
		}
	}
	return nil
}

func writeCarrierOfferLinesSheet(cw *cellWriter, lines []CarrierOfferLineRow) error {
	headers := []string{"lot_number", "amount", "currency_code", "comment"}
	if err := writeHeaderRow(cw, sheetOfferLines, headers); err != nil {
		return err
	}
	sortedLines := append([]CarrierOfferLineRow(nil), lines...)
	sort.Slice(sortedLines, func(i, j int) bool {
		if sortedLines[i].LotNumber != sortedLines[j].LotNumber {
			return sortedLines[i].LotNumber < sortedLines[j].LotNumber
		}
		return sortedLines[i].Amount < sortedLines[j].Amount
	})
	for rowIdx, line := range sortedLines {
		rowNum := rowIdx + 2
		values := []string{line.LotNumber, line.Amount, line.CurrencyCode, line.Comment}
		if err := writeDataRow(cw, sheetOfferLines, rowNum, values); err != nil {
			return err
		}
	}
	return nil
}

// FormatCarrierAnswerValue renders persisted answer JSON as workbook text.
func FormatCarrierAnswerValue(raw json.RawMessage) string {
	trimmed := strings.TrimSpace(string(raw))
	if trimmed == "" || trimmed == "null" {
		return ""
	}
	var value any
	if err := json.Unmarshal(raw, &value); err != nil {
		return trimmed
	}
	switch typed := value.(type) {
	case string:
		return typed
	case float64:
		return strconv.FormatFloat(typed, 'f', -1, 64)
	case bool:
		return strconv.FormatBool(typed)
	default:
		out, err := marshalCanonical(value)
		if err != nil {
			return trimmed
		}
		return string(out)
	}
}
