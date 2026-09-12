package service

import (
	"context"
	"encoding/json"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/freight-platform/rfx-service/internal/domain"
	apperrors "github.com/freight-platform/rfx-service/internal/platform/errors"
	"github.com/freight-platform/rfx-service/internal/xlsxexchange"
)

const importAnalysisTTL = 24 * time.Hour

type previewTransactionRunner interface {
	Run(ctx context.Context, fn func(ctx context.Context, tx pgx.Tx) error) error
}


// BuyerImportPreviewResponse is the structured preview envelope for HTTP 200/422.
type BuyerImportPreviewResponse struct {
	SchemaName            string                          `json:"schema_name"`
	SchemaVersion         string                          `json:"schema_version"`
	Mode                  string                          `json:"mode"`
	TargetEventID         uuid.UUID                       `json:"target_event_id"`
	TargetDraftVersionID  uuid.UUID                       `json:"target_draft_version_id"`
	TargetVersionNumber   int                             `json:"target_version_number"`
	TargetEventRowVersion int                             `json:"target_event_row_version"`
	TargetDraftRowVersion int                             `json:"target_draft_row_version"`
	CanonicalPayloadHash  string                          `json:"canonical_payload_hash,omitempty"`
	AnalysisID            *uuid.UUID                      `json:"analysis_id,omitempty"`
	ExpiresAt             *time.Time                      `json:"expires_at,omitempty"`
	ReadyToCommit         bool                            `json:"ready_to_commit"`
	Summary               xlsxexchange.BuyerImportSummary `json:"summary"`
	QuestionnaireDiff     domain.CompareVersionsResult    `json:"questionnaire_diff"`
	LotsDiff              xlsxexchange.LotsCompareResult  `json:"lots_diff"`
	Errors                []xlsxexchange.BuyerImportIssue `json:"errors"`
	Warnings              []xlsxexchange.BuyerImportIssue `json:"warnings"`
}

// PreviewBuyerImportWorkbook parses workbook bytes against server baseline and optionally persists analysis.
func (s *ExcelExchangeService) PreviewBuyerImportWorkbook(
	ctx context.Context,
	actor domain.ActorContext,
	eventID uuid.UUID,
	workbookBytes []byte,
) (*BuyerImportPreviewResponse, error) {
	event, err := s.authorizeBuyerManage(ctx, actor, eventID)
	if err != nil {
		return nil, err
	}

	version, err := s.qRepo.GetActiveDraftVersion(ctx, actor.TenantID, eventID)
	if err != nil {
		return nil, err
	}

	sections, err := s.qRepo.LoadQuestionnaireTree(ctx, version.ID, actor.TenantID)
	if err != nil {
		return nil, err
	}
	rules, err := s.qRepo.ListRulesByVersion(ctx, version.ID, actor.TenantID)
	if err != nil {
		return nil, err
	}
	lots, err := s.rfxRepo.ListLotsByEvent(ctx, eventID, actor.TenantID)
	if err != nil {
		return nil, err
	}

	target := buildTargetDraftBaseline(event, version, sections, rules, lots)
	preview, err := xlsxexchange.ParseBuyerImportPreview(ctx, workbookBytes, target)
	if err != nil {
		return nil, apperrors.Internal("failed to parse buyer import preview", err)
	}

	response := toBuyerImportPreviewResponse(preview)
	if !preview.ReadyToCommit {
		return response, nil
	}

	if s.importAnalysisRepo == nil || s.txRunner == nil {
		return nil, apperrors.Internal("import analysis persistence is not configured", nil)
	}

	createdAt := s.nowFn().UTC()
	expiresAt := createdAt.Add(importAnalysisTTL)
	payloadJSON, err := xlsxexchange.CanonicalImportPayloadJSON(preview, target, preview.Proposal)
	if err != nil {
		return nil, apperrors.Internal("failed to marshal canonical import payload", err)
	}
	summaryJSON, err := json.Marshal(preview.Summary)
	if err != nil {
		return nil, apperrors.Internal("failed to marshal validation summary", err)
	}

	targetID := event.ID
	targetVersion := version.VersionNumber
	analysisInput := domain.ImportAnalysis{
		TenantID:             actor.TenantID,
		ActorID:              actor.UserID,
		ActorCompanyID:       event.OwnerCompanyID,
		WorkbookType:         domain.WorkbookTypeBuyerTender,
		SchemaVersion:        domain.SchemaVersionBuyerXLSXV1,
		TargetType:           domain.ImportTargetTypeDraftEvent,
		TargetID:             &targetID,
		TargetVersion:        &targetVersion,
		CanonicalPayloadJSON: payloadJSON,
		CanonicalHash:        preview.CanonicalPayloadHash,
		ValidationSummary:    summaryJSON,
		ExpiresAt:            expiresAt,
	}

	var persisted *domain.ImportAnalysis
	if err := s.txRunner.Run(ctx, func(ctx context.Context, tx pgx.Tx) error {
		repo := s.importAnalysisRepo.WithTx(tx)
		created, createErr := repo.CreatePreview(ctx, analysisInput)
		if createErr != nil {
			return createErr
		}
		persisted = created
		return nil
	}); err != nil {
		return nil, apperrors.Internal("failed to persist import analysis", err)
	}

	response.AnalysisID = &persisted.ID
	response.ExpiresAt = &expiresAt
	return response, nil
}

func buildTargetDraftBaseline(
	event *domain.RfxEvent,
	version *domain.RfxVersion,
	sections []domain.SectionWithQuestions,
	rules []domain.QuestionRule,
	lots []domain.RfxLot,
) xlsxexchange.TargetDraftBaseline {
	return xlsxexchange.TargetDraftBaseline{
		TenantID:           event.TenantID,
		EventID:            event.ID,
		DraftVersionID:     version.ID,
		DraftVersionNumber: version.VersionNumber,
		EventRowVersion:    event.Version,
		DraftRowVersion:    version.Version,
		Questionnaire: domain.QuestionnaireDefinition{
			EventID:              event.ID,
			RfxVersionID:         version.ID,
			VersionNumber:        version.VersionNumber,
			QuestionnaireEnabled: true,
			VersionStatus:        version.Status,
			Sections:             sections,
			Rules:                rules,
		},
		Lots: lots,
	}
}

func toBuyerImportPreviewResponse(preview xlsxexchange.BuyerImportPreview) *BuyerImportPreviewResponse {
	errors := preview.Errors
	if errors == nil {
		errors = []xlsxexchange.BuyerImportIssue{}
	}
	warnings := preview.Warnings
	if warnings == nil {
		warnings = []xlsxexchange.BuyerImportIssue{}
	}
	return &BuyerImportPreviewResponse{
		SchemaName:            preview.SchemaName,
		SchemaVersion:         preview.SchemaVersion,
		Mode:                  preview.Mode,
		TargetEventID:         preview.TargetEventID,
		TargetDraftVersionID:  preview.TargetDraftVersionID,
		TargetVersionNumber:   preview.TargetVersionNumber,
		TargetEventRowVersion: preview.TargetEventRowVersion,
		TargetDraftRowVersion: preview.TargetDraftRowVersion,
		CanonicalPayloadHash:  preview.CanonicalPayloadHash,
		ReadyToCommit:         preview.ReadyToCommit,
		Summary:               preview.Summary,
		QuestionnaireDiff:     preview.QuestionnaireDiff,
		LotsDiff:              preview.LotsDiff,
		Errors:                errors,
		Warnings:              warnings,
	}
}

// StructuralPreviewAppError maps the first structural parser issue to a safe HTTP 400 envelope.
func StructuralPreviewAppError(first xlsxexchange.BuyerImportIssue) *apperrors.AppError {
	details := map[string]any{
		"machine_code": first.MachineCode,
		"message_key":  first.MessageKey,
	}
	if first.Sheet != "" {
		details["sheet"] = first.Sheet
	}
	if first.Row > 0 {
		details["row"] = first.Row
	}
	if first.Column != "" {
		details["column"] = first.Column
	}
	return apperrors.Validation("buyer xlsx import preview failed", details)
}
