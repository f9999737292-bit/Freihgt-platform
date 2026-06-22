package service

import (
	"context"

	"github.com/google/uuid"

	"github.com/freight-platform/low-code-service/internal/domain"
	"github.com/freight-platform/low-code-service/internal/repository"
)

type AdminFormTemplateRepository interface {
	CreateDraft(ctx context.Context, input repository.CreateDraftInput) (*repository.CreateDraftResult, error)
	ListAdmin(ctx context.Context, filter repository.AdminListFilter) ([]domain.FormTemplateSummary, error)
	GetByID(ctx context.Context, tenantID uuid.UUID, templateID uuid.UUID) (*domain.FormTemplateDetail, error)
	UpdateDraft(ctx context.Context, input repository.UpdateDraftInput) error
	PublishDraft(ctx context.Context, tenantID uuid.UUID, templateID uuid.UUID, audit domain.AuditContext) (*domain.FormTemplateDetail, error)
	ClonePublishedToDraft(ctx context.Context, tenantID uuid.UUID, sourceTemplateID uuid.UUID, audit domain.AuditContext) (*repository.ClonePublishedToDraftResult, error)
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
