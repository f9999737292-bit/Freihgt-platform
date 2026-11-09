package domain

import (
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"

	apperrors "github.com/freight-platform/document-service/internal/platform/errors"
)

func appCode(err error) apperrors.Code {
	var app *apperrors.AppError
	if !errors.As(err, &app) || app == nil {
		return ""
	}
	return app.Code
}

func TestDecideSigningRecovery(t *testing.T) {
	tenant := uuid.New()
	documentID := uuid.New()
	sourceID := uuid.New()
	now := time.Date(2026, 11, 4, 12, 0, 0, 0, time.UTC)
	source := SigningSession{
		ID: sourceID, TenantID: tenant, DocumentID: documentID,
		Status: SigningSessionStatusExpired, CreatedAt: now,
	}
	input := RecoverSigningInput{TenantID: tenant, SigningSessionID: sourceID, ExpectedVersion: 3}

	if err := DecideSigningRecovery(DocumentStatusSigningInProgress, 3, input, []SigningSession{source}); err != nil {
		t.Fatal(err)
	}

	t.Run("partial history does not block", func(t *testing.T) {
		older := source
		older.ID = uuid.New()
		older.CreatedAt = now.Add(-time.Hour)
		older.CompletedSignersCount = 1
		if err := DecideSigningRecovery(DocumentStatusSigningInProgress, 3, input, []SigningSession{older, source}); err != nil {
			t.Fatal(err)
		}
	})

	t.Run("wrong version", func(t *testing.T) {
		err := DecideSigningRecovery(DocumentStatusSigningInProgress, 4, input, []SigningSession{source})
		if appCode(err) != apperrors.CodeConflict {
			t.Fatalf("code=%s", appCode(err))
		}
	})

	t.Run("not expired", func(t *testing.T) {
		open := source
		open.Status = SigningSessionStatusCreated
		err := DecideSigningRecovery(DocumentStatusSigningInProgress, 3, input, []SigningSession{open})
		if appCode(err) != apperrors.CodeValidation {
			t.Fatalf("code=%s", appCode(err))
		}
	})

	for _, status := range []string{
		DocumentStatusDraft, DocumentStatusReadyForSigning, DocumentStatusSigned,
		DocumentStatusAccepted, DocumentStatusArchived, DocumentStatusCancelled, DocumentStatusSentToOperator,
	} {
		err := DecideSigningRecovery(status, 3, input, []SigningSession{source})
		if appCode(err) != apperrors.CodeValidation {
			t.Fatalf("status %s code=%s", status, appCode(err))
		}
	}

	t.Run("active session", func(t *testing.T) {
		active := source
		active.ID = uuid.New()
		active.Status = SigningSessionStatusCreated
		active.CreatedAt = now.Add(time.Minute)
		err := DecideSigningRecovery(DocumentStatusSigningInProgress, 3, input, []SigningSession{source, active})
		if appCode(err) != apperrors.CodeConflict {
			t.Fatalf("code=%s", appCode(err))
		}
	})

	t.Run("completed session", func(t *testing.T) {
		done := source
		done.ID = uuid.New()
		done.Status = SigningSessionStatusCompleted
		done.CreatedAt = now.Add(-time.Minute)
		err := DecideSigningRecovery(DocumentStatusSigningInProgress, 3, input, []SigningSession{done, source})
		if appCode(err) != apperrors.CodeConflict {
			t.Fatalf("code=%s", appCode(err))
		}
	})

	t.Run("older expired session", func(t *testing.T) {
		newer := source
		newer.ID = uuid.New()
		newer.CreatedAt = now.Add(time.Minute)
		oldInput := input
		oldInput.SigningSessionID = sourceID
		err := DecideSigningRecovery(DocumentStatusSigningInProgress, 3, oldInput, []SigningSession{source, newer})
		if appCode(err) != apperrors.CodeConflict {
			t.Fatalf("code=%s", appCode(err))
		}
	})

	t.Run("foreign tenant", func(t *testing.T) {
		foreign := input
		foreign.TenantID = uuid.New()
		err := DecideSigningRecovery(DocumentStatusSigningInProgress, 3, foreign, []SigningSession{source})
		if appCode(err) != apperrors.CodeNotFound {
			t.Fatalf("code=%s", appCode(err))
		}
	})
}
