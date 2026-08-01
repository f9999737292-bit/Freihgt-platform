package outbox

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/freight-platform/shipment-service/internal/config"
	"github.com/freight-platform/shipment-service/internal/domain"
)

func NewPublisher(cfg config.OutboxConfig) (EventPublisher, error) {
	if !cfg.Enabled {
		return nil, nil
	}
	transport := strings.ToLower(strings.TrimSpace(cfg.Transport))
	if transport == "" {
		return nil, fmt.Errorf("SHIPMENT_OUTBOX_ENABLED=true requires SHIPMENT_OUTBOX_TRANSPORT")
	}
	switch transport {
	case "kafka":
		return NewKafkaPublisher(cfg.Kafka, NewRealClock())
	default:
		return nil, fmt.Errorf("unsupported SHIPMENT_OUTBOX_TRANSPORT %q", cfg.Transport)
	}
}

type OutboxRepository interface {
	ClaimPendingForPublisher(
		ctx context.Context,
		workerID string,
		batchSize int,
		now time.Time,
		leaseTimeout time.Duration,
	) ([]domain.ShipmentOutboxEvent, error)
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
