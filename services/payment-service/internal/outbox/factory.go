package outbox

import (
	"context"
	"time"

	"github.com/google/uuid"

	"github.com/freight-platform/payment-service/internal/domain"
)

type OutboxRepository interface {
	ClaimPendingForPublisher(
		ctx context.Context,
		workerID string,
		batchSize int,
		now time.Time,
		leaseTimeout time.Duration,
	) ([]domain.PaymentOutboxEvent, error)
	MarkPublished(ctx context.Context, eventID uuid.UUID, workerID string, publishedAt time.Time) error
	ReleaseWithRetry(ctx context.Context, eventID uuid.UUID, workerID string, availableAt time.Time, errorCode string) error
	MarkFailed(ctx context.Context, eventID uuid.UUID, workerID string, errorCode string) error
	OutboxGaugeSnapshot(ctx context.Context, now time.Time) (pending int64, failed int64, oldestPendingAgeSeconds float64, err error)
}

type Clock interface {
	Now() time.Time
}

type realClock struct{}

func (realClock) Now() time.Time { return time.Now().UTC() }

func NewRealClock() Clock { return realClock{} }

func NewPublisher(billing BillingSyncClient, cost FreightCostPublisher) EventPublisher {
	return NewRouterPublisher(billing, cost)
}
