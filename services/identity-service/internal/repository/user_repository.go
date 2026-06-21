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

	"github.com/freight-platform/identity-service/internal/domain"
	apperrors "github.com/freight-platform/identity-service/internal/platform/errors"
)

type UserRepository struct {
	pool *pgxpool.Pool
}

func NewUserRepository(pool *pgxpool.Pool) *UserRepository {
	return &UserRepository{pool: pool}
}

func (r *UserRepository) Create(ctx context.Context, in domain.CreateUserInput, passwordHash string) (*domain.User, error) {
	var result *domain.User
	err := measureDB("user_repository", "create_user", func() error {
		const query = `
		INSERT INTO core.users (
			tenant_id, email, phone, password_hash, full_name, preferred_locale, status
		) VALUES ($1, $2, $3, $4, $5, $6, $7)
		RETURNING
			id, tenant_id, email, phone, password_hash, full_name, preferred_locale, status,
			last_login_at, created_at, updated_at, deleted_at, version
	`

		row := r.pool.QueryRow(ctx, query,
			in.TenantID,
			domain.NormalizeEmail(in.Email),
			domain.OptionalString(in.Phone),
			passwordHash,
			strings.TrimSpace(in.FullName),
			domain.NormalizePreferredLocale(in.PreferredLocale),
			domain.UserStatusActive,
		)

		user, err := scanUser(row)
		if err != nil {
			return mapDBError(err)
		}
		result = user
		return nil
	})
	return result, err
}

func (r *UserRepository) GetByID(ctx context.Context, id uuid.UUID) (*domain.User, error) {
	var result *domain.User
	err := measureDB("user_repository", "get_user", func() error {
		const query = `
		SELECT
			id, tenant_id, email, phone, password_hash, full_name, preferred_locale, status,
			last_login_at, created_at, updated_at, deleted_at, version
		FROM core.users
		WHERE id = $1 AND deleted_at IS NULL
	`

		row := r.pool.QueryRow(ctx, query, id)
		user, err := scanUser(row)
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return apperrors.NotFound("user not found")
			}
			return mapDBError(err)
		}
		result = user
		return nil
	})
	return result, err
}

func (r *UserRepository) GetByTenantAndEmail(ctx context.Context, tenantID uuid.UUID, email string) (*domain.User, error) {
	var result *domain.User
	err := measureDB("user_repository", "find_user_by_email", func() error {
		const query = `
		SELECT
			id, tenant_id, email, phone, password_hash, full_name, preferred_locale, status,
			last_login_at, created_at, updated_at, deleted_at, version
		FROM core.users
		WHERE tenant_id = $1 AND email = $2 AND deleted_at IS NULL
	`

		row := r.pool.QueryRow(ctx, query, tenantID, domain.NormalizeEmail(email))
		user, err := scanUser(row)
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return apperrors.NotFound("user not found")
			}
			return mapDBError(err)
		}
		result = user
		return nil
	})
	return result, err
}

func (r *UserRepository) List(ctx context.Context, filter domain.ListUsersFilter) ([]domain.User, int, error) {
	var users []domain.User
	var total int
	err := measureDB("user_repository", "list_users", func() error {
		where := strings.Builder{}
		where.WriteString("FROM core.users WHERE tenant_id = $1 AND deleted_at IS NULL")
		args := []any{filter.TenantID}
		argIdx := 2

		if filter.Status != nil {
			where.WriteString(fmt.Sprintf(" AND status = $%d", argIdx))
			args = append(args, *filter.Status)
			argIdx++
		}
		if filter.Search != nil && strings.TrimSpace(*filter.Search) != "" {
			where.WriteString(fmt.Sprintf(` AND (
			email ILIKE '%%' || $%d || '%%' OR
			phone ILIKE '%%' || $%d || '%%' OR
			full_name ILIKE '%%' || $%d || '%%'
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
			id, tenant_id, email, phone, password_hash, full_name, preferred_locale, status,
			last_login_at, created_at, updated_at, deleted_at, version
	` + where.String() + fmt.Sprintf(" ORDER BY created_at DESC LIMIT $%d OFFSET $%d", argIdx, argIdx+1)
		args = append(args, filter.Limit, filter.Offset)

		rows, err := r.pool.Query(ctx, listQuery, args...)
		if err != nil {
			return mapDBError(err)
		}
		defer rows.Close()

		users = make([]domain.User, 0)
		for rows.Next() {
			user, err := scanUser(rows)
			if err != nil {
				return mapDBError(err)
			}
			users = append(users, *user)
		}
		if err := rows.Err(); err != nil {
			return mapDBError(err)
		}

		return nil
	})
	return users, total, err
}

func (r *UserRepository) Update(ctx context.Context, id uuid.UUID, in domain.UpdateUserInput) (*domain.User, error) {
	var result *domain.User
	err := measureDB("user_repository", "update_user", func() error {
		current, err := r.GetByID(ctx, id)
		if err != nil {
			return err
		}

		phone := current.Phone
		if in.Phone != nil {
			phone = stringPtr(strings.TrimSpace(*in.Phone))
		}
		fullName := current.FullName
		if in.FullName != nil {
			fullName = strings.TrimSpace(*in.FullName)
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
		UPDATE core.users SET
			phone = $2,
			full_name = $3,
			preferred_locale = $4,
			status = $5,
			updated_at = now(),
			version = version + 1
		WHERE id = $1 AND deleted_at IS NULL AND version = $6
		RETURNING
			id, tenant_id, email, phone, password_hash, full_name, preferred_locale, status,
			last_login_at, created_at, updated_at, deleted_at, version
	`

		row := r.pool.QueryRow(ctx, query,
			id,
			nullableString(phone),
			fullName,
			preferredLocale,
			status,
			current.Version,
		)

		user, err := scanUser(row)
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return apperrors.Conflict("user was updated by another request", map[string]any{"field": "version"})
			}
			return mapDBError(err)
		}
		result = user
		return nil
	})
	return result, err
}

func (r *UserRepository) SoftDelete(ctx context.Context, id uuid.UUID) error {
	return measureDB("user_repository", "delete_user", func() error {
		const query = `
		UPDATE core.users
		SET deleted_at = now(), status = $2, updated_at = now(), version = version + 1
		WHERE id = $1 AND deleted_at IS NULL
	`

		tag, err := r.pool.Exec(ctx, query, id, domain.UserStatusDeleted)
		if err != nil {
			return mapDBError(err)
		}
		if tag.RowsAffected() == 0 {
			return apperrors.NotFound("user not found")
		}
		return nil
	})
}

func (r *UserRepository) UpdateLastLogin(ctx context.Context, id uuid.UUID) error {
	const query = `
		UPDATE core.users
		SET last_login_at = now(), updated_at = now()
		WHERE id = $1 AND deleted_at IS NULL
	`
	_, err := r.pool.Exec(ctx, query, id)
	return mapDBError(err)
}

type scannable interface {
	Scan(dest ...any) error
}

func scanUser(row scannable) (*domain.User, error) {
	var user domain.User
	err := row.Scan(
		&user.ID,
		&user.TenantID,
		&user.Email,
		&user.Phone,
		&user.PasswordHash,
		&user.FullName,
		&user.PreferredLocale,
		&user.Status,
		&user.LastLoginAt,
		&user.CreatedAt,
		&user.UpdatedAt,
		&user.DeletedAt,
		&user.Version,
	)
	if err != nil {
		return nil, err
	}
	return &user, nil
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
