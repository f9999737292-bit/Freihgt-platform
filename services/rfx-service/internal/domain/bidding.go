package domain

import (
	"time"

	apperrors "github.com/freight-platform/rfx-service/internal/platform/errors"
)

// BiddingContext captures server-side tender bidding state.
type BiddingContext struct {
	Status           string
	RfxType          string
	ResponseDeadline *time.Time
	BiddingClosedAt  *time.Time
}

func ValidateBiddingOpen(ctx BiddingContext) error {
	if ctx.Status != RfxStatusPublished && ctx.Status != RfxStatusResponsesOpen {
		return apperrors.Validation("bidding is not open for this tender", map[string]any{"field": "status", "status": ctx.Status})
	}
	if deadlinePassed(ctx.ResponseDeadline, ctx.BiddingClosedAt) {
		return apperrors.Validation("bidding deadline has passed", map[string]any{"field": "response_deadline"})
	}
	return nil
}

func deadlinePassed(responseDeadline, biddingClosedAt *time.Time) bool {
	now := time.Now().UTC()
	if biddingClosedAt != nil && !biddingClosedAt.After(now) {
		return true
	}
	if responseDeadline != nil && !responseDeadline.After(now) {
		return true
	}
	return false
}
