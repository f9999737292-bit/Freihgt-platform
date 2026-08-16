package domain

import (
	"testing"
	"time"

	apperrors "github.com/freight-platform/rfx-service/internal/platform/errors"
)

func TestValidateResponseDeadlineOpen(t *testing.T) {
	t.Parallel()
	future := time.Now().UTC().Add(time.Hour)
	if err := ValidateResponseDeadlineOpen(&future, time.Now().UTC()); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	past := time.Now().UTC().Add(-time.Hour)
	if err := ValidateResponseDeadlineOpen(&past, time.Now().UTC()); err == nil {
		t.Fatal("expected conflict for past deadline")
	}
}

func TestValidateSubmissionBeforeDeadlineNil(t *testing.T) {
	t.Parallel()
	if err := ValidateSubmissionBeforeDeadline(nil, time.Now().UTC()); err != nil {
		t.Fatalf("nil deadline should pass, got %v", err)
	}
}

func TestValidateSubmissionBeforeDeadlinePast(t *testing.T) {
	t.Parallel()
	past := time.Now().UTC().Add(-time.Minute)
	err := ValidateSubmissionBeforeDeadline(&past, time.Now().UTC())
	if err == nil {
		t.Fatal("expected conflict")
	}
	if appErr, ok := err.(*apperrors.AppError); !ok || appErr.Code != apperrors.CodeConflict {
		t.Fatalf("expected conflict, got %v", err)
	}
}
