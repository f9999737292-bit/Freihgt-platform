package repository

import (
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	apperrors "github.com/freight-platform/low-code-service/internal/platform/errors"
)

func mapDBError(err error) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, pgx.ErrNoRows) {
		return apperrors.FormTemplateNotFound()
	}
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		if pgErr.Code == "23514" {
			return apperrors.Validation("constraint violation", map[string]any{"detail": pgErr.Message})
		}
	}
	return apperrors.Internal("database error", err)
}
