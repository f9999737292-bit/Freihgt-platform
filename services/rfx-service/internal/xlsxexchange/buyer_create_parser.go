package xlsxexchange

import (
	"context"
	"strings"

	"github.com/google/uuid"

	"github.com/freight-platform/rfx-service/internal/domain"
	"github.com/freight-platform/rfx-service/internal/xlsxsecurity"
)

// BuyerCreatePreview is the CREATE_NEW_DRAFT parser output. It does not bind an existing event.
type BuyerCreatePreview struct {
	SchemaName    string
	SchemaVersion string
	Mode          string
	Proposal      BuyerImportProposal
	ReadyToCommit bool
	Summary       BuyerImportSummary
	Errors        []BuyerImportIssue
	Warnings      []BuyerImportIssue
	ChangeCounts  BuyerCreateChangeCounts
}

// BuyerCreateChangeCounts reports additions versus an empty event.
type BuyerCreateChangeCounts struct {
	Lots      entityCreateCounts `json:"lots"`
	Sections  entityCreateCounts `json:"sections"`
	Questions entityCreateCounts `json:"questions"`
	Options   entityCreateCounts `json:"options"`
	Rules     entityCreateCounts `json:"rules"`
}

type entityCreateCounts struct {
	Added   int `json:"added"`
	Updated int `json:"updated"`
	Deleted int `json:"deleted"`
}

var createUntrustedMetadataKeys = []string{
	"tenant_id",
	"rfx_event_id",
	"rfx_version_id",
	"version_number",
	"version_status",
	"event_row_version",
	"version_row_version",
	"creation_channel",
	"source_template_version_id",
	"source_template_version_number",
	"exported_at_utc",
}

// ParseBuyerCreatePreview parses a V1 buyer workbook for CREATE_NEW_DRAFT without an existing baseline.
func ParseBuyerCreatePreview(ctx context.Context, workbookBytes []byte) (BuyerCreatePreview, error) {
	return parseBuyerCreatePreview(ctx, workbookBytes, productionParseConfig())
}

func parseBuyerCreatePreview(
	ctx context.Context,
	workbookBytes []byte,
	cfg internalParseConfig,
) (BuyerCreatePreview, error) {
	p := newBuyerImportParser(ctx, cfg)
	preview := BuyerCreatePreview{
		SchemaName:    domain.SchemaVersionBuyerXLSXV1,
		SchemaVersion: schemaVersionNumber,
		Mode:          BuyerImportModeCreateNewDraft,
	}
	emptyTarget := createParserValidationTarget()

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
		return finalizeCreatePreview(p, preview, BuyerImportProposal{})
	}
	if _, err := xlsxsecurity.InspectUpload(p.contentType, workbookBytes, p.securityLimits); err != nil {
		mapSecurityError(err, p.issues)
		return finalizeCreatePreview(p, preview, BuyerImportProposal{})
	}
	workbook, err := p.openWorkbook(workbookBytes)
	if err != nil {
		p.issues.addError(issueError(
			MachineCodeInvalidXLSXSignature,
			"rfx.buyer_xlsx_import.invalid_workbook",
			"", "", "", 0, nil,
		))
		return finalizeCreatePreview(p, preview, BuyerImportProposal{})
	}
	defer workbook.Close()

	if err := p.checkContext(); err != nil {
		return preview, err
	}
	if err := p.validateWorkbookStructure(workbook); err != nil {
		return preview, err
	}
	if len(p.issues.errors) > 0 || p.issues.truncated {
		return finalizeCreatePreview(p, preview, BuyerImportProposal{})
	}

	metadata := p.parseCreateMetadataSheet(workbook)
	preview.SchemaName = metadata.schemaName
	preview.SchemaVersion = metadata.schemaVersion
	if p.shouldStop() {
		return finalizeCreatePreview(p, preview, BuyerImportProposal{})
	}

	lots := p.parseLotsSheet(workbook, emptyTarget)
	for i := range lots {
		lots[i].TenantID = emptyTarget.TenantID
		lots[i].RfxEventID = emptyTarget.EventID
	}
	sections := p.parseSectionsSheet(workbook)
	questions := p.parseQuestionsSheet(workbook, sections)
	options := p.parseOptionsSheet(workbook, questions)
	rules := p.parseRulesSheet(workbook, questions)
	proposal := assembleProposal(emptyTarget, lots, sections, questions, options, rules)
	p.validateProposalGraph(emptyTarget, proposal)
	return finalizeCreatePreview(p, preview, proposal)
}

func finalizeCreatePreview(
	p *buyerImportParser,
	preview BuyerCreatePreview,
	proposal BuyerImportProposal,
) (BuyerCreatePreview, error) {
	if err := p.checkContext(); err != nil {
		return preview, err
	}
	errors, warnings := p.issues.sorted()
	preview.Errors = errors
	preview.Warnings = warnings
	preview.ReadyToCommit = len(errors) == 0
	preview.Proposal = proposal
	preview.ChangeCounts = BuyerCreateChangeCounts{
		Lots:      entityCreateCounts{Added: len(proposal.Lots)},
		Sections:  entityCreateCounts{Added: len(proposal.Questionnaire.Sections)},
		Questions: entityCreateCounts{Added: countCreateQuestions(proposal)},
		Options:   entityCreateCounts{Added: countCreateOptions(proposal)},
		Rules:     entityCreateCounts{Added: len(proposal.Questionnaire.Rules)},
	}
	preview.Summary = BuyerImportSummary{
		Errors:         len(errors),
		Warnings:       len(warnings),
		LotsAdded:      preview.ChangeCounts.Lots.Added,
		SectionsAdded:  preview.ChangeCounts.Sections.Added,
		QuestionsAdded: preview.ChangeCounts.Questions.Added,
	}
	return preview, nil
}

func countCreateQuestions(proposal BuyerImportProposal) int {
	total := 0
	for _, swq := range proposal.Questionnaire.Sections {
		total += len(swq.Questions)
	}
	return total
}

func countCreateOptions(proposal BuyerImportProposal) int {
	total := 0
	for _, swq := range proposal.Questionnaire.Sections {
		for _, q := range swq.Questions {
			total += len(q.Options)
		}
	}
	return total
}

func (p *buyerImportParser) parseCreateMetadataSheet(workbook workbookReader) parsedMetadata {
	rows, err := workbook.GetRows(sheetMetadata)
	if err != nil {
		p.issues.addError(issueError(MachineCodeInvalidType, "rfx.buyer_xlsx_import.metadata_unreadable", sheetMetadata, "", "", 0, nil))
		return parsedMetadata{}
	}
	meta := make(map[string]string)
	for rowIdx, row := range rows {
		if p.shouldStop() {
			break
		}
		if !rowHasAnyValue(row) {
			continue
		}
		key := ""
		value := ""
		if len(row) >= 1 {
			key = strings.TrimSpace(row[0])
		}
		if len(row) >= 2 {
			value = strings.TrimSpace(row[1])
		}
		if key == "" {
			continue
		}
		if _, ok := metadataAllowedKeys[key]; !ok {
			p.issues.addError(issueError(
				MachineCodeInvalidHeader,
				"rfx.buyer_xlsx_import.unexpected_metadata_key",
				sheetMetadata, key, key, rowIdx+2, nil,
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
	for _, key := range createUntrustedMetadataKeys {
		if trimCell(meta[key]) == "" {
			continue
		}
		p.issues.addWarning(issueWarning(
			MachineCodeMetadataMismatch,
			"rfx.buyer_xlsx_create.untrusted_metadata_ignored",
			sheetMetadata, key, "", 0,
			map[string]any{"key": key},
		))
	}
	return out
}

func createParserValidationTarget() TargetDraftBaseline {
	placeholder := uuid.MustParse("aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa")
	return TargetDraftBaseline{TenantID: placeholder, EventID: placeholder}
}
