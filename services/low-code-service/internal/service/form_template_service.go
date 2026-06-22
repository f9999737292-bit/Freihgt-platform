package service

import (
	"context"
	"strings"

	"github.com/google/uuid"

	"github.com/freight-platform/low-code-service/internal/domain"
	apperrors "github.com/freight-platform/low-code-service/internal/platform/errors"
)

type FormTemplateRepository interface {
	ListPublished(ctx context.Context, tenantID uuid.UUID, entityType string) ([]domain.FormTemplateSummary, error)
	ListActivePublished(ctx context.Context, tenantID uuid.UUID, entityType string, code string) ([]domain.FormTemplateSummary, error)
	GetPublishedByID(ctx context.Context, tenantID uuid.UUID, templateID uuid.UUID) (*domain.FormTemplateDetail, error)
	GetPublishedByCode(ctx context.Context, tenantID uuid.UUID, code string) (*domain.FormTemplateDetail, error)
}

type FormTemplateService struct {
	repo FormTemplateRepository
}

func NewFormTemplateService(repo FormTemplateRepository) *FormTemplateService {
	return &FormTemplateService{repo: repo}
}

func (s *FormTemplateService) ListPublished(
	ctx context.Context,
	tenantID uuid.UUID,
	entityType string,
) ([]domain.FormTemplateSummary, error) {
	if err := domain.ValidateEntityType(entityType); err != nil {
		return nil, err
	}
	return s.repo.ListPublished(ctx, tenantID, entityType)
}

func (s *FormTemplateService) GetPublished(
	ctx context.Context,
	tenantID uuid.UUID,
	idOrCode string,
) (*domain.FormTemplateDetail, error) {
	if templateID, err := uuid.Parse(idOrCode); err == nil {
		return s.repo.GetPublishedByID(ctx, tenantID, templateID)
	}
	return s.repo.GetPublishedByCode(ctx, tenantID, idOrCode)
}

func (s *FormTemplateService) ListActivePublished(
	ctx context.Context,
	tenantID uuid.UUID,
	entityType string,
	code string,
) ([]domain.FormTemplateSummary, error) {
	entityType = strings.TrimSpace(entityType)
	if entityType == "" {
		return nil, apperrors.Validation("entity_type is required", map[string]any{"field": "entity_type"})
	}
	if err := domain.ValidateEntityType(entityType); err != nil {
		return nil, err
	}
	return s.repo.ListActivePublished(ctx, tenantID, entityType, strings.TrimSpace(code))
}
