package service

import (
	"context"
	"time"

	"github.com/google/uuid"

	"github.com/freight-platform/low-code-service/internal/domain"
	apperrors "github.com/freight-platform/low-code-service/internal/platform/errors"
	"github.com/freight-platform/low-code-service/internal/repository"
)

type AdminFormTemplateRepository interface {
	CreateDraft(ctx context.Context, input repository.CreateDraftInput) (*repository.CreateDraftResult, error)
	ListAdmin(ctx context.Context, filter repository.AdminListFilter) ([]domain.FormTemplateSummary, error)
	GetByID(ctx context.Context, tenantID uuid.UUID, templateID uuid.UUID) (*domain.FormTemplateDetail, error)
	UpdateDraft(ctx context.Context, input repository.UpdateDraftInput) error
	PublishDraft(ctx context.Context, tenantID uuid.UUID, templateID uuid.UUID, audit domain.AuditContext) (*domain.FormTemplateDetail, error)
	ClonePublishedToDraft(ctx context.Context, tenantID uuid.UUID, sourceTemplateID uuid.UUID, audit domain.AuditContext) (*repository.ClonePublishedToDraftResult, error)
	RecordTemplateExport(ctx context.Context, tenantID uuid.UUID, templateID uuid.UUID, detail domain.FormTemplateDetail, audit domain.AuditContext, schemaVersion string) error
	ListByEntityTypeAndCode(ctx context.Context, tenantID uuid.UUID, entityType string, code string) ([]domain.FormTemplateSummary, error)
	RecordTemplateImportPreview(ctx context.Context, tenantID uuid.UUID, entityType string, entityID *uuid.UUID, input domain.TemplateImportPreviewInput, result domain.TemplateImportPreviewResult, audit domain.AuditContext) error
}

type AdminFormTemplateService struct {
	repo AdminFormTemplateRepository
}

func NewAdminFormTemplateService(repo AdminFormTemplateRepository) *AdminFormTemplateService {
	return &AdminFormTemplateService{repo: repo}
}

func (s *AdminFormTemplateService) CreateDraft(
	ctx context.Context,
	tenantID uuid.UUID,
	input domain.DraftFormTemplateInput,
	audit domain.AuditContext,
) (*repository.CreateDraftResult, error) {
	if err := domain.ValidateDraftFormTemplateInput(input); err != nil {
		return nil, err
	}
	return s.repo.CreateDraft(ctx, repository.CreateDraftInput{
		TenantID:    tenantID,
		EntityType:  input.EntityType,
		Code:        input.Code,
		Name:        input.Name,
		Description: input.Description,
		Sections:    input.Sections,
		Audit:       audit,
	})
}

func (s *AdminFormTemplateService) List(
	ctx context.Context,
	tenantID uuid.UUID,
	entityType string,
	status string,
	limit int,
) ([]domain.FormTemplateSummary, error) {
	if err := domain.ValidateEntityType(entityType); err != nil {
		return nil, err
	}
	if err := domain.ValidateTemplateStatusFilter(status); err != nil {
		return nil, err
	}
	if limit <= 0 {
		limit = 50
	}
	if limit > 100 {
		limit = 100
	}
	return s.repo.ListAdmin(ctx, repository.AdminListFilter{
		TenantID:   tenantID,
		EntityType: entityType,
		Status:     status,
		Limit:      limit,
	})
}

func (s *AdminFormTemplateService) GetByID(
	ctx context.Context,
	tenantID uuid.UUID,
	templateID uuid.UUID,
) (*domain.FormTemplateDetail, error) {
	return s.repo.GetByID(ctx, tenantID, templateID)
}

func (s *AdminFormTemplateService) UpdateDraft(
	ctx context.Context,
	tenantID uuid.UUID,
	templateID uuid.UUID,
	input domain.DraftFormTemplateInput,
	audit domain.AuditContext,
) error {
	if err := domain.ValidateDraftFormTemplateInput(input); err != nil {
		return err
	}
	return s.repo.UpdateDraft(ctx, repository.UpdateDraftInput{
		TenantID:    tenantID,
		TemplateID:  templateID,
		Name:        input.Name,
		Description: input.Description,
		Sections:    input.Sections,
		Audit:       audit,
	})
}

func (s *AdminFormTemplateService) PublishDraft(
	ctx context.Context,
	tenantID uuid.UUID,
	templateID uuid.UUID,
	audit domain.AuditContext,
) (*domain.FormTemplateDetail, error) {
	return s.repo.PublishDraft(ctx, tenantID, templateID, audit)
}

func (s *AdminFormTemplateService) ClonePublishedToDraft(
	ctx context.Context,
	tenantID uuid.UUID,
	sourceTemplateID uuid.UUID,
	audit domain.AuditContext,
) (*repository.ClonePublishedToDraftResult, error) {
	return s.repo.ClonePublishedToDraft(ctx, tenantID, sourceTemplateID, audit)
}

func (s *AdminFormTemplateService) Export(
	ctx context.Context,
	tenantID uuid.UUID,
	templateID uuid.UUID,
	audit domain.AuditContext,
	exportedAt time.Time,
) (domain.TemplateExportEnvelope, error) {
	detail, err := s.repo.GetByID(ctx, tenantID, templateID)
	if err != nil {
		return domain.TemplateExportEnvelope{}, err
	}
	if !domain.IsExportableTemplateStatus(detail.Status) {
		return domain.TemplateExportEnvelope{}, apperrors.Validation(
			"template status is not exportable",
			map[string]any{"status": detail.Status},
		)
	}

	envelope, err := domain.BuildTemplateExportEnvelope(
		*detail,
		audit,
		exportedAt,
		domain.DefaultExportEnvironment,
		domain.DefaultExportServiceName,
	)
	if err != nil {
		return domain.TemplateExportEnvelope{}, apperrors.Internal("failed to build export envelope", err)
	}

	if err := s.repo.RecordTemplateExport(ctx, tenantID, templateID, *detail, audit, domain.TemplateExportSchemaVersion); err != nil {
		return domain.TemplateExportEnvelope{}, err
	}

	return envelope, nil
}

func (s *AdminFormTemplateService) ImportPreview(
	ctx context.Context,
	tenantID uuid.UUID,
	rawBody []byte,
	audit domain.AuditContext,
) (domain.TemplateImportPreviewResult, error) {
	input, err := domain.ParseImportPreviewRequest(rawBody)
	if err != nil {
		return domain.TemplateImportPreviewResult{}, err
	}

	draftInput := domain.ExportedTemplateToDraftInput(input.Template, input.TargetCode)
	if err := domain.ValidateImportSystemFields(draftInput, input.AllowSystemFields); err != nil {
		return domain.TemplateImportPreviewResult{}, err
	}
	if err := domain.ValidateDraftFormTemplateInput(draftInput); err != nil {
		return domain.TemplateImportPreviewResult{}, err
	}

	existing, err := s.repo.ListByEntityTypeAndCode(ctx, tenantID, draftInput.EntityType, draftInput.Code)
	if err != nil {
		return domain.TemplateImportPreviewResult{}, err
	}

	var comparisonTemplate *domain.FormTemplateDetail
	if published := domain.SelectComparisonPublishedTemplate(existing); published != nil {
		detail, err := s.repo.GetByID(ctx, tenantID, published.ID)
		if err != nil {
			return domain.TemplateImportPreviewResult{}, err
		}
		comparisonTemplate = detail
	}

	result, err := domain.BuildTemplateImportPreview(input, draftInput, existing, comparisonTemplate)
	if err != nil {
		return domain.TemplateImportPreviewResult{}, err
	}

	entityID := domain.ResolveImportPreviewAuditEntityID(input, existing)
	if err := s.repo.RecordTemplateImportPreview(ctx, tenantID, draftInput.EntityType, entityID, input, result, audit); err != nil {
		return domain.TemplateImportPreviewResult{}, err
	}

	return result, nil
}
