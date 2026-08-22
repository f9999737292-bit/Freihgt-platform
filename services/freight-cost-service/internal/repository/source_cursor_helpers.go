package repository

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"

	"github.com/freight-platform/freight-cost-service/internal/domain"
	apperrors "github.com/freight-platform/freight-cost-service/internal/platform/errors"
)

func (r *SourceCursorRepository) GetOrZero(ctx context.Context, tx pgx.Tx, key domain.SourceCursorKey) (*domain.SourceCursor, error) {
	cursor, err := r.Get(ctx, tx, key)
	if err != nil {
		var appErr *apperrors.AppError
		if errors.As(err, &appErr) && appErr.Code == apperrors.CodeNotFound {
			return &domain.SourceCursor{
				SourceCursorKey:    key,
				LastSourceRevision: 0,
			}, nil
		}
		return nil, err
	}
	return cursor, nil
}
