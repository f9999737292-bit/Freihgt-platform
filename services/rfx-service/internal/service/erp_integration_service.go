package service

import (
	"context"
	"encoding/json"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/freight-platform/rfx-service/internal/domain"
	"github.com/freight-platform/rfx-service/internal/erpjson"
	apperrors "github.com/freight-platform/rfx-service/internal/platform/errors"
	"github.com/freight-platform/rfx-service/internal/repository"
	"github.com/freight-platform/rfx-service/internal/xlsxexchange"
)

const erpImportAnalysisTTL = 24 * time.Hour

type ErpIntegrationService struct {
	rfxRepo            *repository.RfxRepository
	qRepo              *repository.QuestionnaireRepository
	importAnalysisRepo *repository.ImportAnalysisRepository
	mappingResolver    *ErpMappingResolver
	auditRepo          *repository.AuditRepository
	idemRepo           *repository.IdempotencyRepository
	linkRepo           *repository.ExternalObjectLinkRepository
	mappingRepo        *repository.ReferenceMappingRepository
	txRunner           previewTransactionRunner
	nowFn              func() time.Time
	// afterCreateCommitIdempotencyMiss is a test-only hook invoked inside the
	// commit transaction after a nil idempotency Get, before event creation.
	afterCreateCommitIdempotencyMiss func()
	// afterUpdateCommitIdempotencyMiss is a test-only hook invoked inside the
	// UPDATE commit transaction after a nil idempotency Get, before apply.
	afterUpdateCommitIdempotencyMiss func()
	// afterUpdateCommitApply is a test-only hook invoked after graph apply
	// and before consume/idempotency store so rollback can be proven.
	afterUpdateCommitApply func() error
}

// SetAfterCreateCommitIdempotencyMiss arms a test-only barrier so concurrent
// commits can both observe a missing idempotency row before either Stores.
func (s *ErpIntegrationService) SetAfterCreateCommitIdempotencyMiss(fn func()) {
	if s == nil {
		return
	}
	s.afterCreateCommitIdempotencyMiss = fn
}

func (s *ErpIntegrationService) SetAfterUpdateCommitIdempotencyMiss(fn func()) {
	if s == nil {
		return
	}
	s.afterUpdateCommitIdempotencyMiss = fn
}

func (s *ErpIntegrationService) SetAfterUpdateCommitApply(fn func() error) {
	if s == nil {
		return
	}
	s.afterUpdateCommitApply = fn
}

func NewErpIntegrationService(
	rfxRepo *repository.RfxRepository,
	qRepo *repository.QuestionnaireRepository,
	importAnalysisRepo *repository.ImportAnalysisRepository,
	mappingResolver *ErpMappingResolver,
	auditRepo *repository.AuditRepository,
	txRunner previewTransactionRunner,
	idemRepo *repository.IdempotencyRepository,
	linkRepo *repository.ExternalObjectLinkRepository,
	mappingRepo *repository.ReferenceMappingRepository,
) *ErpIntegrationService {
	return &ErpIntegrationService{
		rfxRepo:            rfxRepo,
		qRepo:              qRepo,
		importAnalysisRepo: importAnalysisRepo,
		mappingResolver:    mappingResolver,
		auditRepo:          auditRepo,
		idemRepo:           idemRepo,
		linkRepo:           linkRepo,
		mappingRepo:        mappingRepo,
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
		return nil, apperrors.Conflict("stale_target", map[string]any{"machine_code": domain.MachineCodeStaleTarget})
	}
	version, err := s.qRepo.GetActiveDraftVersion(ctx, actor.TenantID, eventID)
	if err != nil {
		return nil, err
	}
	lots, err := s.rfxRepo.ListLotsByEvent(ctx, eventID, actor.TenantID)
	if err != nil {
		return nil, err
	}
	fingerprint, err := xlsxexchange.ComputeBaselineLotsFingerprint(lots)
	if err != nil {
		return nil, apperrors.Internal("failed to compute baseline lots fingerprint", err)
	}
	targetVersion := version.VersionNumber
	parsed, err := erpjson.ParsePreview(ctx, actor.TenantID, erpjson.OperationUpdateDraft, raw, s.mappingResolver)
	if err != nil {
		return nil, err
	}
	if parsed.ReadyToCommit && !erpjson.HasErrors(parsed.Errors) {
		existing, resolvedRevision, policyErr := s.validateUpdatePreviewExternalPolicy(ctx, actor, eventID, parsed)
		if policyErr != nil {
			return nil, policyErr
		}
		canonical := parsed.CanonicalJSON
		if parsed.External != nil && strings.TrimSpace(resolvedRevision) != "" {
			patched, patchErr := erpjson.ApplyStoredExternalRevision(canonical, resolvedRevision)
			if patchErr != nil {
				return nil, apperrors.Internal("failed to persist update external revision", patchErr)
			}
			canonical = patched
		}
		bound, bindErr := erpjson.BindUpdateBaselineTokens(canonical, updatePreviewBaselineTokens(
			event.Version, version.Version, fingerprint, existing,
		))
		if bindErr != nil {
			return nil, apperrors.Internal("failed to bind update baseline tokens", bindErr)
		}
		hash, hashErr := erpjson.StableHash(bound)
		if hashErr != nil {
			return nil, apperrors.Internal("failed to hash update canonical payload", hashErr)
		}
		parsed.CanonicalJSON = bound
		parsed.CanonicalHash = hash
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
