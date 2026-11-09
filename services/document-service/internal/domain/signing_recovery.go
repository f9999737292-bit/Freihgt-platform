package domain

import (
	"github.com/google/uuid"

	apperrors "github.com/freight-platform/document-service/internal/platform/errors"
)

type RecoverSigningInput struct {
	TenantID         uuid.UUID
	SigningSessionID uuid.UUID
	ExpectedVersion  int
}

func ValidateRecoverSigningInput(in RecoverSigningInput) error {
	if in.TenantID == uuid.Nil {
		return apperrors.Validation("tenant_id is required", map[string]any{"field": "tenant_id"})
	}
	if in.SigningSessionID == uuid.Nil {
		return apperrors.Validation("signing_session_id is required", map[string]any{"field": "signing_session_id"})
	}
	if in.ExpectedVersion < 1 {
		return apperrors.Validation("expected_version is required", map[string]any{"field": "expected_version"})
	}
	return nil
}

// DecideSigningRecovery allows one transition: SIGNING_IN_PROGRESS to
// READY_FOR_SIGNING, for the latest persisted EXPIRED session of that document.
// It does not expire a session from a timestamp and does not create a session.
func DecideSigningRecovery(documentStatus string, actualVersion int, in RecoverSigningInput, sessions []SigningSession) error {
	if err := ValidateRecoverSigningInput(in); err != nil {
		return err
	}
	var source *SigningSession
	for i := range sessions {
		if sessions[i].ID == in.SigningSessionID {
			source = &sessions[i]
			break
		}
	}
	if source == nil {
		return apperrors.NotFound("signing session not found")
	}
	if source.TenantID != in.TenantID {
		return apperrors.NotFound("signing session not found")
	}
	if source.Status != SigningSessionStatusExpired {
		return apperrors.Validation("signing session is not expired", map[string]any{
			"field":  "status",
			"status": source.Status,
		})
	}
	if actualVersion != in.ExpectedVersion {
		return apperrors.Conflict("document version does not match expected_version", map[string]any{
			"field":            "expected_version",
			"expected_version": in.ExpectedVersion,
			"version":          actualVersion,
		})
	}
	if documentStatus != DocumentStatusSigningInProgress {
		return apperrors.Validation("document cannot be recovered for signing from its current status", map[string]any{
			"field":  "document_status",
			"status": documentStatus,
		})
	}
	for i := range sessions {
		session := &sessions[i]
		if session.Status == SigningSessionStatusCompleted {
			return apperrors.Conflict("completed signing attempt blocks recovery", nil)
		}
		if session.Status == SigningSessionStatusCreated || session.Status == SigningSessionStatusInProgress {
			return apperrors.Conflict("active signing session blocks recovery", nil)
		}
		if session.ID != source.ID && signingSessionIsNewer(session, source) {
			return apperrors.Conflict("signing session is not the current attempt", nil)
		}
	}
	return nil
}

func signingSessionIsNewer(candidate, source *SigningSession) bool {
	if candidate.CreatedAt.After(source.CreatedAt) {
		return true
	}
	if source.CreatedAt.After(candidate.CreatedAt) {
		return false
	}
	return candidate.ID.String() > source.ID.String()
}
