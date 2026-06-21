package repository

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/freight-platform/company-service/internal/domain"
	apperrors "github.com/freight-platform/company-service/internal/platform/errors"
)

type CompanyRepository struct {
	pool *pgxpool.Pool
}

func NewCompanyRepository(pool *pgxpool.Pool) *CompanyRepository {
	return &CompanyRepository{pool: pool}
}

func (r *CompanyRepository) Create(ctx context.Context, in domain.CreateCompanyInput) (*domain.Company, error) {
	var result *domain.Company
	err := measureDB("company_repository", "create_company", func() error {
		const query = `
		INSERT INTO core.companies (
			tenant_id, legal_name, short_name, legal_name_en, legal_name_zh,
			company_type, tax_id, registration_number, country_code, preferred_locale, status
		) VALUES (
			$1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11
		)
		RETURNING
			id, tenant_id, legal_name, short_name, legal_name_en, legal_name_zh,
			company_type, tax_id, registration_number, country_code, preferred_locale,
			status, created_at, created_by, updated_at, updated_by, deleted_at, version
	`

		row := r.pool.QueryRow(ctx, query,
			in.TenantID,
			strings.TrimSpace(in.LegalName),
			domain.OptionalString(in.ShortName),
			domain.OptionalString(in.LegalNameEN),
			domain.OptionalString(in.LegalNameZH),
			in.CompanyType,
			domain.OptionalString(in.TaxID),
			domain.OptionalString(in.RegistrationNumber),
			domain.NormalizeCountryCode(in.CountryCode),
			domain.NormalizePreferredLocale(in.PreferredLocale),
			domain.StatusActive,
		)

		company, err := scanCompany(row)
		if err != nil {
			return mapDBError(err)
		}
		result = company
		return nil
	})
	return result, err
}

func (r *CompanyRepository) GetByID(ctx context.Context, id uuid.UUID) (*domain.Company, error) {
	var result *domain.Company
	err := measureDB("company_repository", "get_company", func() error {
		const query = `
		SELECT
			id, tenant_id, legal_name, short_name, legal_name_en, legal_name_zh,
			company_type, tax_id, registration_number, country_code, preferred_locale,
			status, created_at, created_by, updated_at, updated_by, deleted_at, version
		FROM core.companies
		WHERE id = $1 AND deleted_at IS NULL
	`

		row := r.pool.QueryRow(ctx, query, id)
		company, err := scanCompany(row)
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return apperrors.NotFound("company not found")
			}
			return mapDBError(err)
		}
		result = company
		return nil
	})
	return result, err
}

func (r *CompanyRepository) List(ctx context.Context, filter domain.ListCompaniesFilter) ([]domain.Company, int, error) {
	var items []domain.Company
	var total int
	err := measureDB("company_repository", "list_companies", func() error {
		where := strings.Builder{}
		where.WriteString("FROM core.companies WHERE tenant_id = $1 AND deleted_at IS NULL")
		args := []any{filter.TenantID}
		argIdx := 2

		if filter.CompanyType != nil {
			where.WriteString(fmt.Sprintf(" AND company_type = $%d", argIdx))
			args = append(args, *filter.CompanyType)
			argIdx++
		}
		if filter.Status != nil {
			where.WriteString(fmt.Sprintf(" AND status = $%d", argIdx))
			args = append(args, *filter.Status)
			argIdx++
		}
		if filter.Search != nil && strings.TrimSpace(*filter.Search) != "" {
			where.WriteString(fmt.Sprintf(` AND (
			legal_name ILIKE '%%' || $%d || '%%' OR
			short_name ILIKE '%%' || $%d || '%%' OR
			tax_id ILIKE '%%' || $%d || '%%'
		)`, argIdx, argIdx, argIdx))
			args = append(args, strings.TrimSpace(*filter.Search))
			argIdx++
		}

		countQuery := "SELECT COUNT(*) " + where.String()
		if err := r.pool.QueryRow(ctx, countQuery, args...).Scan(&total); err != nil {
			return mapDBError(err)
		}

		listQuery := `
		SELECT
			id, tenant_id, legal_name, short_name, legal_name_en, legal_name_zh,
			company_type, tax_id, registration_number, country_code, preferred_locale,
			status, created_at, created_by, updated_at, updated_by, deleted_at, version
	` + where.String() + fmt.Sprintf(" ORDER BY created_at DESC LIMIT $%d OFFSET $%d", argIdx, argIdx+1)
		args = append(args, filter.Limit, filter.Offset)

		rows, err := r.pool.Query(ctx, listQuery, args...)
		if err != nil {
			return mapDBError(err)
		}
		defer rows.Close()

		companies := make([]domain.Company, 0)
		for rows.Next() {
			company, err := scanCompany(rows)
			if err != nil {
				return mapDBError(err)
			}
			companies = append(companies, *company)
		}
		if err := rows.Err(); err != nil {
			return mapDBError(err)
		}

		items = companies
		return nil
	})
	return items, total, err
}

func (r *CompanyRepository) Update(ctx context.Context, id uuid.UUID, in domain.UpdateCompanyInput) (*domain.Company, error) {
	var result *domain.Company
	err := measureDB("company_repository", "update_company", func() error {
		current, err := r.GetByID(ctx, id)
		if err != nil {
			return err
		}

		legalName := current.LegalName
		if in.LegalName != nil {
			legalName = strings.TrimSpace(*in.LegalName)
		}
		shortName := current.ShortName
		if in.ShortName != nil {
			shortName = stringPtr(strings.TrimSpace(*in.ShortName))
		}
		legalNameEN := current.LegalNameEN
		if in.LegalNameEN != nil {
			legalNameEN = stringPtr(strings.TrimSpace(*in.LegalNameEN))
		}
		legalNameZH := current.LegalNameZH
		if in.LegalNameZH != nil {
			legalNameZH = stringPtr(strings.TrimSpace(*in.LegalNameZH))
		}
		companyType := current.CompanyType
		if in.CompanyType != nil {
			companyType = *in.CompanyType
		}
		taxID := current.TaxID
		if in.TaxID != nil {
			taxID = stringPtr(strings.TrimSpace(*in.TaxID))
		}
		registrationNumber := current.RegistrationNumber
		if in.RegistrationNumber != nil {
			registrationNumber = stringPtr(strings.TrimSpace(*in.RegistrationNumber))
		}
		countryCode := current.CountryCode
		if in.CountryCode != nil {
			countryCode = domain.NormalizeCountryCode(*in.CountryCode)
		}
		preferredLocale := current.PreferredLocale
		if in.PreferredLocale != nil {
			preferredLocale = domain.NormalizePreferredLocale(*in.PreferredLocale)
		}
		status := current.Status
		if in.Status != nil {
			status = strings.TrimSpace(*in.Status)
		}

		const query = `
		UPDATE core.companies SET
			legal_name = $2,
			short_name = $3,
			legal_name_en = $4,
			legal_name_zh = $5,
			company_type = $6,
			tax_id = $7,
			registration_number = $8,
			country_code = $9,
			preferred_locale = $10,
			status = $11,
			updated_at = now(),
			version = version + 1
		WHERE id = $1 AND deleted_at IS NULL AND version = $12
		RETURNING
			id, tenant_id, legal_name, short_name, legal_name_en, legal_name_zh,
			company_type, tax_id, registration_number, country_code, preferred_locale,
			status, created_at, created_by, updated_at, updated_by, deleted_at, version
	`

		row := r.pool.QueryRow(ctx, query,
			id,
			legalName,
			nullableString(shortName),
			nullableString(legalNameEN),
			nullableString(legalNameZH),
			companyType,
			nullableString(taxID),
			nullableString(registrationNumber),
			countryCode,
			preferredLocale,
			status,
			current.Version,
		)

		company, err := scanCompany(row)
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return apperrors.Conflict("company was updated by another request", map[string]any{"field": "version"})
			}
			return mapDBError(err)
		}
		result = company
		return nil
	})
	return result, err
}

func (r *CompanyRepository) SoftDelete(ctx context.Context, id uuid.UUID) error {
	return measureDB("company_repository", "delete_company", func() error {
		const query = `
		UPDATE core.companies
		SET deleted_at = now(), status = $2, updated_at = now(), version = version + 1
		WHERE id = $1 AND deleted_at IS NULL
	`

		tag, err := r.pool.Exec(ctx, query, id, domain.StatusDeleted)
		if err != nil {
			return mapDBError(err)
		}
		if tag.RowsAffected() == 0 {
			return apperrors.NotFound("company not found")
		}
		return nil
	})
}

type scannable interface {
	Scan(dest ...any) error
}

func scanCompany(row scannable) (*domain.Company, error) {
	var company domain.Company
	err := row.Scan(
		&company.ID,
		&company.TenantID,
		&company.LegalName,
		&company.ShortName,
		&company.LegalNameEN,
		&company.LegalNameZH,
		&company.CompanyType,
		&company.TaxID,
		&company.RegistrationNumber,
		&company.CountryCode,
		&company.PreferredLocale,
		&company.Status,
		&company.CreatedAt,
		&company.CreatedBy,
		&company.UpdatedAt,
		&company.UpdatedBy,
		&company.DeletedAt,
		&company.Version,
	)
	if err != nil {
		return nil, err
	}
	return &company, nil
}

func mapDBError(err error) error {
	if err == nil {
		return nil
	}
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		if pgErr.Code == "23505" {
			return apperrors.Conflict("record already exists", map[string]any{"detail": pgErr.ConstraintName})
		}
		if pgErr.Code == "23503" {
			return apperrors.Validation("referenced record does not exist", map[string]any{"detail": pgErr.Message})
		}
		if pgErr.Code == "23514" {
			return apperrors.Validation("constraint violation", map[string]any{"detail": pgErr.Message})
		}
	}
	return apperrors.Internal("database error", err)
}

func stringPtr(value string) *string {
	if value == "" {
		return nil
	}
	return &value
}

func nullableString(value *string) any {
	if value == nil || strings.TrimSpace(*value) == "" {
		return nil
	}
	return strings.TrimSpace(*value)
}
