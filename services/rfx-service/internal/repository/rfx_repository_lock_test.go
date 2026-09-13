package repository

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	apperrors "github.com/freight-platform/rfx-service/internal/platform/errors"
)

type lotLockTx struct {
	pgx.Tx
}

func (lotLockTx) Query(context.Context, string, ...any) (pgx.Rows, error) {
	return nil, nil
}

func TestLockRfxEventForLotMutationRequiresTransaction(t *testing.T) {
	t.Parallel()
	repo := NewRfxRepository(nil)
	err := repo.LockRfxEventForLotMutation(context.Background(), uuid.New(), uuid.New())
	if err == nil {
		t.Fatal("expected error without transaction")
	}
	appErr, ok := err.(*apperrors.AppError)
	if !ok || appErr.Code != apperrors.CodeInternal {
		t.Fatalf("expected internal error, got %v", err)
	}
}

func TestLockEventLotsForUpdateRequiresTransaction(t *testing.T) {
	t.Parallel()
	repo := NewRfxRepository(nil)
	_, err := repo.LockEventLotsForUpdate(context.Background(), uuid.New(), uuid.New())
	if err == nil {
		t.Fatal("expected error without transaction")
	}
	appErr, ok := err.(*apperrors.AppError)
	if !ok || appErr.Code != apperrors.CodeInternal {
		t.Fatalf("expected internal error, got %v", err)
	}
}
