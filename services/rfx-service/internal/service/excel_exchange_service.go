package service

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/freight-platform/rfx-service/internal/domain"
	apperrors "github.com/freight-platform/rfx-service/internal/platform/errors"
	"github.com/freight-platform/rfx-service/internal/repository"
	"github.com/freight-platform/rfx-service/internal/xlsxexchange"
)

const BuyerDraftXLSXContentType = "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet"

type ExcelExchangeQuestionnaireStore interface {
	GetActiveDraftVersion(ctx context.Context, tenantID, eventID uuid.UUID) (*domain.RfxVersion, error)
	LoadQuestionnaireTree(ctx context.Context, versionID, tenantID uuid.UUID) ([]domain.SectionWithQuestions, error)
	ListRulesByVersion(ctx context.Context, versionID, tenantID uuid.UUID) ([]domain.QuestionRule, error)
}

type ExcelExchangeRfxStore interface {
	GetEventByID(ctx context.Context, id, tenantID uuid.UUID) (*domain.RfxEvent, error)
	ListLotsByEvent(ctx context.Context, eventID, tenantID uuid.UUID) ([]domain.RfxLot, error)
	GetEventExchangeMetadata(ctx context.Context, eventID, tenantID uuid.UUID) (*repository.EventExchangeMetadata, error)
}

type ExcelExchangeService struct {
	rfxRepo            ExcelExchangeRfxStore
	qRepo              ExcelExchangeQuestionnaireStore
	auth               *RfxService
	importAnalysisRepo *repository.ImportAnalysisRepository
	txRunner           previewTransactionRunner
	nowFn              func() time.Time
}

func NewExcelExchangeService(
	rfxRepo ExcelExchangeRfxStore,
	qRepo ExcelExchangeQuestionnaireStore,
	auth *RfxService,
	importAnalysisRepo *repository.ImportAnalysisRepository,
	txRunner previewTransactionRunner,
) *ExcelExchangeService {
	return &ExcelExchangeService{
		rfxRepo:            rfxRepo,
		qRepo:              qRepo,
		auth:               auth,
		importAnalysisRepo: importAnalysisRepo,
		txRunner:           txRunner,
		nowFn:              nowUTC,
	}
}

func (s *ExcelExchangeService) SetNowFunc(fn func() time.Time) {
	if fn != nil {
		s.nowFn = fn
	}
}

func (s *ExcelExchangeService) ExportBuyerDraftWorkbook(
	ctx context.Context,
	actor domain.ActorContext,
	eventID uuid.UUID,
) ([]byte, string, error) {
	event, err := s.authorizeBuyerManage(ctx, actor, eventID)
	if err != nil {
		return nil, "", err
	}

	version, err := s.qRepo.GetActiveDraftVersion(ctx, actor.TenantID, eventID)
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
	exchangeMeta, err := s.rfxRepo.GetEventExchangeMetadata(ctx, eventID, actor.TenantID)
	if err != nil {
		return nil, "", err
	}

	snapshot := buildBuyerDraftSnapshot(event, version, sections, rules, lots, exchangeMeta, s.nowFn())
	data, err := xlsxexchange.GenerateBuyerDraftWorkbook(snapshot)
	if err != nil {
		return nil, "", apperrors.Internal("failed to generate buyer draft workbook", err)
	}
	filename := fmt.Sprintf("bintrans-rfx-%s-draft-v%d.xlsx", eventID, version.VersionNumber)
	return data, filename, nil
}

func buildBuyerDraftSnapshot(
	event *domain.RfxEvent,
	version *domain.RfxVersion,
	sections []domain.SectionWithQuestions,
	rules []domain.QuestionRule,
	lots []domain.RfxLot,
	exchangeMeta *repository.EventExchangeMetadata,
	exportedAt time.Time,
) xlsxexchange.BuyerDraftSnapshot {
	flatSections := make([]domain.Section, 0, len(sections))
	questions := make([]xlsxexchange.BuyerDraftQuestion, 0)
	options := make([]xlsxexchange.BuyerDraftOption, 0)
	questionCodes := make(map[uuid.UUID]string, len(sections))

	for _, swq := range sections {
		flatSections = append(flatSections, swq.Section)
		for _, question := range swq.Questions {
			questionCodes[question.ID] = question.QuestionCode
			questions = append(questions, xlsxexchange.BuyerDraftQuestion{
				SectionCode: swq.Section.SectionCode,
				Question:    question,
			})
			for _, option := range question.Options {
				options = append(options, xlsxexchange.BuyerDraftOption{
					QuestionCode: question.QuestionCode,
					Option:       option,
				})
			}
		}
	}

	exportRules := make([]xlsxexchange.BuyerDraftRule, 0, len(rules))
	for _, rule := range rules {
		targetQuestionCode := ""
		if rule.TargetQuestionID != nil {
			targetQuestionCode = questionCodes[*rule.TargetQuestionID]
		}
		exportRules = append(exportRules, xlsxexchange.BuyerDraftRule{
			RuleCode:           rule.RuleCode,
			SourceQuestionCode: xlsxexchange.ExtractSourceQuestionCode(rule.ConditionJSON),
			ConditionJSON:      rule.ConditionJSON,
			TargetQuestionCode: targetQuestionCode,
			Action:             rule.Action,
			SortOrder:          rule.SortOrder,
		})
	}

	creationChannel := domain.CreationChannelManual
	var sourceTemplateVersionID *uuid.UUID
	var sourceTemplateVersionNumber *int
	if exchangeMeta != nil {
		creationChannel = exchangeMeta.CreationChannel
		sourceTemplateVersionID = exchangeMeta.SourceTemplateVersionID
		sourceTemplateVersionNumber = exchangeMeta.SourceTemplateVersionNumber
	}

	return xlsxexchange.BuyerDraftSnapshot{
		Metadata: xlsxexchange.BuyerDraftMetadata{
			ExportedAtUTC:               exportedAt.UTC(),
			TenantID:                    event.TenantID,
			RfxEventID:                  event.ID,
			RfxVersionID:                version.ID,
			VersionNumber:               version.VersionNumber,
			VersionStatus:               version.Status,
			EventRowVersion:             event.Version,
			VersionRowVersion:           version.Version,
			CreationChannel:             creationChannel,
			SourceTemplateVersionID:     sourceTemplateVersionID,
			SourceTemplateVersionNumber: sourceTemplateVersionNumber,
		},
		Lots:      lots,
		Sections:  flatSections,
		Questions: questions,
		Options:   options,
		Rules:     exportRules,
	}
}

func (s *ExcelExchangeService) authorizeBuyerManage(ctx context.Context, actor domain.ActorContext, eventID uuid.UUID) (*domain.RfxEvent, error) {
	event, err := s.authorizeBuyerRead(ctx, actor, eventID)
	if err != nil {
		return nil, err
	}
	resolver, ok := s.auth.actors.(CompanyMembershipResolver)
	if !ok {
		return nil, apperrors.Forbidden("buyer manage permission is required")
	}
	roles, err := resolver.ListUserRoleCodes(ctx, actor.TenantID, actor.UserID)
	if err != nil {
		return nil, err
	}
	if !domain.HasBuyerManageRole(roles) {
		return nil, apperrors.Forbidden("buyer manage permission is required")
	}
	return event, nil
}

func (s *ExcelExchangeService) authorizeBuyerRead(ctx context.Context, actor domain.ActorContext, eventID uuid.UUID) (*domain.RfxEvent, error) {
	if err := actor.Validate(); err != nil {
		return nil, err
	}
	event, err := s.rfxRepo.GetEventByID(ctx, eventID, actor.TenantID)
	if err != nil {
		return nil, err
	}
	if err := s.auth.requireBuyerActor(ctx, actor); err != nil {
		return nil, err
	}
	buyerCompanyIDs, err := s.auth.listBuyerCompanyIDs(ctx, actor)
	if err != nil {
		return nil, err
	}
	if !domain.ContainsCompanyID(buyerCompanyIDs, event.OwnerCompanyID) {
		return nil, apperrors.NotFound("rfx event not found")
	}
	return event, nil
}
