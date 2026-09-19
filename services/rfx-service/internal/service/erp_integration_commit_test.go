package service

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"

	"github.com/freight-platform/rfx-service/internal/domain"
	apperrors "github.com/freight-platform/rfx-service/internal/platform/errors"
	"github.com/freight-platform/rfx-service/internal/repository"
)

func TestResolveCreateCommitAfterTxDoesNotInventSuccess(t *testing.T) {
	t.Parallel()
	s := &ErpIntegrationService{}
	scope := repository.IdempotencyScope{
		TenantID:               uuid.New(),
		IntegrationPrincipalID: uuid.New(),
		OwnerKind:              domain.OwnerKindIntegrationPrincipal,
		Operation:              domain.ERPBuyerCreateCommitOperation,
	}

	replay, err := s.resolveCreateCommitAfterTx(context.Background(), scope, "key", "hash", repository.ErrIdempotencyRecordActive)
	if replay != nil {
		t.Fatal("missing winner record must not become a successful replay")
	}
	var appErr *apperrors.AppError
	if !errors.As(err, &appErr) || appErr.Code != apperrors.CodeInternal {
		t.Fatalf("expected internal error after missing winner, got %v", err)
	}

	original := apperrors.Conflict("analysis expired", map[string]any{"machine_code": "analysis_expired"})
	replay, err = s.resolveCreateCommitAfterTx(context.Background(), scope, "key", "hash", original)
	if replay != nil {
		t.Fatal("absent idempotency record must not convert an unrelated failure into success")
	}
	if !errors.Is(err, original) && err != original {
		t.Fatalf("expected original error when winner is absent, got %v", err)
	}
}
