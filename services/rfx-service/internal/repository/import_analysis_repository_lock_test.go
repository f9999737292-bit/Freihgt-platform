package repository

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	apperrors "github.com/freight-platform/rfx-service/internal/platform/errors"
)

type lockTx struct {
	pgx.Tx
}

func (lockTx) QueryRow(context.Context, string, ...any) pgx.Row {
	return nil
}

func TestLockImportAnalysisForUpdateRequiresTransaction(t *testing.T) {
	t.Parallel()
	repo := NewImportAnalysisRepository(nil)
	_, err := repo.LockImportAnalysisForUpdate(context.Background(), uuid.New(), uuid.New())
	if err == nil {
		t.Fatal("expected error without transaction")
	}
	appErr, ok := err.(*apperrors.AppError)
	if !ok || appErr.Code != apperrors.CodeInternal {
		t.Fatalf("expected internal error, got %v", err)
	}
}
