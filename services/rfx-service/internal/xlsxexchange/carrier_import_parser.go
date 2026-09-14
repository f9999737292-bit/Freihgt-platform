package xlsxexchange

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/google/uuid"

	"github.com/freight-platform/rfx-service/internal/domain"
	"github.com/freight-platform/rfx-service/internal/xlsxsecurity"
)

type carrierImportParser struct {
	ctx            context.Context
	issues         *issueCollector
	securityLimits xlsxsecurity.Limits
	importLimits   CarrierImportLimits
	contentType    string
	openWorkbook   openWorkbookFn
	rowChecks      uint64
	cellChecks     uint64
}

// ParseCarrierImportPreview parses CARRIER XLSX V1 bytes into a deterministic preview proposal.
func ParseCarrierImportPreview(
	ctx context.Context,
	workbookBytes []byte,
	target TargetCarrierBaseline,
) (CarrierImportPreview, error) {
	return parseCarrierImportPreview(ctx, workbookBytes, target, internalParseConfig{
		contentType:  "",
		openWorkbook: defaultOpenWorkbook,
	})
}

func parseCarrierImportPreview(
	ctx context.Context,
	workbookBytes []byte,
	target TargetCarrierBaseline,
	cfg internalParseConfig,
) (CarrierImportPreview, error) {
	p := newCarrierImportParser(ctx, cfg)
	preview := newCarrierPreviewBase(target)

	if err := p.checkContext(); err != nil {
		return preview, err
	}
	if int64(len(workbookBytes)) > p.securityLimits.MaxUploadBytes {
		p.issues.addError(carrierIssueError(
			MachineCodeFileTooLarge,
			"rfx.carrier_xlsx_import.file_too_large",
			"", "", "", 0,
			map[string]any{"max_bytes": p.securityLimits.MaxUploadBytes},
		))
		return p.finalizePreview(preview, target, CarrierImportProposal{})
	}
	if _, err := xlsxsecurity.InspectUpload(p.contentType, workbookBytes, p.securityLimits); err != nil {
		mapSecurityError(err, p.issues)
		return p.finalizePreview(preview, target, CarrierImportProposal{})
	}
	workbook, err := p.openWorkbook(workbookBytes)
	if err != nil {
		p.issues.addError(carrierIssueError(
			MachineCodeInvalidXLSXSignature,
			"rfx.carrier_xlsx_import.invalid_workbook",
			"", "", "", 0, nil,
		))
		return p.finalizePreview(preview, target, CarrierImportProposal{})
	}
	defer workbook.Close()

	if err := p.validateCarrierWorkbookStructure(workbook); err != nil {
		return preview, err
	}
	if len(p.issues.errors) > 0 || p.issues.truncated {
		return p.finalizePreview(preview, target, CarrierImportProposal{})
	}

	meta := p.parseCarrierMetadata(workbook, target)
	preview.SchemaName = meta.schemaName
	preview.SchemaVersion = meta.schemaVersion
	preview.StaleBaseline = meta.staleBaseline

	if p.shouldStop() || preview.StaleBaseline {
		return p.finalizePreview(preview, target, CarrierImportProposal{})
	}

	answers := p.parseCarrierAnswersSheet(workbook, target)
	offerLines := p.parseCarrierOfferLinesSheet(workbook, target)
	proposal := CarrierImportProposal{Answers: answers, OfferLines: offerLines}
	p.validateCarrierProposal(target, proposal)
	return p.finalizePreview(preview, target, proposal)
}

func newCarrierImportParser(ctx context.Context, cfg internalParseConfig) *carrierImportParser {
	openFn := cfg.openWorkbook
	if openFn == nil {
		openFn = defaultOpenWorkbook
	}
	return &carrierImportParser{
		ctx:            ctx,
		issues:         newIssueCollector(),
		securityLimits: productionSecurityLimits(),
		importLimits:   DefaultCarrierImportLimits(),
		contentType:    cfg.contentType,
		openWorkbook:   openFn,
	}
}

func newCarrierPreviewBase(target TargetCarrierBaseline) CarrierImportPreview {
	return CarrierImportPreview{
		SchemaName:                domain.SchemaVersionCarrierXLSXV1,
		SchemaVersion:             schemaVersionNumber,
		Mode:                      CarrierImportModeUpdateDraft,
		TargetEventID:             target.EventID,
		TargetResponseID:          target.ResponseID,
		TargetRfxVersionID:        target.RfxVersionID,
		TargetVersionNumber:       target.QuestionnaireVersionNumber,
		TargetEventRowVersion:     target.EventRowVersion,
		TargetResponseSaveVersion: target.ResponseSaveVersion,
	}
}

func (p *carrierImportParser) checkContext() error {
	return p.ctx.Err()
}

func (p *carrierImportParser) checkContextEveryRow() error {
	p.rowChecks++
	if p.rowChecks%contextCheckRowInterval == 0 {
		return p.ctx.Err()
	}
	return nil
}

func (p *carrierImportParser) checkContextEveryCell() error {
	p.cellChecks++
	if p.cellChecks%contextCheckCellInterval == 0 {
		return p.ctx.Err()
	}
	return nil
}

func (p *carrierImportParser) shouldStop() bool {
	return p.issues.stopped()
}

func (p *carrierImportParser) finalizePreview(
	preview CarrierImportPreview,
	target TargetCarrierBaseline,
	proposal CarrierImportProposal,
) (CarrierImportPreview, error) {
	if err := p.checkContext(); err != nil {
		return preview, err
	}
	errors, warnings := p.issues.sorted()
	sortCarrierIssues(errors)
	sortCarrierIssues(warnings)
	preview.Errors = errors
	preview.Warnings = warnings
	preview.ReadyToCommit = len(errors) == 0 && !preview.StaleBaseline

	propAnswers := proposedAnswerRows(proposal)
	propOffers := proposedOfferRows(proposal)
	preview.AnswersDiff = compareCarrierAnswersDiff(target.BaselineAnswers, propAnswers)
	preview.OfferLinesDiff = compareCarrierOfferLinesDiff(target.BaselineOfferLines, propOffers)
	preview.Summary = buildCarrierImportSummary(errors, warnings, preview.AnswersDiff, preview.OfferLinesDiff)

	if preview.ReadyToCommit {
		hash, err := computeCarrierCanonicalPayloadHash(preview, target, proposal)
		if err != nil {
			return preview, err
		}
		preview.CanonicalPayloadHash = hash
	}
	preview.Proposal = proposal
	return preview, nil
}

func proposedAnswerRows(proposal CarrierImportProposal) []CarrierAnswerRow {
	out := make([]CarrierAnswerRow, 0, len(proposal.Answers))
	for _, patch := range proposal.Answers {
		if patch.Delete {
			continue
		}
		out = append(out, CarrierAnswerRow{
			QuestionCode: patch.QuestionCode,
			AnswerValue:  strings.TrimSpace(string(patch.Value)),
		})
	}
	return out
}

func proposedOfferRows(proposal CarrierImportProposal) []CarrierOfferLineRow {
	out := make([]CarrierOfferLineRow, 0, len(proposal.OfferLines))
	for _, line := range proposal.OfferLines {
		if line.Delete {
			continue
		}
		out = append(out, CarrierOfferLineRow{
			LotNumber:    line.LotNumber,
			Amount:       formatOfferAmount(line.Amount),
			CurrencyCode: line.CurrencyCode,
			Comment:      line.Comment,
		})
	}
	return out
}

func computeCarrierCanonicalPayloadHash(preview CarrierImportPreview, target TargetCarrierBaseline, proposal CarrierImportProposal) (string, error) {
	raw, err := CanonicalCarrierImportPayloadJSON(preview, target, proposal)
	if err != nil {
		return "", err
	}
	return StableStoredCarrierPayloadHash(raw)
}

type parsedCarrierMetadata struct {
	schemaName    string
	schemaVersion string
	staleBaseline bool
}

func (p *carrierImportParser) parseCarrierMetadata(workbook workbookReader, target TargetCarrierBaseline) parsedCarrierMetadata {
	rows, err := workbook.GetRows(sheetMetadata)
	if err != nil {
		p.issues.addError(carrierIssueError(MachineCodeInvalidType, "rfx.carrier_xlsx_import.metadata_unreadable", sheetMetadata, "", "", 0, nil))
		return parsedCarrierMetadata{}
	}
	meta := make(map[string]string)
	for rowIdx, row := range rows {
		if p.shouldStop() {
			return parsedCarrierMetadata{}
		}
		if !rowHasAnyValue(row) {
			continue
		}
		key := trimCell(row[0])
		value := ""
		if len(row) >= 2 {
			value = row[1]
		}
		if key == "" {
			continue
		}
		if _, dup := meta[key]; dup {
			p.issues.addError(carrierIssueError(
				MachineCodeDuplicateStableCode,
				"rfx.carrier_xlsx_import.duplicate_metadata_key",
				sheetMetadata, key, key, rowIdx+1, nil,
			))
			continue
		}
		if _, allowed := carrierMetadataAllowedKeys[key]; !allowed {
			p.issues.addError(carrierIssueError(
				MachineCodeInvalidHeader,
				"rfx.carrier_xlsx_import.unknown_metadata_key",
				sheetMetadata, key, key, rowIdx+1, nil,
			))
			continue
		}
		meta[key] = value
	}

	out := parsedCarrierMetadata{
		schemaName:    trimCell(meta["schema_name"]),
		schemaVersion: trimCell(meta["schema_version"]),
	}
	if out.schemaName == "" || out.schemaVersion == "" {
		p.issues.addError(carrierIssueError(
			MachineCodeMissingRequiredValue,
			"rfx.carrier_xlsx_import.missing_schema_metadata",
			sheetMetadata, "schema_name", "", 0, nil,
		))
	}
	if out.schemaName != domain.SchemaVersionCarrierXLSXV1 || out.schemaVersion != schemaVersionNumber {
		p.issues.addError(carrierIssueError(
			MachineCodeUnsupportedSchema,
			"rfx.carrier_xlsx_import.unsupported_schema",
			sheetMetadata, "schema_name", "", 0,
			map[string]any{"schema_name": out.schemaName, "schema_version": out.schemaVersion},
		))
	}
	if status := trimCell(meta["response_status"]); status != "" && status != domain.RfxResponseStatusDraft {
		p.issues.addError(carrierIssueError(
			MachineCodeResponseNotEditable,
			"rfx.carrier_xlsx_import.response_not_editable",
			sheetMetadata, "response_status", "", 0,
			map[string]any{"value": status},
		))
	}
	if exportMode := trimCell(meta["export_mode"]); exportMode != "" && exportMode != CarrierExportModeDraftEdit {
		p.issues.addError(carrierIssueError(
			MachineCodeResponseNotEditable,
			"rfx.carrier_xlsx_import.export_mode_not_editable",
			sheetMetadata, "export_mode", "", 0,
			map[string]any{"value": exportMode},
		))
	}
	out.staleBaseline = p.verifyCarrierMetadataBaselines(meta, target)
	return out
}

func (p *carrierImportParser) verifyCarrierMetadataBaselines(meta map[string]string, target TargetCarrierBaseline) bool {
	stale := false
	check := func(key, expected string) {
		got := trimCell(meta[key])
		if got == "" || expected == "" {
			return
		}
		if got != expected {
			stale = true
		}
	}
	check("tenant_id", target.TenantID.String())
	check("rfx_event_id", target.EventID.String())
	check("rfx_response_id", target.ResponseID.String())
	check("carrier_company_id", target.CarrierCompanyID.String())
	check("rfx_version_id", target.RfxVersionID.String())
	check("available_lots_fingerprint", target.AvailableLotsFingerprint)
	check("answers_fingerprint", target.AnswersFingerprint)
	check("offer_lines_fingerprint", target.OfferLinesFingerprint)
	if saveVersion := trimCell(meta["response_save_version"]); saveVersion != "" {
		parsed, err := strconv.ParseInt(saveVersion, 10, 64)
		if err == nil && parsed != target.ResponseSaveVersion {
			stale = true
		}
	}
	if eventVersion := trimCell(meta["event_row_version"]); eventVersion != "" {
		parsed, err := strconv.Atoi(eventVersion)
		if err == nil && parsed != target.EventRowVersion {
			stale = true
		}
	}
	return stale
}

func (p *carrierImportParser) parseCarrierAnswersSheet(workbook workbookReader, target TargetCarrierBaseline) []CarrierAnswerPatch {
	rows, err := workbook.GetRows(sheetAnswers)
	if err != nil {
		p.issues.addError(carrierIssueError(MachineCodeInvalidType, "rfx.carrier_xlsx_import.sheet_unreadable", sheetAnswers, "", "", 0, nil))
		return nil
	}
	if len(rows) <= 1 {
		return nil
	}
	rt := domain.BuildQuestionnaireRuntime(&target.Questionnaire)
	seen := make(map[string]struct{})
	out := make([]CarrierAnswerPatch, 0)
	for rowIdx, row := range rows[1:] {
		if p.shouldStop() {
			return out
		}
		rowNum := rowIdx + 2
		code := ""
		value := ""
		if len(row) >= 1 {
			code = trimCell(row[0])
		}
		if len(row) >= 2 {
			value = row[1]
		}
		if code == "" && value == "" {
			continue
		}
		if code == "" {
			p.issues.addError(carrierIssueError(
				MachineCodeMissingRequiredValue,
				"rfx.carrier_xlsx_import.missing_question_code",
				sheetAnswers, "question_code", "", rowNum, nil,
			))
			continue
		}
		if _, dup := seen[code]; dup {
			p.issues.addError(carrierIssueError(
				MachineCodeDuplicateStableCode,
				"rfx.carrier_xlsx_import.duplicate_question_code",
				sheetAnswers, "question_code", code, rowNum, nil,
			))
			continue
		}
		seen[code] = struct{}{}
		question, ok := rt.QuestionsByCode[code]
		if !ok {
			p.issues.addError(carrierIssueError(
				MachineCodeDanglingReference,
				"rfx.carrier_xlsx_import.unknown_question_code",
				sheetAnswers, "question_code", code, rowNum, nil,
			))
			continue
		}
		if strings.TrimSpace(value) == "" {
			out = append(out, CarrierAnswerPatch{QuestionCode: code, QuestionID: question.ID, Delete: true})
			continue
		}
		out = append(out, CarrierAnswerPatch{
			QuestionCode: code,
			QuestionID:   question.ID,
			Value:        parseAnswerValueJSON(value),
		})
	}
	return out
}

func (p *carrierImportParser) parseCarrierOfferLinesSheet(workbook workbookReader, target TargetCarrierBaseline) []CarrierOfferLinePatch {
	rows, err := workbook.GetRows(sheetOfferLines)
	if err != nil {
		p.issues.addError(carrierIssueError(MachineCodeInvalidType, "rfx.carrier_xlsx_import.sheet_unreadable", sheetOfferLines, "", "", 0, nil))
		return nil
	}
	if len(rows) <= 1 {
		return nil
	}
	lotByNumber := make(map[string]uuid.UUID, len(target.Lots))
	for _, lot := range target.Lots {
		lotByNumber[strings.TrimSpace(lot.LotNumber)] = lot.ID
	}
	out := make([]CarrierOfferLinePatch, 0)
	for rowIdx, row := range rows[1:] {
		if p.shouldStop() {
			return out
		}
		rowNum := rowIdx + 2
		lotNumber := ""
		amountText := ""
		currency := ""
		comment := ""
		if len(row) >= 1 {
			lotNumber = trimCell(row[0])
		}
		if len(row) >= 2 {
			amountText = trimCell(row[1])
		}
		if len(row) >= 3 {
			currency = trimCell(row[2])
		}
		if len(row) >= 4 {
			comment = row[3]
		}
		if lotNumber == "" && amountText == "" && currency == "" && strings.TrimSpace(comment) == "" {
			continue
		}
		if amountText == "" && currency == "" {
			out = append(out, CarrierOfferLinePatch{LotNumber: lotNumber, Delete: true})
			continue
		}
		amount, ok := parseOfferAmount(amountText)
		if !ok {
			p.issues.addError(carrierIssueError(
				MachineCodeInvalidType,
				"rfx.carrier_xlsx_import.invalid_amount",
				sheetOfferLines, "amount", lotNumber, rowNum, nil,
			))
			continue
		}
		lotID := uuid.Nil
		if target.LotCount > 0 {
			id, found := lotByNumber[lotNumber]
			if found {
				lotID = id
			}
		}
		out = append(out, CarrierOfferLinePatch{
			LotNumber:    lotNumber,
			RfxLotID:     lotID,
			Amount:       amount,
			CurrencyCode: currency,
			Comment:      comment,
		})
	}
	return out
}

func (p *carrierImportParser) validateCarrierWorkbookStructure(workbook workbookReader) error {
	sheets := workbook.GetSheetList()
	if len(sheets) != len(carrierSheetOrder) {
		if len(sheets) > len(carrierSheetOrder) {
			for _, sheet := range sheets {
				if p.shouldStop() {
					return nil
				}
				if !isCarrierExpectedSheet(sheet) {
					p.issues.addError(carrierIssueError(
						MachineCodeUnexpectedSheet,
						"rfx.carrier_xlsx_import.unexpected_sheet",
						sheet, "", "", 0, nil,
					))
				}
			}
		}
		for _, expected := range carrierSheetOrder {
			if p.shouldStop() {
				return nil
			}
			if sheetIndexByName(sheets, expected) < 0 {
				p.issues.addError(carrierIssueError(
					MachineCodeMissingSheet,
					"rfx.carrier_xlsx_import.missing_sheet",
					expected, "", "", 0, nil,
				))
			}
		}
	}
	for idx, expected := range carrierSheetOrder {
		if p.shouldStop() {
			return nil
		}
		if idx >= len(sheets) || sheets[idx] != expected {
			p.issues.addError(carrierIssueError(
				MachineCodeInvalidHeader,
				"rfx.carrier_xlsx_import.sheet_order_mismatch",
				expected, "", "", 0,
				map[string]any{"expected_index": idx + 1},
			))
		}
	}
	presentSheets := make(map[string]struct{}, len(sheets))
	for _, sheet := range sheets {
		presentSheets[sheet] = struct{}{}
	}
	for _, sheet := range carrierSheetOrder {
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
		if vis == sheetHidden || vis == sheetVeryHidden {
			p.issues.addError(carrierIssueError(
				MachineCodeHiddenSheetDenied,
				"rfx.carrier_xlsx_import.hidden_sheet_denied",
				sheet, "", "", 0, nil,
			))
		}
	}
	totalCells := 0
	for _, sheet := range carrierSheetOrder {
		if err := p.checkContext(); err != nil {
			return err
		}
		if p.shouldStop() {
			return nil
		}
		if _, ok := presentSheets[sheet]; !ok {
			continue
		}
		if err := p.checkCarrierMergedCells(workbook, sheet); err != nil {
			return err
		}
		rows, err := workbook.GetRows(sheet)
		if err != nil {
			return fmt.Errorf("read rows %s: %w", sheet, err)
		}
		if len(rows) > p.importLimits.MaxRowsPerSheet {
			p.issues.addError(carrierIssueError(
				MachineCodeTooManyRows,
				"rfx.carrier_xlsx_import.too_many_rows",
				sheet, "", "", 0,
				map[string]any{"max_rows": p.importLimits.MaxRowsPerSheet},
			))
		}
		if err := p.checkCarrierHiddenRowsColumns(workbook, sheet, rows); err != nil {
			return err
		}
		if headers, ok := carrierDataSheetHeaders[sheet]; ok {
			p.validateCarrierHeaderRow(sheet, rows, headers)
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
					p.issues.addError(carrierIssueError(
						MachineCodeTooManyCells,
						"rfx.carrier_xlsx_import.too_many_cells",
						sheet, "", "", rowNum,
						map[string]any{"max_cells": p.importLimits.MaxTotalCells},
					))
					break
				}
				cell := cellName(colIdx+1, rowNum)
				if err := p.checkCarrierCellSecurity(workbook, sheet, cell, rowNum, columnName(colIdx+1)); err != nil {
					return err
				}
			}
		}
	}
	return nil
}

func isCarrierExpectedSheet(name string) bool {
	for _, expected := range carrierSheetOrder {
		if expected == name {
			return true
		}
	}
	return false
}

func (p *carrierImportParser) checkCarrierMergedCells(workbook workbookReader, sheet string) error {
	merges, err := workbook.GetMergeCells(sheet)
	if err != nil {
		return fmt.Errorf("merge cells %s: %w", sheet, err)
	}
	if len(merges) > 0 {
		p.issues.addError(carrierIssueError(
			MachineCodeMergedCellDenied,
			"rfx.carrier_xlsx_import.merged_cell_denied",
			sheet, "", "", 0, nil,
		))
	}
	return nil
}

func (p *carrierImportParser) checkCarrierHiddenRowsColumns(workbook workbookReader, sheet string, rows [][]string) error {
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
			p.issues.addWarning(carrierIssueWarning(
				MachineCodeHiddenContentWarning,
				"rfx.carrier_xlsx_import.hidden_row_warning",
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
				p.issues.addWarning(carrierIssueWarning(
					MachineCodeHiddenContentWarning,
					"rfx.carrier_xlsx_import.hidden_column_warning",
					sheet, colLabel, "", rowIdx+1, nil,
				))
				break
			}
		}
	}
	return nil
}

func (p *carrierImportParser) validateCarrierHeaderRow(sheet string, rows [][]string, expected []string) {
	if len(rows) == 0 {
		p.issues.addError(carrierIssueError(
			MachineCodeInvalidHeader,
			"rfx.carrier_xlsx_import.missing_header_row",
			sheet, "", "", 0, nil,
		))
		return
	}
	header := rows[0]
	competitorDenied := false
	for idx, cell := range header {
		if p.shouldStop() {
			return
		}
		name := trimCell(cell)
		if name == "" {
			continue
		}
		if isCarrierForbiddenColumn(name) {
			p.issues.addError(carrierIssueError(
				MachineCodeCompetitorColumnDenied,
				"rfx.carrier_xlsx_import.competitor_column_denied",
				sheet, name, columnName(idx+1), 1, nil,
			))
			competitorDenied = true
		}
	}
	if competitorDenied {
		return
	}
	if len(header) != len(expected) {
		p.issues.addError(carrierIssueError(
			MachineCodeInvalidHeader,
			"rfx.carrier_xlsx_import.header_column_count_mismatch",
			sheet, "", "", 1,
			map[string]any{"expected": len(expected), "actual": len(header)},
		))
	}
	for idx, want := range expected {
		if p.shouldStop() {
			return
		}
		if idx >= len(header) {
			p.issues.addError(carrierIssueError(
				MachineCodeInvalidHeader,
				"rfx.carrier_xlsx_import.missing_required_header",
				sheet, want, "", 1, nil,
			))
			continue
		}
		got := trimCell(header[idx])
		if got != want {
			p.issues.addError(carrierIssueError(
				MachineCodeInvalidHeader,
				"rfx.carrier_xlsx_import.header_order_mismatch",
				sheet, want, "", 1,
				map[string]any{"expected": want, "actual": got, "index": idx + 1},
			))
		}
	}
	for idx := len(expected); idx < len(header); idx++ {
		if p.shouldStop() {
			return
		}
		extra := trimCell(header[idx])
		if extra == "" {
			continue
		}
		if isCarrierForbiddenColumn(extra) {
			p.issues.addError(carrierIssueError(
				MachineCodeCompetitorColumnDenied,
				"rfx.carrier_xlsx_import.competitor_column_denied",
				sheet, extra, "", 1, nil,
			))
		} else {
			p.issues.addError(carrierIssueError(
				MachineCodeInvalidHeader,
				"rfx.carrier_xlsx_import.unexpected_column",
				sheet, extra, "", 1, nil,
			))
		}
	}
}

func (p *carrierImportParser) checkCarrierCellSecurity(workbook workbookReader, sheet, cell string, rowNum int, column string) error {
	formula, err := workbook.GetCellFormula(sheet, cell)
	if err != nil {
		return fmt.Errorf("read formula %s!%s: %w", sheet, cell, err)
	}
	if stringsTrimSpace(formula) != "" {
		p.issues.addError(carrierIssueError(
			MachineCodeFormulaDenied,
			"rfx.carrier_xlsx_import.formula_denied",
			sheet, column, "", rowNum, nil,
		))
		return nil
	}
	value, err := workbook.GetCellValue(sheet, cell)
	if err != nil {
		return fmt.Errorf("read cell %s!%s: %w", sheet, cell, err)
	}
	if len([]rune(value)) > p.importLimits.MaxStringLength {
		p.issues.addError(carrierIssueError(
			MachineCodeInvalidType,
			"rfx.carrier_xlsx_import.string_too_long",
			sheet, column, "", rowNum,
			map[string]any{"max_length": p.importLimits.MaxStringLength},
		))
	}
	return nil
}
