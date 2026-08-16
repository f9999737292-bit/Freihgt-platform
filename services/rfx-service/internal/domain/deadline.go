package domain

import (
	"time"

	apperrors "github.com/freight-platform/rfx-service/internal/platform/errors"
)

func ValidateResponseDeadlineOpen(deadline *time.Time, now time.Time) error {
	if deadline == nil {
		return nil
	}
	if !now.Before(deadline.UTC()) {
		return apperrors.Conflict("response deadline has passed", map[string]any{"field": "response_deadline"})
	}
	return nil
}

func ValidateSubmissionBeforeDeadline(deadline *time.Time, now time.Time) error {
	if deadline == nil {
		return nil
	}
	if !now.Before(deadline.UTC()) {
		return apperrors.Conflict("submission is not allowed after response deadline", map[string]any{"field": "response_deadline"})
	}
	return nil
}
