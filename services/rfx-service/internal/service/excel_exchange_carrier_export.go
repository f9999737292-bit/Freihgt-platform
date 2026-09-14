package service

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/freight-platform/rfx-service/internal/domain"
	apperrors "github.com/freight-platform/rfx-service/internal/platform/errors"
	"github.com/freight-platform/rfx-service/internal/xlsxexchange"
)

func (s *ExcelExchangeService) ExportCarrierResponseWorkbook(
	ctx context.Context,
	actor domain.ActorContext,
	eventID uuid.UUID,
	responseID uuid.UUID,
) ([]byte, string, error) {
	event, response, err := s.authorizeCarrierResponseExport(ctx, actor, eventID, responseID)
	if err != nil {
		return nil, "", err
	}

	version, err := s.qRepo.GetVersionByID(ctx, *response.RfxVersionID, actor.TenantID)
	if err != nil {
		return nil, "", err
	}
	sections, err := s.qRepo.LoadQuestionnaireTree(ctx, version.ID, actor.TenantID)
	if err != nil {
		return nil, "", err
	}
	rules, err := s.qRepo.ListRulesByVersion(ctx, version.ID, actor.TenantID)
	if err != nil {
		return nil, "", err
	}
	lots, err := s.rfxRepo.ListLotsByEvent(ctx, eventID, actor.TenantID)
	if err != nil {
		return nil, "", err
	}
	answers, err := s.answerRepo.ListByResponse(ctx, response.ID, actor.TenantID)
	if err != nil {
		return nil, "", err
	}
	offerLines, err := s.listCarrierOfferLines(ctx, response.ID, actor.TenantID)
	if err != nil {
		return nil, "", err
	}

	snapshot, err := buildCarrierResponseSnapshot(
		event,
		response,
		version,
		sections,
		rules,
		lots,
		answers,
		offerLines,
		s.nowFn(),
	)
	if err != nil {
		return nil, "", apperrors.Internal("failed to build carrier response snapshot", err)
	}
	data, err := xlsxexchange.GenerateCarrierResponseWorkbook(snapshot)
	if err != nil {
		return nil, "", apperrors.Internal("failed to generate carrier response workbook", err)
	}
	filename := fmt.Sprintf(
		"BINTRANS_RFX_CARRIER_%s_%s_V1.xlsx",
		shortResourceID(eventID),
		shortResourceID(responseID),
	)
	return data, filename, nil
}

func (s *ExcelExchangeService) authorizeCarrierResponseExport(
	ctx context.Context,
	actor domain.ActorContext,
	eventID uuid.UUID,
	responseID uuid.UUID,
) (*domain.RfxEvent, *domain.RfxResponse, error) {
	if err := actor.Validate(); err != nil {
		return nil, nil, err
	}
	if s.answerRepo == nil {
		return nil, nil, apperrors.Internal("answer store unavailable", nil)
	}
	response, err := s.rfxRepo.GetResponseByID(ctx, responseID, actor.TenantID)
	if err != nil {
		return nil, nil, err
	}
	if response.RfxEventID != eventID {
		return nil, nil, apperrors.NotFound("rfx response not found")
	}
	event, err := s.rfxRepo.GetEventByID(ctx, eventID, actor.TenantID)
	if err != nil {
		return nil, nil, err
	}
	if _, err := s.auth.requireCarrierActor(ctx, actor); err != nil {
		return nil, nil, err
	}
	_, carrierIDs, err := s.auth.resolveActor(ctx, actor)
	if err != nil {
		return nil, nil, err
	}
	if !carrierCanViewResponse(carrierIDs, response.ParticipantCompanyID) {
		return nil, nil, apperrors.NotFound("rfx response not found")
	}
	switch response.Status {
	case domain.RfxResponseStatusDraft, domain.RfxResponseStatusSubmitted:
	default:
		return nil, nil, apperrors.NotFound("rfx response not found")
	}
	if response.RfxVersionID == nil {
		return nil, nil, apperrors.Validation(
			"carrier response is not bound to a published questionnaire version",
			map[string]any{"field": "rfx_version_id"},
		)
	}
	return event, response, nil
}

func (s *ExcelExchangeService) listCarrierOfferLines(
	ctx context.Context,
	responseID, tenantID uuid.UUID,
) ([]domain.RfxResponseOfferLine, error) {
	store := s.auth.evalStore()
	if store == nil {
		return nil, apperrors.Internal("evaluation store unavailable", nil)
	}
	return store.ListOfferLinesByResponse(ctx, responseID, tenantID)
}

func buildCarrierResponseSnapshot(
	event *domain.RfxEvent,
	response *domain.RfxResponse,
	version *domain.RfxVersion,
	sections []domain.SectionWithQuestions,
	rules []domain.QuestionRule,
	lots []domain.RfxLot,
	answers []domain.CarrierAnswer,
	offerLines []domain.RfxResponseOfferLine,
	exportedAt time.Time,
) (xlsxexchange.CarrierResponseSnapshot, error) {
	questionCodes := make(map[uuid.UUID]string)
	questions := make([]xlsxexchange.CarrierQuestion, 0)
	options := make([]xlsxexchange.CarrierOption, 0)
	for _, swq := range sections {
		for _, question := range swq.Questions {
			questionCodes[question.ID] = question.QuestionCode
			questions = append(questions, xlsxexchange.CarrierQuestion{
				SectionCode: swq.Section.SectionCode,
				Question:    question,
			})
			for _, option := range question.Options {
				options = append(options, xlsxexchange.CarrierOption{
					QuestionCode: question.QuestionCode,
					Option:       option,
				})
			}
		}
	}

	exportRules := make([]xlsxexchange.CarrierRule, 0, len(rules))
	for _, rule := range rules {
		targetQuestionCode := ""
		if rule.TargetQuestionID != nil {
			targetQuestionCode = questionCodes[*rule.TargetQuestionID]
		}
		exportRules = append(exportRules, xlsxexchange.CarrierRule{
			RuleCode:           rule.RuleCode,
			SourceQuestionCode: xlsxexchange.ExtractSourceQuestionCode(rule.ConditionJSON),
			ConditionJSON:      rule.ConditionJSON,
			TargetQuestionCode: targetQuestionCode,
			Action:             rule.Action,
			SortOrder:          rule.SortOrder,
		})
	}

	answerRows := make([]xlsxexchange.CarrierAnswerRow, 0, len(answers))
	for _, answer := range answers {
		code := questionCodes[answer.QuestionID]
		if code == "" {
			continue
		}
		answerRows = append(answerRows, xlsxexchange.CarrierAnswerRow{
			QuestionCode: code,
			AnswerValue:  xlsxexchange.FormatCarrierAnswerValue(answer.AnswerValueJSON),
		})
	}

	lotNumberByID := make(map[uuid.UUID]string, len(lots))
	exportLots := make([]domain.RfxLot, 0, len(lots))
	for _, lot := range lots {
		if strings.EqualFold(strings.TrimSpace(lot.Status), "DELETED") {
			continue
		}
		exportLots = append(exportLots, lot)
		lotNumberByID[lot.ID] = lot.LotNumber
	}

	exportOfferLines := make([]xlsxexchange.CarrierOfferLineRow, 0, len(offerLines))
	for _, line := range offerLines {
		lotNumber := ""
		if line.RfxLotID != uuid.Nil {
			lotNumber = lotNumberByID[line.RfxLotID]
		}
		exportOfferLines = append(exportOfferLines, xlsxexchange.CarrierOfferLineRow{
			LotNumber:    lotNumber,
			Amount:       xlsxexchange.FormatOfferAmountForExport(line.Amount),
			CurrencyCode: line.CurrencyCode,
			Comment:      optionalStringPtr(line.Comment),
		})
	}

	lotsFingerprint, err := xlsxexchange.ComputeCarrierAvailableLotsFingerprint(exportLots)
	if err != nil {
		return xlsxexchange.CarrierResponseSnapshot{}, err
	}
	answersFingerprint, err := xlsxexchange.ComputeCarrierAnswersFingerprint(answerRows)
	if err != nil {
		return xlsxexchange.CarrierResponseSnapshot{}, err
	}
	offerLinesFingerprint, err := xlsxexchange.ComputeCarrierOfferLinesFingerprint(offerLines, lotNumberByID)
	if err != nil {
		return xlsxexchange.CarrierResponseSnapshot{}, err
	}

	exportMode := xlsxexchange.CarrierExportModeDraftEdit
	if response.Status == domain.RfxResponseStatusSubmitted {
		exportMode = xlsxexchange.CarrierExportModeSubmittedReadonly
	}

	return xlsxexchange.CarrierResponseSnapshot{
		Metadata: xlsxexchange.CarrierResponseMetadata{
			ExportedAtUTC:              exportedAt.UTC(),
			TenantID:                   event.TenantID,
			RfxEventID:                 event.ID,
			RfxResponseID:              response.ID,
			CarrierCompanyID:           response.ParticipantCompanyID,
			RfxVersionID:               version.ID,
			QuestionnaireVersionNumber: version.VersionNumber,
			ResponseSaveVersion:        response.SaveVersion,
			ResponseStatus:             response.Status,
			EventRowVersion:            event.Version,
			AvailableLotsFingerprint:   lotsFingerprint,
			AnswersFingerprint:         answersFingerprint,
			OfferLinesFingerprint:      offerLinesFingerprint,
			ExportMode:                 exportMode,
		},
		Lots:       exportLots,
		Questions:  questions,
		Options:    options,
		Rules:      exportRules,
		Answers:    answerRows,
		OfferLines: exportOfferLines,
	}, nil
}

func shortResourceID(id uuid.UUID) string {
	s := strings.ToUpper(strings.ReplaceAll(id.String(), "-", ""))
	if len(s) > 8 {
		return s[:8]
	}
	return s
}

func optionalStringPtr(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}
