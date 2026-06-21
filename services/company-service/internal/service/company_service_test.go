package service

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/freight-platform/company-service/internal/domain"
)

type mockCompanyStore struct {
	createFn  func(ctx context.Context, in domain.CreateCompanyInput) (*domain.Company, error)
	getByIDFn func(ctx context.Context, id uuid.UUID) (*domain.Company, error)
	listFn    func(ctx context.Context, filter domain.ListCompaniesFilter) ([]domain.Company, int, error)
}

func (m *mockCompanyStore) Create(ctx context.Context, in domain.CreateCompanyInput) (*domain.Company, error) {
	return m.createFn(ctx, in)
}

func (m *mockCompanyStore) GetByID(ctx context.Context, id uuid.UUID) (*domain.Company, error) {
	if m.getByIDFn != nil {
		return m.getByIDFn(ctx, id)
	}
	return nil, nil
}

func (m *mockCompanyStore) List(ctx context.Context, filter domain.ListCompaniesFilter) ([]domain.Company, int, error) {
	return m.listFn(ctx, filter)
}

func (m *mockCompanyStore) Update(context.Context, uuid.UUID, domain.UpdateCompanyInput) (*domain.Company, error) {
	return nil, nil
}

func (m *mockCompanyStore) SoftDelete(context.Context, uuid.UUID) error {
	return nil
}

func TestCompanyServiceCreateValidation(t *testing.T) {
	t.Parallel()

	svc := NewCompanyService(&mockCompanyStore{
		createFn: func(_ context.Context, in domain.CreateCompanyInput) (*domain.Company, error) {
			return &domain.Company{
				ID:              uuid.New(),
				TenantID:        in.TenantID,
				LegalName:       in.LegalName,
				CompanyType:     in.CompanyType,
				CountryCode:     in.CountryCode,
				PreferredLocale: in.PreferredLocale,
				Status:          domain.StatusActive,
				CreatedAt:       time.Now().UTC(),
				UpdatedAt:       time.Now().UTC(),
				Version:         1,
			}, nil
		},
		listFn: func(context.Context, domain.ListCompaniesFilter) ([]domain.Company, int, error) {
			return nil, 0, nil
		},
	})

	tenantID := uuid.MustParse("11111111-1111-1111-1111-111111111111")
	company, err := svc.Create(context.Background(), domain.CreateCompanyInput{
		TenantID:        tenantID,
		LegalName:       "ООО Ромашка",
		CompanyType:     "SHIPPER",
		CountryCode:     "RU",
		PreferredLocale: "ru-RU",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if company.LegalName != "ООО Ромашка" {
		t.Fatalf("unexpected legal name: %s", company.LegalName)
	}

	_, err = svc.Create(context.Background(), domain.CreateCompanyInput{
		LegalName:       "ООО Ромашка",
		CompanyType:     "SHIPPER",
		PreferredLocale: "ru-RU",
	})
	if err == nil {
		t.Fatalf("expected validation error")
	}
}

func TestCompanyServiceListDefaults(t *testing.T) {
	t.Parallel()

	var captured domain.ListCompaniesFilter
	svc := NewCompanyService(&mockCompanyStore{
		createFn: func(context.Context, domain.CreateCompanyInput) (*domain.Company, error) {
			return nil, nil
		},
		listFn: func(_ context.Context, filter domain.ListCompaniesFilter) ([]domain.Company, int, error) {
			captured = filter
			return []domain.Company{}, 0, nil
		},
	})

	tenantID := uuid.MustParse("11111111-1111-1111-1111-111111111111")
	_, _, err := svc.List(context.Background(), domain.ListCompaniesFilter{TenantID: tenantID})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if captured.Limit != 20 {
		t.Fatalf("expected default limit 20, got %d", captured.Limit)
	}
}
