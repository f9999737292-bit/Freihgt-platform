package service

import (
	"context"

	"github.com/google/uuid"

	"github.com/freight-platform/company-service/internal/domain"
	apperrors "github.com/freight-platform/company-service/internal/platform/errors"
)

type CompanyStore interface {
	Create(ctx context.Context, in domain.CreateCompanyInput) (*domain.Company, error)
	GetByID(ctx context.Context, id uuid.UUID) (*domain.Company, error)
	List(ctx context.Context, filter domain.ListCompaniesFilter) ([]domain.Company, int, error)
	Update(ctx context.Context, id uuid.UUID, in domain.UpdateCompanyInput) (*domain.Company, error)
	SoftDelete(ctx context.Context, id uuid.UUID) error
}

type CompanyService struct {
	repo CompanyStore
}

func NewCompanyService(repo CompanyStore) *CompanyService {
	return &CompanyService{repo: repo}
}

func (s *CompanyService) Create(ctx context.Context, in domain.CreateCompanyInput) (*domain.Company, error) {
	in.CountryCode = domain.NormalizeCountryCode(in.CountryCode)
	in.PreferredLocale = domain.NormalizePreferredLocale(in.PreferredLocale)
	if err := domain.ValidateCreateInput(in); err != nil {
		return nil, err
	}
	return s.repo.Create(ctx, in)
}

func (s *CompanyService) GetByID(ctx context.Context, id uuid.UUID) (*domain.Company, error) {
	if id == uuid.Nil {
		return nil, apperrors.Validation("id is required", map[string]any{"field": "id"})
	}
	return s.repo.GetByID(ctx, id)
}

func (s *CompanyService) List(ctx context.Context, filter domain.ListCompaniesFilter) ([]domain.Company, int, error) {
	if filter.Limit == 0 {
		filter.Limit = 20
	}
	if err := domain.ValidateListFilter(filter); err != nil {
		return nil, 0, err
	}
	return s.repo.List(ctx, filter)
}

func (s *CompanyService) Update(ctx context.Context, id uuid.UUID, in domain.UpdateCompanyInput) (*domain.Company, error) {
	if id == uuid.Nil {
		return nil, apperrors.Validation("id is required", map[string]any{"field": "id"})
	}
	if err := domain.ValidateUpdateInput(in); err != nil {
		return nil, err
	}
	return s.repo.Update(ctx, id, in)
}

func (s *CompanyService) Delete(ctx context.Context, id uuid.UUID) error {
	if id == uuid.Nil {
		return apperrors.Validation("id is required", map[string]any{"field": "id"})
	}
	return s.repo.SoftDelete(ctx, id)
}
