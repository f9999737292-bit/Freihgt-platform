package repository

import (
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	apperrors "github.com/freight-platform/freight-cost-service/internal/platform/errors"
)

func mapDBError(err error) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, pgx.ErrNoRows) {
		return apperrors.NotFound("record not found")
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

func isUniqueViolation(err error, constraint string) bool {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == "23505" {
		return constraint == "" || pgErr.ConstraintName == constraint
	}
	return false
}
