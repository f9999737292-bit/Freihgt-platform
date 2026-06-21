package repository

import (
	"errors"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgconn"

	apperrors "github.com/freight-platform/shipment-service/internal/platform/errors"
)

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

func optionalString(value *string) any {
	if value == nil {
		return nil
	}
	trimmed := strings.TrimSpace(*value)
	if trimmed == "" {
		return nil
	}
	return trimmed
}

func optionalUUID(v *uuid.UUID) any {
	if v == nil {
		return nil
	}
	return *v
}

func optionalFloat(v *float64) any {
	if v == nil {
		return nil
	}
	return *v
}

func optionalTime(v *time.Time) any {
	if v == nil {
		return nil
	}
	return *v
}
