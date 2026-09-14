package service

import (
	"context"
	"encoding/json"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/freight-platform/rfx-service/internal/domain"
	apperrors "github.com/freight-platform/rfx-service/internal/platform/errors"
	"github.com/freight-platform/rfx-service/internal/xlsxexchange"
)

// CarrierImportPreviewResponse is the structured preview envelope for HTTP 200/422.
type CarrierImportPreviewResponse struct {
	SchemaName                string                             `json:"schema_name"`
	SchemaVersion             string                             `json:"schema_version"`
	Mode                      string                             `json:"mode"`
	TargetEventID             uuid.UUID                          `json:"target_event_id"`
	TargetResponseID          uuid.UUID                          `json:"target_response_id"`
	TargetRfxVersionID        uuid.UUID                          `json:"target_rfx_version_id"`
	TargetVersionNumber       int                                `json:"target_version_number"`
	TargetEventRowVersion     int                                `json:"target_event_row_version"`
	TargetResponseSaveVersion int64                              `json:"target_response_save_version"`
	CanonicalPayloadHash      string                             `json:"canonical_payload_hash,omitempty"`
	AnalysisID                *uuid.UUID                         `json:"analysis_id,omitempty"`
	ExpiresAt                 *time.Time                         `json:"expires_at,omitempty"`
	ReadyToCommit             bool                               `json:"ready_to_commit"`
	Summary                   xlsxexchange.CarrierImportSummary  `json:"summary"`
	AnswersDiff               xlsxexchange.CarrierAnswersDiff    `json:"answers_diff"`
	OfferLinesDiff            xlsxexchange.CarrierOfferLinesDiff `json:"offer_lines_diff"`
	Errors                    []xlsxexchange.BuyerImportIssue    `json:"errors"`
	Warnings                  []xlsxexchange.BuyerImportIssue    `json:"warnings"`
}

// PreviewCarrierImportWorkbook parses carrier workbook bytes against server baseline and optionally persists analysis.
func (s *ExcelExchangeService) PreviewCarrierImportWorkbook(
	ctx context.Context,
	actor domain.ActorContext,
	eventID uuid.UUID,
	responseID uuid.UUID,
	workbookBytes []byte,
) (*CarrierImportPreviewResponse, error) {
	event, response, err := s.authorizeCarrierResponsePreview(ctx, actor, eventID, responseID)
	if err != nil {
		return nil, err
	}

	version, err := s.qRepo.GetVersionByID(ctx, *response.RfxVersionID, actor.TenantID)
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
	answers, err := s.answerRepo.ListByResponse(ctx, response.ID, actor.TenantID)
	if err != nil {
		return nil, err
	}
	offerLines, err := s.listCarrierOfferLines(ctx, response.ID, actor.TenantID)
	if err != nil {
		return nil, err
	}

	target, err := buildTargetCarrierBaseline(event, response, version, sections, rules, lots, answers, offerLines)
	if err != nil {
		return nil, apperrors.Internal("failed to build carrier import baseline", err)
	}

	preview, err := xlsxexchange.ParseCarrierImportPreview(ctx, workbookBytes, target)
	if err != nil {
		return nil, apperrors.Internal("failed to parse carrier import preview", err)
	}
	responseEnvelope := toCarrierImportPreviewResponse(preview)
	if preview.StaleBaseline {
		return responseEnvelope, apperrors.Conflict("carrier xlsx import baseline is stale", map[string]any{
			"machine_code": domain.MachineCodeStaleTarget,
		})
	}
	if !preview.ReadyToCommit {
		return responseEnvelope, nil
	}

	if s.importAnalysisRepo == nil || s.txRunner == nil {
		return nil, apperrors.Internal("import analysis persistence is not configured", nil)
	}

	createdAt := s.nowFn().UTC()
	expiresAt := createdAt.Add(importAnalysisTTL)
	payloadJSON, err := xlsxexchange.CanonicalCarrierImportPayloadJSON(preview, target, preview.Proposal)
	if err != nil {
		return nil, apperrors.Internal("failed to marshal canonical carrier import payload", err)
	}
	canonicalHash, err := xlsxexchange.StableStoredCarrierPayloadHash(payloadJSON)
	if err != nil {
		return nil, apperrors.Internal("failed to compute canonical carrier import payload hash", err)
	}
	summaryJSON, err := json.Marshal(preview.Summary)
	if err != nil {
		return nil, apperrors.Internal("failed to marshal validation summary", err)
	}

	targetID := response.ID
	targetVersion := int(response.SaveVersion)
	saveVersion := targetVersion
	analysisInput := domain.ImportAnalysis{
		TenantID:             actor.TenantID,
		ActorID:              actor.UserID,
		ActorCompanyID:       response.ParticipantCompanyID,
		WorkbookType:         domain.WorkbookTypeCarrierOffer,
		SchemaVersion:        domain.SchemaVersionCarrierXLSXV1,
		TargetType:           domain.ImportTargetTypeCarrierResponse,
		TargetID:             &targetID,
		TargetVersion:        &saveVersion,
		CanonicalPayloadJSON: payloadJSON,
		CanonicalHash:        canonicalHash,
		ValidationSummary:    summaryJSON,
		CreatedAt:            createdAt,
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

	responseEnvelope.AnalysisID = &persisted.ID
	responseEnvelope.ExpiresAt = &persisted.ExpiresAt
	responseEnvelope.CanonicalPayloadHash = persisted.CanonicalHash
	return responseEnvelope, nil
}

func (s *ExcelExchangeService) authorizeCarrierResponsePreview(
	ctx context.Context,
	actor domain.ActorContext,
	eventID uuid.UUID,
	responseID uuid.UUID,
) (*domain.RfxEvent, *domain.RfxResponse, error) {
	event, response, err := s.authorizeCarrierResponseExport(ctx, actor, eventID, responseID)
	if err != nil {
		return nil, nil, err
	}
	if response.Status != domain.RfxResponseStatusDraft {
		return nil, nil, apperrors.Conflict("response is not editable", map[string]any{
			"machine_code": xlsxexchange.MachineCodeResponseNotEditable,
			"status":       response.Status,
		})
	}
	return event, response, nil
}

func buildTargetCarrierBaseline(
	event *domain.RfxEvent,
	response *domain.RfxResponse,
	version *domain.RfxVersion,
	sections []domain.SectionWithQuestions,
	rules []domain.QuestionRule,
	lots []domain.RfxLot,
	answers []domain.CarrierAnswer,
	offerLines []domain.RfxResponseOfferLine,
) (xlsxexchange.TargetCarrierBaseline, error) {
	snapshot, err := buildCarrierResponseSnapshot(event, response, version, sections, rules, lots, answers, offerLines, time.Time{})
	if err != nil {
		return xlsxexchange.TargetCarrierBaseline{}, err
	}
	activeLots := make([]domain.RfxLot, 0, len(lots))
	for _, lot := range lots {
		if strings.EqualFold(strings.TrimSpace(lot.Status), "DELETED") {
			continue
		}
		activeLots = append(activeLots, lot)
	}
	eventCurrency := ""
	if event.CurrencyCode != nil {
		eventCurrency = *event.CurrencyCode
	}
	return xlsxexchange.TargetCarrierBaseline{
		TenantID:                   event.TenantID,
		EventID:                    event.ID,
		ResponseID:                 response.ID,
		CarrierCompanyID:           response.ParticipantCompanyID,
		RfxVersionID:               version.ID,
		QuestionnaireVersionNumber: version.VersionNumber,
		ResponseSaveVersion:        response.SaveVersion,
		ResponseStatus:             response.Status,
		EventRowVersion:            event.Version,
		EventCurrency:              eventCurrency,
		LotCount:                   len(activeLots),
		AvailableLotsFingerprint:   snapshot.Metadata.AvailableLotsFingerprint,
		AnswersFingerprint:         snapshot.Metadata.AnswersFingerprint,
		OfferLinesFingerprint:      snapshot.Metadata.OfferLinesFingerprint,
		Questionnaire: domain.QuestionnaireDefinition{
			EventID:              event.ID,
			RfxVersionID:         version.ID,
			VersionNumber:        version.VersionNumber,
			QuestionnaireEnabled: true,
			VersionStatus:        version.Status,
			Sections:             sections,
			Rules:                rules,
		},
		Lots:               activeLots,
		BaselineAnswers:    snapshot.Answers,
		BaselineOfferLines: snapshot.OfferLines,
	}, nil
}

func toCarrierImportPreviewResponse(preview xlsxexchange.CarrierImportPreview) *CarrierImportPreviewResponse {
	errors := preview.Errors
	if errors == nil {
		errors = []xlsxexchange.BuyerImportIssue{}
	}
	warnings := preview.Warnings
	if warnings == nil {
		warnings = []xlsxexchange.BuyerImportIssue{}
	}
	return &CarrierImportPreviewResponse{
		SchemaName:                preview.SchemaName,
		SchemaVersion:             preview.SchemaVersion,
		Mode:                      preview.Mode,
		TargetEventID:             preview.TargetEventID,
		TargetResponseID:          preview.TargetResponseID,
		TargetRfxVersionID:        preview.TargetRfxVersionID,
		TargetVersionNumber:       preview.TargetVersionNumber,
		TargetEventRowVersion:     preview.TargetEventRowVersion,
		TargetResponseSaveVersion: preview.TargetResponseSaveVersion,
		CanonicalPayloadHash:      preview.CanonicalPayloadHash,
		ReadyToCommit:             preview.ReadyToCommit,
		Summary:                   preview.Summary,
		AnswersDiff:               preview.AnswersDiff,
		OfferLinesDiff:            preview.OfferLinesDiff,
		Errors:                    errors,
		Warnings:                  warnings,
	}
}

// CarrierStructuralPreviewAppError maps the first structural parser issue to a safe HTTP 400 envelope.
func CarrierStructuralPreviewAppError(first xlsxexchange.BuyerImportIssue) *apperrors.AppError {
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
	return apperrors.Validation("carrier xlsx import preview failed", details)
}
