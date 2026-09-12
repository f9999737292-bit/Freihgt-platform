package xlsxexchange

import (
	"context"
	"fmt"
	"strings"

	"github.com/google/uuid"

	"github.com/freight-platform/rfx-service/internal/domain"
	"github.com/freight-platform/rfx-service/internal/xlsxsecurity"
)

var dataSheetHeaders = map[string][]string{
	sheetLots:      lotsHeaders,
	sheetSections:  sectionsHeaders,
	sheetQuestions: questionsHeaders,
	sheetOptions:   optionsHeaders,
	sheetRules:     rulesHeaders,
}

// ParseBuyerImportPreview parses BUYER XLSX V1 bytes into a deterministic preview proposal.
// Production callers cannot override security/import limits or workbook open hooks.
func ParseBuyerImportPreview(
	ctx context.Context,
	workbookBytes []byte,
	target TargetDraftBaseline,
) (BuyerImportPreview, error) {
	return parseBuyerImportPreview(ctx, workbookBytes, target, productionParseConfig())
}

func parseBuyerImportPreview(
	ctx context.Context,
	workbookBytes []byte,
	target TargetDraftBaseline,
	cfg internalParseConfig,
) (BuyerImportPreview, error) {
	p := newBuyerImportParser(ctx, cfg)
	preview := newPreviewBase(target)

	if err := p.checkContext(); err != nil {
		return preview, err
	}

	if int64(len(workbookBytes)) > p.securityLimits.MaxUploadBytes {
		p.issues.addError(issueError(
			MachineCodeFileTooLarge,
			"rfx.buyer_xlsx_import.file_too_large",
			"", "", "", 0,
			map[string]any{"max_bytes": p.securityLimits.MaxUploadBytes},
		))
		return p.finalizePreview(preview, target, BuyerImportProposal{})
	}

	if _, err := xlsxsecurity.InspectUpload(p.contentType, workbookBytes, p.securityLimits); err != nil {
		mapSecurityError(err, p.issues)
		return p.finalizePreview(preview, target, BuyerImportProposal{})
	}

	workbook, err := p.openWorkbook(workbookBytes)
	if err != nil {
		p.issues.addError(issueError(
			MachineCodeInvalidXLSXSignature,
			"rfx.buyer_xlsx_import.invalid_workbook",
			"", "", "", 0, nil,
		))
		return p.finalizePreview(preview, target, BuyerImportProposal{})
	}
	defer workbook.Close()

	if err := p.checkContext(); err != nil {
		return preview, err
	}

	if err := p.validateWorkbookStructure(workbook); err != nil {
		return preview, err
	}
	if len(p.issues.errors) > 0 || p.issues.truncated {
		return p.finalizePreview(preview, target, BuyerImportProposal{})
	}

	metadata := p.parseMetadataSheet(workbook, target)
	preview.SchemaName = metadata.schemaName
	preview.SchemaVersion = metadata.schemaVersion

	if p.shouldStop() {
		return p.finalizePreview(preview, target, BuyerImportProposal{})
	}

	lots := p.parseLotsSheet(workbook, target)
	sections := p.parseSectionsSheet(workbook)
	questions := p.parseQuestionsSheet(workbook, sections)
	options := p.parseOptionsSheet(workbook, questions)
	rules := p.parseRulesSheet(workbook, questions)

	proposal := assembleProposal(target, lots, sections, questions, options, rules)
	p.validateProposalGraph(target, proposal)
	return p.finalizePreview(preview, target, proposal)
}

func newPreviewBase(target TargetDraftBaseline) BuyerImportPreview {
	return BuyerImportPreview{
		SchemaName:            domain.SchemaVersionBuyerXLSXV1,
		SchemaVersion:         schemaVersionNumber,
		Mode:                  BuyerImportModeUpdateDraft,
		TargetEventID:         target.EventID,
		TargetDraftVersionID:  target.DraftVersionID,
		TargetVersionNumber:   target.DraftVersionNumber,
		TargetEventRowVersion: target.EventRowVersion,
		TargetDraftRowVersion: target.DraftRowVersion,
	}
}

func (p *buyerImportParser) finalizePreview(
	preview BuyerImportPreview,
	target TargetDraftBaseline,
	proposal BuyerImportProposal,
) (BuyerImportPreview, error) {
	if err := p.checkContext(); err != nil {
		return preview, err
	}

	errors, warnings := p.issues.sorted()
	preview.Errors = errors
	preview.Warnings = warnings
	preview.ReadyToCommit = len(errors) == 0

	if err := p.checkContext(); err != nil {
		return preview, err
	}

	qDiff := compareQuestionnaireDiff(target, proposal)
	lDiff := compareLotsDiff(target.Lots, proposal.Lots)
	preview.QuestionnaireDiff = qDiff
	preview.LotsDiff = lDiff
	preview.Summary = buildImportSummary(errors, warnings, qDiff, lDiff)

	if preview.ReadyToCommit {
		if err := p.checkContext(); err != nil {
			return preview, err
		}
		hash, err := computeCanonicalPayloadHash(target, proposal, qDiff, lDiff, errors, warnings)
		if err != nil {
			return preview, err
		}
		preview.CanonicalPayloadHash = hash
	}
	preview.Proposal = proposal
	return preview, nil
}

func mapSecurityError(err error, issues *issueCollector) {
	msg := err.Error()
	switch {
	case strings.Contains(msg, "max upload size"):
		issues.addError(issueError(MachineCodeFileTooLarge, "rfx.buyer_xlsx_import.file_too_large", "", "", "", 0, nil))
	case strings.Contains(msg, "invalid_zip_signature"), strings.Contains(msg, "malformed xlsx"), strings.Contains(msg, "empty xlsx"):
		issues.addError(issueError(MachineCodeInvalidXLSXSignature, "rfx.buyer_xlsx_import.invalid_xlsx_signature", "", "", "", 0, nil))
	case strings.Contains(msg, "unsupported content type"):
		issues.addError(issueError(MachineCodeInvalidMultipart, "rfx.buyer_xlsx_import.invalid_content_type", "", "", "", 0, nil))
	default:
		issues.addError(issueError(MachineCodeUnsafePackage, "rfx.buyer_xlsx_import.unsafe_package", "", "", "", 0, nil))
	}
}

func (p *buyerImportParser) validateWorkbookStructure(workbook workbookReader) error {
	sheets := workbook.GetSheetList()
	if len(sheets) != len(buyerSheetOrder) {
		if len(sheets) > len(buyerSheetOrder) {
			for _, sheet := range sheets {
				if p.shouldStop() {
					return nil
				}
				if !isExpectedSheet(sheet) {
					p.issues.addError(issueError(
						MachineCodeUnexpectedSheet,
						"rfx.buyer_xlsx_import.unexpected_sheet",
						sheet, "", "", 0, nil,
					))
				}
			}
		}
		for _, expected := range buyerSheetOrder {
			if p.shouldStop() {
				return nil
			}
			if sheetIndexByName(sheets, expected) < 0 {
				p.issues.addError(issueError(
					MachineCodeMissingSheet,
					"rfx.buyer_xlsx_import.missing_sheet",
					expected, "", "", 0, nil,
				))
			}
		}
	}
	for idx, expected := range buyerSheetOrder {
		if p.shouldStop() {
			return nil
		}
		if idx >= len(sheets) || sheets[idx] != expected {
			p.issues.addError(issueError(
				MachineCodeInvalidHeader,
				"rfx.buyer_xlsx_import.sheet_order_mismatch",
				expected, "", "", 0,
				map[string]any{"expected_index": idx + 1},
			))
		}
	}
	presentSheets := make(map[string]struct{}, len(sheets))
	for _, sheet := range sheets {
		presentSheets[sheet] = struct{}{}
	}
	for _, sheet := range buyerSheetOrder {
		if err := p.checkContext(); err != nil {
			return err
		}
		if p.shouldStop() {
			return nil
		}
		if _, ok := presentSheets[sheet]; !ok {
			continue
		}
		vis, ok, err := workbook.GetSheetVisible(sheet)
		if err != nil {
			return fmt.Errorf("sheet visibility %s: %w", sheet, err)
		}
		if !ok {
			continue
		}
		switch vis {
		case sheetHidden, sheetVeryHidden:
			p.issues.addError(issueError(
				MachineCodeHiddenSheetDenied,
				"rfx.buyer_xlsx_import.hidden_sheet_denied",
				sheet, "", "", 0, nil,
			))
		}
	}
	totalCells := 0
	for _, sheet := range buyerSheetOrder {
		if err := p.checkContext(); err != nil {
			return err
		}
		if p.shouldStop() {
			return nil
		}
		if _, ok := presentSheets[sheet]; !ok {
			continue
		}
		if err := p.checkMergedCells(workbook, sheet); err != nil {
			return err
		}
		rows, err := workbook.GetRows(sheet)
		if err != nil {
			return fmt.Errorf("read rows %s: %w", sheet, err)
		}
		if len(rows) > p.importLimits.MaxRowsPerSheet {
			p.issues.addError(issueError(
				MachineCodeTooManyRows,
				"rfx.buyer_xlsx_import.too_many_rows",
				sheet, "", "", 0,
				map[string]any{"max_rows": p.importLimits.MaxRowsPerSheet},
			))
		}
		if err := p.checkHiddenRowsColumns(workbook, sheet, rows); err != nil {
			return err
		}
		if headers, ok := dataSheetHeaders[sheet]; ok {
			p.validateHeaderRow(sheet, rows, headers)
		}
		for rowIdx, row := range rows {
			if err := p.checkContextEveryRow(); err != nil {
				return err
			}
			if p.shouldStop() {
				return nil
			}
			rowNum := rowIdx + 1
			for colIdx := range row {
				if err := p.checkContextEveryCell(); err != nil {
					return err
				}
				if p.shouldStop() {
					return nil
				}
				totalCells++
				if totalCells > p.importLimits.MaxTotalCells {
					p.issues.addError(issueError(
						MachineCodeTooManyCells,
						"rfx.buyer_xlsx_import.too_many_cells",
						sheet, "", "", rowNum,
						map[string]any{"max_cells": p.importLimits.MaxTotalCells},
					))
					break
				}
				cell := cellName(colIdx+1, rowNum)
				if err := p.checkCellSecurity(workbook, sheet, cell, rowNum, columnName(colIdx+1)); err != nil {
					return err
				}
			}
		}
	}
	return nil
}

func isExpectedSheet(name string) bool {
	for _, expected := range buyerSheetOrder {
		if expected == name {
			return true
		}
	}
	return false
}

func (p *buyerImportParser) checkMergedCells(workbook workbookReader, sheet string) error {
	merges, err := workbook.GetMergeCells(sheet)
	if err != nil {
		return fmt.Errorf("merge cells %s: %w", sheet, err)
	}
	if len(merges) > 0 {
		p.issues.addError(issueError(
			MachineCodeMergedCellDenied,
			"rfx.buyer_xlsx_import.merged_cell_denied",
			sheet, "", "", 0, nil,
		))
	}
	return nil
}

func (p *buyerImportParser) checkHiddenRowsColumns(workbook workbookReader, sheet string, rows [][]string) error {
	for rowIdx := range rows {
		if p.shouldStop() {
			return nil
		}
		rowNum := rowIdx + 1
		visible, err := workbook.GetRowVisible(sheet, rowNum)
		if err != nil {
			return fmt.Errorf("row visibility %s:%d: %w", sheet, rowNum, err)
		}
		if !visible && rowHasAnyValue(rows[rowIdx]) {
			p.issues.addWarning(issueWarning(
				MachineCodeHiddenContentWarning,
				"rfx.buyer_xlsx_import.hidden_row_warning",
				sheet, "", "", rowNum, nil,
			))
		}
	}
	if len(rows) == 0 {
		return nil
	}
	maxCols := 0
	for _, row := range rows {
		if len(row) > maxCols {
			maxCols = len(row)
		}
	}
	for col := 1; col <= maxCols; col++ {
		if p.shouldStop() {
			return nil
		}
		colLabel := columnName(col)
		visible, err := workbook.GetColVisible(sheet, colLabel)
		if err != nil {
			return fmt.Errorf("column visibility %s:%s: %w", sheet, colLabel, err)
		}
		if visible {
			continue
		}
		for rowIdx, row := range rows {
			if col-1 < len(row) && trimCell(row[col-1]) != "" {
				p.issues.addWarning(issueWarning(
					MachineCodeHiddenContentWarning,
					"rfx.buyer_xlsx_import.hidden_column_warning",
					sheet, colLabel, "", rowIdx+1, nil,
				))
				break
			}
		}
	}
	return nil
}

func (p *buyerImportParser) validateHeaderRow(sheet string, rows [][]string, expected []string) {
	if len(rows) == 0 {
		p.issues.addError(issueError(
			MachineCodeInvalidHeader,
			"rfx.buyer_xlsx_import.missing_header_row",
			sheet, "", "", 0, nil,
		))
		return
	}
	header := rows[0]
	if len(header) != len(expected) {
		p.issues.addError(issueError(
			MachineCodeInvalidHeader,
			"rfx.buyer_xlsx_import.header_column_count_mismatch",
			sheet, "", "", 1,
			map[string]any{"expected": len(expected), "actual": len(header)},
		))
	}
	seen := make(map[string]struct{})
	for idx, want := range expected {
		if p.shouldStop() {
			return
		}
		if idx >= len(header) {
			p.issues.addError(issueError(
				MachineCodeInvalidHeader,
				"rfx.buyer_xlsx_import.missing_required_header",
				sheet, want, "", 1, nil,
			))
			continue
		}
		got := trimCell(header[idx])
		if got != want {
			p.issues.addError(issueError(
				MachineCodeInvalidHeader,
				"rfx.buyer_xlsx_import.header_order_mismatch",
				sheet, want, "", 1,
				map[string]any{"expected": want, "actual": got, "index": idx + 1},
			))
		}
		if _, dup := seen[got]; dup && got != "" {
			p.issues.addError(issueError(
				MachineCodeInvalidHeader,
				"rfx.buyer_xlsx_import.duplicate_header",
				sheet, got, "", 1, nil,
			))
		}
		seen[got] = struct{}{}
	}
	for idx := len(expected); idx < len(header); idx++ {
		if p.shouldStop() {
			return
		}
		extra := trimCell(header[idx])
		if extra == "" {
			continue
		}
		p.issues.addError(issueError(
			MachineCodeCompetitorColumnDenied,
			"rfx.buyer_xlsx_import.unexpected_column",
			sheet, extra, "", 1, nil,
		))
	}
}

func (p *buyerImportParser) checkCellSecurity(
	workbook workbookReader,
	sheet, cell string,
	rowNum int,
	column string,
) error {
	formula, err := workbook.GetCellFormula(sheet, cell)
	if err != nil {
		return fmt.Errorf("read formula %s!%s: %w", sheet, cell, err)
	}
	if stringsTrimSpace(formula) != "" {
		p.issues.addError(issueError(
			MachineCodeFormulaDenied,
			"rfx.buyer_xlsx_import.formula_denied",
			sheet, column, "", rowNum, nil,
		))
		return nil
	}
	value, err := workbook.GetCellValue(sheet, cell)
	if err != nil {
		return fmt.Errorf("read cell %s!%s: %w", sheet, cell, err)
	}
	if len([]rune(value)) > p.importLimits.MaxStringLength {
		p.issues.addError(issueError(
			MachineCodeInvalidType,
			"rfx.buyer_xlsx_import.string_too_long",
			sheet, column, "", rowNum,
			map[string]any{"max_length": p.importLimits.MaxStringLength},
		))
	}
	return nil
}

type parsedMetadata struct {
	schemaName    string
	schemaVersion string
}

func (p *buyerImportParser) parseMetadataSheet(workbook workbookReader, target TargetDraftBaseline) parsedMetadata {
	rows, err := workbook.GetRows(sheetMetadata)
	if err != nil {
		p.issues.addError(issueError(MachineCodeInvalidType, "rfx.buyer_xlsx_import.metadata_unreadable", sheetMetadata, "", "", 0, nil))
		return parsedMetadata{}
	}
	meta := make(map[string]string)
	for rowIdx, row := range rows {
		if p.shouldStop() {
			return parsedMetadata{}
		}
		if !rowHasAnyValue(row) {
			continue
		}
		key := ""
		value := ""
		if len(row) >= 1 {
			key = trimCell(row[0])
		}
		if len(row) >= 2 {
			value = row[1]
		}
		if key == "" {
			continue
		}
		if _, dup := meta[key]; dup {
			p.issues.addError(issueError(
				MachineCodeDuplicateStableCode,
				"rfx.buyer_xlsx_import.duplicate_metadata_key",
				sheetMetadata, key, key, rowIdx+1, nil,
			))
			continue
		}
		if _, allowed := metadataAllowedKeys[key]; !allowed {
			p.issues.addError(issueError(
				MachineCodeInvalidHeader,
				"rfx.buyer_xlsx_import.unknown_metadata_key",
				sheetMetadata, key, key, rowIdx+1, nil,
			))
			continue
		}
		meta[key] = value
	}

	out := parsedMetadata{
		schemaName:    trimCell(meta["schema_name"]),
		schemaVersion: trimCell(meta["schema_version"]),
	}
	if out.schemaName == "" || out.schemaVersion == "" {
		p.issues.addError(issueError(
			MachineCodeMissingRequiredValue,
			"rfx.buyer_xlsx_import.missing_schema_metadata",
			sheetMetadata, "schema_name", "", 0, nil,
		))
	}
	if out.schemaName != domain.SchemaVersionBuyerXLSXV1 || out.schemaVersion != schemaVersionNumber {
		p.issues.addError(issueError(
			MachineCodeUnsupportedSchema,
			"rfx.buyer_xlsx_import.unsupported_schema",
			sheetMetadata, "schema_name", "", 0,
			map[string]any{"schema_name": out.schemaName, "schema_version": out.schemaVersion},
		))
	}
	if status := trimCell(meta["version_status"]); status != "" && status != domain.RfxVersionStatusDraft {
		p.issues.addError(issueError(
			MachineCodeInvalidType,
			"rfx.buyer_xlsx_import.invalid_version_status",
			sheetMetadata, "version_status", "", 0,
			map[string]any{"value": status},
		))
	}
	if eventID := trimCell(meta["rfx_event_id"]); eventID != "" {
		parsed, err := uuid.Parse(eventID)
		if err == nil && parsed != target.EventID {
			p.issues.addWarning(issueWarning(
				MachineCodeMetadataMismatch,
				"rfx.buyer_xlsx_import.event_id_mismatch",
				sheetMetadata, "rfx_event_id", "", 0,
				map[string]any{"workbook_event_id": eventID},
			))
		}
	}
	if versionID := trimCell(meta["rfx_version_id"]); versionID != "" {
		parsed, err := uuid.Parse(versionID)
		if err == nil && parsed != target.DraftVersionID {
			p.issues.addWarning(issueWarning(
				MachineCodeMetadataMismatch,
				"rfx.buyer_xlsx_import.version_id_mismatch",
				sheetMetadata, "rfx_version_id", "", 0,
				map[string]any{"workbook_version_id": versionID},
			))
		}
	}
	if tenantID := trimCell(meta["tenant_id"]); tenantID != "" {
		parsed, err := uuid.Parse(tenantID)
		if err == nil && parsed != target.TenantID {
			p.issues.addWarning(issueWarning(
				MachineCodeMetadataMismatch,
				"rfx.buyer_xlsx_import.tenant_id_mismatch",
				sheetMetadata, "tenant_id", "", 0, nil,
			))
		}
	}
	if channel := trimCell(meta["creation_channel"]); channel != "" {
		if err := domain.ValidateCreationChannel(channel); err != nil {
			p.issues.addError(issueError(
				MachineCodeInvalidType,
				"rfx.buyer_xlsx_import.invalid_creation_channel",
				sheetMetadata, "creation_channel", "", 0,
				map[string]any{"value": channel},
			))
		}
	}
	return out
}

func (p *buyerImportParser) parseLotsSheet(workbook workbookReader, target TargetDraftBaseline) []domain.RfxLot {
	rows, err := workbook.GetRows(sheetLots)
	if err != nil {
		return nil
	}
	if len(rows) <= 1 {
		return nil
	}
	out := make([]domain.RfxLot, 0)
	seen := make(map[string]struct{})
	for rowIdx, row := range rows[1:] {
		if err := p.checkContextEveryRow(); err != nil {
			return out
		}
		if p.shouldStop() {
			return out
		}
		rowNum := rowIdx + 2
		if !rowHasAnyValue(row) {
			continue
		}
		rowMap := rowToMap(lotsHeaders, row)
		lotNumber := trimCell(rowMap["lot_number"])
		if lotNumber == "" {
			p.issues.addError(issueError(
				MachineCodeMissingRequiredValue,
				"rfx.buyer_xlsx_import.missing_lot_number",
				sheetLots, "lot_number", "", rowNum, nil,
			))
			continue
		}
		if _, dup := seen[lotNumber]; dup {
			p.issues.addError(issueError(
				MachineCodeDuplicateStableCode,
				"rfx.buyer_xlsx_import.duplicate_lot_number",
				sheetLots, "lot_number", lotNumber, rowNum, nil,
			))
			continue
		}
		seen[lotNumber] = struct{}{}
		name := trimCell(rowMap["name"])
		if name == "" {
			p.issues.addError(issueError(
				MachineCodeMissingRequiredValue,
				"rfx.buyer_xlsx_import.missing_lot_name",
				sheetLots, "name", lotNumber, rowNum, nil,
			))
		}
		estimated, ok := parseOptionalFloat(rowMap["estimated_value"], sheetLots, "estimated_value", lotNumber, rowNum, p.issues)
		if !ok {
			continue
		}
		out = append(out, domain.RfxLot{
			TenantID:       target.TenantID,
			RfxEventID:     target.EventID,
			LotNumber:      lotNumber,
			Name:           name,
			Description:    normalizeOptionalString(rowMap["description"]),
			Category:       normalizeOptionalString(rowMap["category"]),
			EstimatedValue: estimated,
			CurrencyCode:   normalizeOptionalString(rowMap["currency_code"]),
			Status:         trimCell(rowMap["status"]),
		})
	}
	if len(out) > p.importLimits.MaxLots {
		p.issues.addError(issueError(
			MachineCodeTooManyRows,
			"rfx.buyer_xlsx_import.too_many_lots",
			sheetLots, "", "", 0,
			map[string]any{"max_lots": p.importLimits.MaxLots},
		))
	}
	sortLots(out)
	return out
}

func (p *buyerImportParser) parseSectionsSheet(workbook workbookReader) []domain.Section {
	rows, err := workbook.GetRows(sheetSections)
	if err != nil {
		return nil
	}
	out := make([]domain.Section, 0)
	seen := make(map[string]struct{})
	for rowIdx, row := range dropHeader(rows) {
		if err := p.checkContextEveryRow(); err != nil {
			return out
		}
		if p.shouldStop() {
			return out
		}
		rowNum := rowIdx + 2
		if !rowHasAnyValue(row) {
			continue
		}
		rowMap := rowToMap(sectionsHeaders, row)
		code := trimCell(rowMap["section_code"])
		if code == "" {
			p.issues.addError(issueError(
				MachineCodeMissingRequiredValue,
				"rfx.buyer_xlsx_import.missing_section_code",
				sheetSections, "section_code", "", rowNum, nil,
			))
			continue
		}
		if _, dup := seen[code]; dup {
			p.issues.addError(issueError(
				MachineCodeDuplicateStableCode,
				"rfx.buyer_xlsx_import.duplicate_section_code",
				sheetSections, "section_code", code, rowNum, nil,
			))
			continue
		}
		seen[code] = struct{}{}
		title := normalizeI18NValue(sheetSections, "title_ru", code, rowNum, rowMap["title_ru"], rowMap["title_en"], rowMap["title_zh"], p.issues)
		if title == "" {
			p.issues.addError(issueError(
				MachineCodeMissingRequiredValue,
				"rfx.buyer_xlsx_import.missing_section_title",
				sheetSections, "title_ru", code, rowNum, nil,
			))
		}
		desc := normalizeI18NValue(sheetSections, "description_ru", code, rowNum, rowMap["description_ru"], rowMap["description_en"], rowMap["description_zh"], p.issues)
		sortOrder, ok := parseRequiredInt(rowMap["sort_order"], sheetSections, "sort_order", code, rowNum, p.issues)
		if !ok {
			continue
		}
		out = append(out, domain.Section{
			ID:          stableImportUUID("section", code),
			SectionCode: code,
			Title:       title,
			Description: normalizeOptionalString(desc),
			SortOrder:   sortOrder,
		})
	}
	if len(out) > p.importLimits.MaxSections {
		p.issues.addError(issueError(MachineCodeTooManyRows, "rfx.buyer_xlsx_import.too_many_sections", sheetSections, "", "", 0, map[string]any{"max_sections": p.importLimits.MaxSections}))
	}
	return out
}

type parsedQuestion struct {
	sectionCode string
	question    domain.Question
}

func (p *buyerImportParser) parseQuestionsSheet(workbook workbookReader, sections []domain.Section) []parsedQuestion {
	sectionCodes := make(map[string]struct{}, len(sections))
	for _, section := range sections {
		sectionCodes[section.SectionCode] = struct{}{}
	}
	rows, err := workbook.GetRows(sheetQuestions)
	if err != nil {
		return nil
	}
	out := make([]parsedQuestion, 0)
	seen := make(map[string]struct{})
	for rowIdx, row := range dropHeader(rows) {
		if err := p.checkContextEveryRow(); err != nil {
			return out
		}
		if p.shouldStop() {
			return out
		}
		rowNum := rowIdx + 2
		if !rowHasAnyValue(row) {
			continue
		}
		rowMap := rowToMap(questionsHeaders, row)
		sectionCode := trimCell(rowMap["section_code"])
		questionCode := trimCell(rowMap["question_code"])
		if sectionCode == "" || questionCode == "" {
			p.issues.addError(issueError(MachineCodeMissingRequiredValue, "rfx.buyer_xlsx_import.missing_question_identity", sheetQuestions, "question_code", questionCode, rowNum, nil))
			continue
		}
		if _, ok := sectionCodes[sectionCode]; !ok {
			p.issues.addError(issueError(MachineCodeDanglingReference, "rfx.buyer_xlsx_import.dangling_section_reference", sheetQuestions, "section_code", questionCode, rowNum, map[string]any{"section_code": sectionCode}))
		}
		if _, dup := seen[questionCode]; dup {
			p.issues.addError(issueError(MachineCodeDuplicateStableCode, "rfx.buyer_xlsx_import.duplicate_question_code", sheetQuestions, "question_code", questionCode, rowNum, nil))
			continue
		}
		seen[questionCode] = struct{}{}
		label := normalizeI18NValue(sheetQuestions, "title_ru", questionCode, rowNum, rowMap["title_ru"], rowMap["title_en"], rowMap["title_zh"], p.issues)
		help := normalizeI18NValue(sheetQuestions, "description_ru", questionCode, rowNum, rowMap["description_ru"], rowMap["description_en"], rowMap["description_zh"], p.issues)
		required, ok := parseRequiredBool(rowMap["required"], sheetQuestions, "required", questionCode, rowNum, p.issues)
		if !ok {
			continue
		}
		sortOrder, ok := parseRequiredInt(rowMap["sort_order"], sheetQuestions, "sort_order", questionCode, rowNum, p.issues)
		if !ok {
			continue
		}
		validationJSON, ok := parseCanonicalJSONField(rowMap["validation_json"], sheetQuestions, "validation_json", questionCode, rowNum, p.issues)
		if !ok {
			continue
		}
		out = append(out, parsedQuestion{
			sectionCode: sectionCode,
			question: domain.Question{
				ID:                 stableImportUUID("question", questionCode),
				QuestionCode:       questionCode,
				QuestionType:       trimCell(rowMap["question_type"]),
				Label:              label,
				HelpText:           normalizeOptionalString(help),
				Required:           required,
				ValidationRuleJSON: validationJSON,
				SortOrder:          sortOrder,
			},
		})
	}
	if len(out) > p.importLimits.MaxQuestions {
		p.issues.addError(issueError(MachineCodeTooManyRows, "rfx.buyer_xlsx_import.too_many_questions", sheetQuestions, "", "", 0, map[string]any{"max_questions": p.importLimits.MaxQuestions}))
	}
	return out
}

type parsedOption struct {
	questionCode string
	option       domain.QuestionOption
}

func (p *buyerImportParser) parseOptionsSheet(workbook workbookReader, questions []parsedQuestion) []parsedOption {
	questionCodes := make(map[string]string, len(questions))
	questionTypes := make(map[string]string, len(questions))
	for _, q := range questions {
		questionCodes[q.question.QuestionCode] = q.question.QuestionType
		questionTypes[q.question.QuestionCode] = q.question.QuestionType
	}
	rows, err := workbook.GetRows(sheetOptions)
	if err != nil {
		return nil
	}
	out := make([]parsedOption, 0)
	seen := make(map[string]struct{})
	for rowIdx, row := range dropHeader(rows) {
		if err := p.checkContextEveryRow(); err != nil {
			return out
		}
		if p.shouldStop() {
			return out
		}
		rowNum := rowIdx + 2
		if !rowHasAnyValue(row) {
			continue
		}
		rowMap := rowToMap(optionsHeaders, row)
		questionCode := trimCell(rowMap["question_code"])
		optionCode := trimCell(rowMap["option_code"])
		if questionCode == "" || optionCode == "" {
			p.issues.addError(issueError(MachineCodeMissingRequiredValue, "rfx.buyer_xlsx_import.missing_option_identity", sheetOptions, "option_code", optionCode, rowNum, nil))
			continue
		}
		if _, ok := questionCodes[questionCode]; !ok {
			p.issues.addError(issueError(MachineCodeDanglingReference, "rfx.buyer_xlsx_import.dangling_question_reference", sheetOptions, "question_code", optionCode, rowNum, map[string]any{"question_code": questionCode}))
			continue
		}
		key := questionCode + "\x00" + optionCode
		if _, dup := seen[key]; dup {
			p.issues.addError(issueError(MachineCodeDuplicateStableCode, "rfx.buyer_xlsx_import.duplicate_option_code", sheetOptions, "option_code", optionCode, rowNum, nil))
			continue
		}
		seen[key] = struct{}{}
		qType := questionTypes[questionCode]
		if !domain.QuestionTypeRequiresOptions(qType) {
			p.issues.addError(issueError(MachineCodeInvalidType, "rfx.buyer_xlsx_import.options_not_allowed", sheetOptions, "option_code", optionCode, rowNum, map[string]any{"question_type": qType}))
		}
		label := normalizeI18NValue(sheetOptions, "label_ru", optionCode, rowNum, rowMap["label_ru"], rowMap["label_en"], rowMap["label_zh"], p.issues)
		sortOrder, ok := parseRequiredInt(rowMap["sort_order"], sheetOptions, "sort_order", optionCode, rowNum, p.issues)
		if !ok {
			continue
		}
		out = append(out, parsedOption{
			questionCode: questionCode,
			option: domain.QuestionOption{
				ID:         stableImportUUID("option", questionCode+":"+optionCode),
				OptionCode: optionCode,
				Label:      label,
				SortOrder:  sortOrder,
			},
		})
	}
	if len(out) > p.importLimits.MaxOptions {
		p.issues.addError(issueError(MachineCodeTooManyRows, "rfx.buyer_xlsx_import.too_many_options", sheetOptions, "", "", 0, map[string]any{"max_options": p.importLimits.MaxOptions}))
	}
	return out
}

func (p *buyerImportParser) parseRulesSheet(workbook workbookReader, questions []parsedQuestion) []domain.QuestionRule {
	questionCodes := make(map[string]uuid.UUID, len(questions))
	for _, q := range questions {
		questionCodes[q.question.QuestionCode] = q.question.ID
	}
	rows, err := workbook.GetRows(sheetRules)
	if err != nil {
		return nil
	}
	out := make([]domain.QuestionRule, 0)
	seen := make(map[string]struct{})
	for rowIdx, row := range dropHeader(rows) {
		if err := p.checkContextEveryRow(); err != nil {
			return out
		}
		if p.shouldStop() {
			return out
		}
		rowNum := rowIdx + 2
		if !rowHasAnyValue(row) {
			continue
		}
		rowMap := rowToMap(rulesHeaders, row)
		ruleCode := trimCell(rowMap["rule_code"])
		sourceCode := trimCell(rowMap["source_question_code"])
		targetCode := trimCell(rowMap["target_question_code"])
		if ruleCode == "" {
			p.issues.addError(issueError(MachineCodeMissingRequiredValue, "rfx.buyer_xlsx_import.missing_rule_code", sheetRules, "rule_code", "", rowNum, nil))
			continue
		}
		if _, dup := seen[ruleCode]; dup {
			p.issues.addError(issueError(MachineCodeDuplicateStableCode, "rfx.buyer_xlsx_import.duplicate_rule_code", sheetRules, "rule_code", ruleCode, rowNum, nil))
			continue
		}
		seen[ruleCode] = struct{}{}
		if sourceCode != "" {
			if _, ok := questionCodes[sourceCode]; !ok {
				p.issues.addError(issueError(MachineCodeDanglingReference, "rfx.buyer_xlsx_import.dangling_source_question", sheetRules, "source_question_code", ruleCode, rowNum, map[string]any{"source_question_code": sourceCode}))
			}
		}
		targetID, ok := questionCodes[targetCode]
		if targetCode == "" || !ok {
			p.issues.addError(issueError(MachineCodeDanglingReference, "rfx.buyer_xlsx_import.dangling_target_question", sheetRules, "target_question_code", ruleCode, rowNum, map[string]any{"target_question_code": targetCode}))
			continue
		}
		if sourceCode == targetCode {
			p.issues.addError(issueError(MachineCodeSelfTargetRule, "rfx.buyer_xlsx_import.self_target_rule", sheetRules, "target_question_code", ruleCode, rowNum, nil))
		}
		conditionJSON, ok := parseCanonicalJSONField(rowMap["condition"], sheetRules, "condition", ruleCode, rowNum, p.issues)
		if !ok {
			continue
		}
		sortOrder, ok := parseRequiredInt(rowMap["sort_order"], sheetRules, "sort_order", ruleCode, rowNum, p.issues)
		if !ok {
			continue
		}
		out = append(out, domain.QuestionRule{
			ID:               stableImportUUID("rule", ruleCode),
			RuleCode:         ruleCode,
			Action:           trimCell(rowMap["action"]),
			ConditionJSON:    conditionJSON,
			TargetQuestionID: &targetID,
			SortOrder:        sortOrder,
		})
	}
	if len(out) > p.importLimits.MaxRules {
		p.issues.addError(issueError(MachineCodeTooManyRows, "rfx.buyer_xlsx_import.too_many_rules", sheetRules, "", "", 0, map[string]any{"max_rules": p.importLimits.MaxRules}))
	}
	return out
}

func assembleProposal(
	target TargetDraftBaseline,
	lots []domain.RfxLot,
	sections []domain.Section,
	questions []parsedQuestion,
	options []parsedOption,
	rules []domain.QuestionRule,
) BuyerImportProposal {
	sectionIndex := make(map[string]int)
	sectionWithQuestions := make([]domain.SectionWithQuestions, 0, len(sections))
	for _, section := range sections {
		sectionIndex[section.SectionCode] = len(sectionWithQuestions)
		sectionWithQuestions = append(sectionWithQuestions, domain.SectionWithQuestions{
			Section:   section,
			Questions: nil,
		})
	}
	for _, item := range questions {
		idx, ok := sectionIndex[item.sectionCode]
		if !ok {
			continue
		}
		sectionWithQuestions[idx].Questions = append(sectionWithQuestions[idx].Questions, item.question)
	}
	for _, item := range options {
		for sIdx, swq := range sectionWithQuestions {
			for qIdx, q := range swq.Questions {
				if q.QuestionCode != item.questionCode {
					continue
				}
				sectionWithQuestions[sIdx].Questions[qIdx].Options = append(sectionWithQuestions[sIdx].Questions[qIdx].Options, item.option)
			}
		}
	}
	sortSections(sectionWithQuestions)
	sortRules(rules)
	sortLots(lots)
	return BuyerImportProposal{
		Lots: lots,
		Questionnaire: domain.QuestionnaireDefinition{
			EventID:              target.EventID,
			RfxVersionID:         target.DraftVersionID,
			VersionNumber:        target.DraftVersionNumber,
			QuestionnaireEnabled: target.Questionnaire.QuestionnaireEnabled,
			VersionStatus:        domain.RfxVersionStatusDraft,
			Sections:             sectionWithQuestions,
			Rules:                rules,
		},
	}
}

func rowToMap(headers []string, row []string) map[string]string {
	out := make(map[string]string, len(headers))
	for idx, header := range headers {
		if idx < len(row) {
			out[header] = row[idx]
		} else {
			out[header] = ""
		}
	}
	return out
}

func dropHeader(rows [][]string) [][]string {
	if len(rows) <= 1 {
		return nil
	}
	return rows[1:]
}
