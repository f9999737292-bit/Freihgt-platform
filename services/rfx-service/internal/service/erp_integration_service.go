package service

import (
	"context"
	"encoding/json"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/freight-platform/rfx-service/internal/domain"
	"github.com/freight-platform/rfx-service/internal/erpjson"
	apperrors "github.com/freight-platform/rfx-service/internal/platform/errors"
	"github.com/freight-platform/rfx-service/internal/repository"
)

const erpImportAnalysisTTL = 24 * time.Hour

type ErpIntegrationService struct {
	rfxRepo            *repository.RfxRepository
	qRepo              *repository.QuestionnaireRepository
	importAnalysisRepo *repository.ImportAnalysisRepository
	mappingResolver    *ErpMappingResolver
	auditRepo          *repository.AuditRepository
	txRunner           previewTransactionRunner
	nowFn              func() time.Time
}

func NewErpIntegrationService(
	rfxRepo *repository.RfxRepository,
	qRepo *repository.QuestionnaireRepository,
	importAnalysisRepo *repository.ImportAnalysisRepository,
	mappingResolver *ErpMappingResolver,
	auditRepo *repository.AuditRepository,
	txRunner previewTransactionRunner,
) *ErpIntegrationService {
	return &ErpIntegrationService{
		rfxRepo:            rfxRepo,
		qRepo:              qRepo,
		importAnalysisRepo: importAnalysisRepo,
		mappingResolver:    mappingResolver,
		auditRepo:          auditRepo,
		txRunner:           txRunner,
		nowFn:              nowUTC,
	}
}

func (s *ErpIntegrationService) PreviewCreateDraft(ctx context.Context, actor IntegrationActor, raw []byte) (*erpjson.PreviewResponse, error) {
	if err := actor.Validate(); err != nil {
		return nil, err
	}
	if !actor.HasScopes(domain.ScopeDraftPreview, domain.ScopeDraftCreate) {
		return nil, apperrors.Forbidden("missing required scope")
	}
	parsed, err := erpjson.ParsePreview(ctx, actor.TenantID, erpjson.OperationCreateDraft, raw, s.mappingResolver)
	if err != nil {
		return nil, err
	}
	return s.finalizePreview(ctx, actor, parsed, domain.ImportTargetTypeNewEvent, nil, nil)
}

func (s *ErpIntegrationService) PreviewUpdateDraft(ctx context.Context, actor IntegrationActor, eventID uuid.UUID, raw []byte) (*erpjson.PreviewResponse, error) {
	if err := actor.Validate(); err != nil {
		return nil, err
	}
	if !actor.HasScopes(domain.ScopeDraftPreview, domain.ScopeDraftRead) {
		return nil, apperrors.Forbidden("missing required scope")
	}
	event, err := s.authorizeIntegrationEvent(ctx, actor, eventID)
	if err != nil {
		return nil, err
	}
	if event.Status != domain.RfxStatusDraft {
		return nil, apperrors.Conflict("stale_target", map[string]any{"machine_code": "stale_target"})
	}
	version, err := s.qRepo.GetActiveDraftVersion(ctx, actor.TenantID, eventID)
	if err != nil {
		return nil, err
	}
	targetVersion := version.VersionNumber
	parsed, err := erpjson.ParsePreview(ctx, actor.TenantID, erpjson.OperationUpdateDraft, raw, s.mappingResolver)
	if err != nil {
		return nil, err
	}
	return s.finalizePreview(ctx, actor, parsed, domain.ImportTargetTypeDraftEvent, &eventID, &targetVersion)
}

func (s *ErpIntegrationService) authorizeIntegrationEvent(ctx context.Context, actor IntegrationActor, eventID uuid.UUID) (*domain.RfxEvent, error) {
	event, err := s.rfxRepo.GetEventByID(ctx, eventID, actor.TenantID)
	if err != nil {
		return nil, err
	}
	if event.OwnerCompanyID != actor.CompanyID {
		return nil, apperrors.NotFound("rfx event not found")
	}
	return event, nil
}

func (s *ErpIntegrationService) finalizePreview(
	ctx context.Context,
	actor IntegrationActor,
	parsed *erpjson.ParsedPreview,
	targetType string,
	targetID *uuid.UUID,
	targetVersion *int,
) (*erpjson.PreviewResponse, error) {
	resp := &erpjson.PreviewResponse{
		SchemaVersion: domain.SchemaVersionERPJSONV1,
		ReadyToCommit: parsed.ReadyToCommit,
		Errors:        parsed.Errors,
		Warnings:      parsed.Warnings,
	}
	if resp.Errors == nil {
		resp.Errors = []erpjson.Issue{}
	}
	if resp.Warnings == nil {
		resp.Warnings = []erpjson.Issue{}
	}
	if erpjson.HasErrors(parsed.Errors) || !parsed.ReadyToCommit {
		resp.ReadyToCommit = false
		return resp, nil
	}
	if s.importAnalysisRepo == nil || s.txRunner == nil {
		return nil, apperrors.Internal("import analysis persistence is not configured", nil)
	}
	createdAt := s.nowFn().UTC()
	expiresAt := createdAt.Add(erpImportAnalysisTTL)
	summaryJSON, _ := json.Marshal(map[string]any{"ready_to_commit": true})

	analysisInput := actor.ToAnalysisOwner(actor.CompanyID)
	analysisInput.WorkbookType = domain.WorkbookTypeERPBuyerJSON
	analysisInput.SchemaVersion = domain.SchemaVersionERPJSONV1
	analysisInput.TargetType = targetType
	analysisInput.TargetID = targetID
	analysisInput.TargetVersion = targetVersion
	analysisInput.CanonicalPayloadJSON = parsed.CanonicalJSON
	analysisInput.CanonicalHash = parsed.CanonicalHash
	analysisInput.ValidationSummary = summaryJSON
	analysisInput.CreatedAt = createdAt
	analysisInput.ExpiresAt = expiresAt

	var persisted *domain.ImportAnalysis
	if err := s.txRunner.Run(ctx, func(ctx context.Context, tx pgx.Tx) error {
		repo := s.importAnalysisRepo.WithTx(tx)
		created, createErr := repo.CreatePreviewForIntegrationPrincipal(ctx, analysisInput)
		if createErr != nil {
			return createErr
		}
		persisted = created
		return nil
	}); err != nil {
		return nil, apperrors.Internal("failed to persist import analysis", err)
	}

	resp.AnalysisID = &persisted.ID
	resp.ExpiresAt = &persisted.ExpiresAt
	resp.CanonicalPayloadHash = persisted.CanonicalHash
	resp.ReadyToCommit = true
	return resp, nil
}
