package service

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"
	"golang.org/x/text/unicode/norm"

	"github.com/freight-platform/rfx-service/internal/domain"
	"github.com/freight-platform/rfx-service/internal/erpjson"
	apperrors "github.com/freight-platform/rfx-service/internal/platform/errors"
)

type ErpPublishReadinessSummary struct {
	Ready             bool `json:"ready"`
	BlockingFailCount int  `json:"blocking_fail_count"`
	WarningCount      int  `json:"warning_count"`
}

type ErpQuestionnaireCounts struct {
	SectionCount  int `json:"section_count"`
	QuestionCount int `json:"question_count"`
	RuleCount     int `json:"rule_count"`
}

type ErpExternalLink struct {
	System            string  `json:"system"`
	ObjectType        string  `json:"object_type"`
	ObjectID          string  `json:"object_id"`
	Revision          string  `json:"revision"`
	RequestedRevision *string `json:"requested_revision,omitempty"`
}

type ErpRfxDraftSummary struct {
	RfxEventID              uuid.UUID                  `json:"rfx_event_id"`
	Status                  string                     `json:"status"`
	RfxType                 string                     `json:"rfx_type"`
	Title                   string                     `json:"title"`
	Description             *string                    `json:"description,omitempty"`
	CurrencyCode            *string                    `json:"currency_code,omitempty"`
	Timezone                string                     `json:"timezone"`
	ResponseDeadline        *time.Time                 `json:"response_deadline,omitempty"`
	CreationChannel         string                     `json:"creation_channel"`
	ExternalLink            *ErpExternalLink           `json:"external_link"`
	PublishReadinessSummary ErpPublishReadinessSummary `json:"publish_readiness_summary"`
	LotCount                int                        `json:"lot_count"`
	QuestionnaireCounts     ErpQuestionnaireCounts     `json:"questionnaire_counts"`
	EventRowVersion         int                        `json:"event_row_version"`
	DraftRowVersion         int                        `json:"draft_row_version"`
}

type ErpAnalysisStatus struct {
	AnalysisID        uuid.UUID       `json:"analysis_id"`
	Status            string          `json:"status"`
	ExpiresAt         time.Time       `json:"expires_at"`
	ConsumedAt        *time.Time      `json:"consumed_at,omitempty"`
	ValidationSummary json.RawMessage `json:"validation_summary"`
	ReadyToCommit     bool            `json:"ready_to_commit"`
}

type ErpIntegrationLimits struct {
	MaxBodyBytes int `json:"max_body_bytes"`
	MaxJSONDepth int `json:"max_json_depth"`
	MaxLots      int `json:"max_lots"`
}

type ErpIntegrationCapabilities struct {
	SchemaVersion         string               `json:"schema_version"`
	Limits                ErpIntegrationLimits `json:"limits"`
	SupportedMappingTypes []string             `json:"supported_mapping_types"`
	DeferredFields        []string             `json:"deferred_fields"`
}

func (s *ErpIntegrationService) GetRfxDraftSummaryByID(ctx context.Context, actor IntegrationActor, eventID uuid.UUID) (*ErpRfxDraftSummary, error) {
	if !actor.HasScopes(domain.ScopeDraftRead) {
		return nil, apperrors.Forbidden("missing required scope")
	}
	event, err := s.authorizeIntegrationEvent(ctx, actor, eventID)
	if err != nil {
		return nil, err
	}
	if event.Status != domain.RfxStatusDraft {
		return nil, eventNotDraft()
	}
	return s.buildRfxDraftSummary(ctx, actor, event, nil)
}

func (s *ErpIntegrationService) GetRfxDraftSummaryByExternalID(
	ctx context.Context,
	actor IntegrationActor,
	externalSystem, externalObjectID, requestedRevision string,
) (*ErpRfxDraftSummary, error) {
	if !actor.HasScopes(domain.ScopeDraftRead) {
		return nil, apperrors.Forbidden("missing required scope")
	}
	system, objectID, err := normalizeExternalLookup(externalSystem, externalObjectID)
	if err != nil {
		return nil, err
	}
	if s.linkRepo == nil {
		return nil, apperrors.NotFound("rfx event not found")
	}
	link, err := s.linkRepo.GetByStableIdentity(ctx, actor.TenantID, actor.PrincipalID, system, domain.ExternalObjectTypeRfxEvent, objectID)
	if err != nil {
		if isNotFound(err) {
			return nil, apperrors.NotFound("rfx event not found")
		}
		return nil, err
	}
	if err := s.ensureExternalRevisionKnown(ctx, actor.TenantID, link, requestedRevision); err != nil {
		return nil, err
	}
	event, err := s.authorizeIntegrationEvent(ctx, actor, link.RfxEventID)
	if err != nil {
		return nil, err
	}
	if event.Status != domain.RfxStatusDraft {
		return nil, eventNotDraft()
	}
	var requested *string
	if requestedRevision != "" {
		requested = &requestedRevision
	}
	return s.buildRfxDraftSummary(ctx, actor, event, requested)
}

func (s *ErpIntegrationService) GetAnalysisStatus(ctx context.Context, actor IntegrationActor, analysisID uuid.UUID) (*ErpAnalysisStatus, error) {
	if !actor.HasScopes(domain.ScopeStatusRead) {
		return nil, apperrors.Forbidden("missing required scope")
	}
	if s.importAnalysisRepo == nil {
		return nil, analysisNotFound()
	}
	analysis, err := s.importAnalysisRepo.GetByID(ctx, analysisID, actor.TenantID)
	if err != nil {
		if isNotFound(err) {
			return nil, analysisNotFound()
		}
		return nil, err
	}
	if analysis.ActorCompanyID != actor.CompanyID {
		return nil, analysisNotFound()
	}
	if analysis.IntegrationPrincipalID == nil || *analysis.IntegrationPrincipalID != actor.PrincipalID || analysis.ActorID != uuid.Nil {
		return nil, analysisNotFound()
	}
	status := analysis.Status
	ready := false
	if status == domain.ImportAnalysisStatusPreviewed && !analysis.ExpiresAt.After(s.nowFn().UTC()) {
		status = domain.ImportAnalysisStatusExpired
	}
	summary := json.RawMessage(`{}`)
	if len(analysis.ValidationSummary) > 0 {
		summary = json.RawMessage(analysis.ValidationSummary)
		var parsed map[string]any
		if json.Unmarshal(analysis.ValidationSummary, &parsed) == nil {
			if flag, ok := parsed["ready_to_commit"].(bool); ok {
				ready = flag
			}
		}
	}
	if status != domain.ImportAnalysisStatusPreviewed {
		ready = false
	}
	return &ErpAnalysisStatus{
		AnalysisID:        analysis.ID,
		Status:            status,
		ExpiresAt:         analysis.ExpiresAt,
		ConsumedAt:        analysis.ConsumedAt,
		ValidationSummary: summary,
		ReadyToCommit:     ready,
	}, nil
}

func (s *ErpIntegrationService) GetCapabilities(_ context.Context, actor IntegrationActor) (*ErpIntegrationCapabilities, error) {
	if !actor.HasScopes(domain.ScopeStatusRead) {
		return nil, apperrors.Forbidden("missing required scope")
	}
	return &ErpIntegrationCapabilities{
		SchemaVersion: domain.SchemaVersionERPJSONV1,
		Limits: ErpIntegrationLimits{
			MaxBodyBytes: erpjson.MaxBodyBytes,
			MaxJSONDepth: erpjson.MaxJSONDepth,
			MaxLots:      erpjson.MaxLots,
		},
		SupportedMappingTypes: erpjson.SupportedMappingTypesV1(),
		DeferredFields:        erpjson.DeferredFieldsV1(),
	}, nil
}

func (s *ErpIntegrationService) buildRfxDraftSummary(
	ctx context.Context,
	actor IntegrationActor,
	event *domain.RfxEvent,
	requestedRevision *string,
) (*ErpRfxDraftSummary, error) {
	summary := &ErpRfxDraftSummary{
		RfxEventID:       event.ID,
		Status:           event.Status,
		RfxType:          event.RfxType,
		Title:            event.Title,
		Description:      event.Description,
		CurrencyCode:     event.CurrencyCode,
		EventRowVersion:  event.Version,
		CreationChannel:  domain.CreationChannelManual,
		ExternalLink:     nil,
		ResponseDeadline: event.ResponseDeadline,
	}
	if s.rfxRepo != nil {
		if meta, err := s.rfxRepo.GetEventExchangeMetadata(ctx, event.ID, actor.TenantID); err != nil && !isNotFound(err) {
			return nil, err
		} else if err == nil && meta != nil {
			summary.CreationChannel = meta.CreationChannel
		}
		lots, err := s.rfxRepo.ListLotsByEvent(ctx, event.ID, actor.TenantID)
		if err != nil {
			return nil, err
		}
		summary.LotCount = len(lots)
	}
	if s.linkRepo != nil {
		link, err := s.linkRepo.GetByRfxEventID(ctx, actor.TenantID, actor.PrincipalID, event.ID)
		if err != nil && !isNotFound(err) {
			return nil, err
		}
		if err == nil && link != nil {
			summary.ExternalLink = &ErpExternalLink{
				System:            link.ExternalSystem,
				ObjectType:        link.ExternalObjectType,
				ObjectID:          link.ExternalObjectID,
				Revision:          link.ExternalRevision,
				RequestedRevision: requestedRevision,
			}
		}
	}
	if s.qRepo != nil {
		version, err := s.qRepo.GetActiveDraftVersion(ctx, actor.TenantID, event.ID)
		if err != nil && !isNotFound(err) && !isDraftMissing(err) {
			return nil, err
		}
		if err == nil && version != nil {
			summary.DraftRowVersion = version.Version
			sections, secErr := s.qRepo.LoadQuestionnaireTree(ctx, version.ID, actor.TenantID)
			if secErr != nil {
				return nil, secErr
			}
			rules, ruleErr := s.qRepo.ListRulesByVersion(ctx, version.ID, actor.TenantID)
			if ruleErr != nil {
				return nil, ruleErr
			}
			questionCount := 0
			for _, section := range sections {
				questionCount += len(section.Questions)
			}
			summary.QuestionnaireCounts = ErpQuestionnaireCounts{
				SectionCount:  len(sections),
				QuestionCount: questionCount,
				RuleCount:     len(rules),
			}
			readiness := domain.EvaluatePublishReadiness(*version, sections, rules)
			summary.PublishReadinessSummary = ErpPublishReadinessSummary{
				Ready:             readiness.Ready,
				BlockingFailCount: readiness.BlockingFail,
				WarningCount:      readiness.WarningCount,
			}
		}
	}
	if tz, err := s.timezoneFromLatestConsumedAnalysis(ctx, actor, event.ID); err == nil {
		summary.Timezone = tz
	}
	return summary, nil
}

func (s *ErpIntegrationService) timezoneFromLatestConsumedAnalysis(ctx context.Context, actor IntegrationActor, eventID uuid.UUID) (string, error) {
	if s.importAnalysisRepo == nil {
		return "", nil
	}
	analysis, err := s.importAnalysisRepo.GetLatestConsumedERPForEvent(ctx, actor.TenantID, actor.PrincipalID, eventID)
	if err != nil {
		if isNotFound(err) {
			return "", nil
		}
		return "", err
	}
	var payload struct {
		Event struct {
			Timezone string `json:"timezone"`
		} `json:"event"`
	}
	if json.Unmarshal(analysis.CanonicalPayloadJSON, &payload) != nil {
		return "", nil
	}
	return strings.TrimSpace(payload.Event.Timezone), nil
}

func (s *ErpIntegrationService) ensureExternalRevisionKnown(ctx context.Context, tenantID uuid.UUID, link *domain.ExternalObjectLink, requestedRevision string) error {
	requestedRevision = strings.TrimSpace(requestedRevision)
	if requestedRevision == "" {
		return nil
	}
	if requestedRevision == strings.TrimSpace(link.ExternalRevision) {
		return nil
	}
	exists, err := s.linkRepo.RevisionExists(ctx, tenantID, link.ID, requestedRevision)
	if err != nil {
		return err
	}
	if !exists {
		return apperrors.NotFound("rfx event not found")
	}
	return nil
}

func normalizeExternalLookup(system, objectID string) (string, string, error) {
	normalizedSystem := strings.ToUpper(strings.TrimSpace(system))
	normalizedObjectID := norm.NFC.String(strings.TrimSpace(objectID))
	if normalizedSystem == "" || len(normalizedSystem) > 64 {
		return "", "", apperrors.Validation("external_system is required", map[string]any{"field": "external_system"})
	}
	if normalizedObjectID == "" || utf8.RuneCountInString(normalizedObjectID) > 256 {
		return "", "", apperrors.Validation("external_object_id is required", map[string]any{"field": "external_object_id"})
	}
	return normalizedSystem, normalizedObjectID, nil
}

func eventNotDraft() error {
	return apperrors.Conflict("rfx event is not a draft", map[string]any{"machine_code": domain.MachineCodeEventNotDraft})
}

func isDraftMissing(err error) bool {
	var appErr *apperrors.AppError
	return errors.As(err, &appErr) && appErr.Code == apperrors.CodeConflict
}
