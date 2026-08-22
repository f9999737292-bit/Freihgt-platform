package outbox

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/freight-platform/billing-register-service/internal/domain"
)

const (
	ErrorCodeTransientNetwork     = "TRANSIENT_NETWORK"
	ErrorCodeTransientTimeout     = "TRANSIENT_TIMEOUT"
	ErrorCodeFreightCostUnavailable = "FREIGHT_COST_UNAVAILABLE"
	ErrorCodePayloadRejected      = "PAYLOAD_REJECTED"
	ErrorCodeConfigurationError   = "CONFIGURATION_ERROR"
	ErrorCodeIntegrityViolation   = "INTEGRITY_VIOLATION"
	ErrorCodeUnknownPublishError  = "UNKNOWN_PUBLISH_ERROR"
	ErrorCodePublishStateConflict = "PUBLISH_STATE_CONFLICT"
)

type PublishError struct {
	Code      string
	Retryable bool
	Err       error
}

func (e *PublishError) Error() string {
	if e.Err != nil {
		return e.Code + ": " + e.Err.Error()
	}
	return e.Code
}

func (e *PublishError) Unwrap() error { return e.Err }

type FreightCostPublisher interface {
	PublishSourceEvent(ctx context.Context, tenantID string, payload json.RawMessage) error
}

type EventPublisher interface {
	Publish(ctx context.Context, event domain.FreightCostOutboxEvent) error
}

type RouterPublisher struct {
	cost FreightCostPublisher
}

func NewRouterPublisher(cost FreightCostPublisher) EventPublisher {
	if cost == nil {
		return nil
	}
	return &RouterPublisher{cost: cost}
}

func (p *RouterPublisher) Publish(ctx context.Context, event domain.FreightCostOutboxEvent) error {
	if p == nil || p.cost == nil {
		return &PublishError{Code: ErrorCodeConfigurationError, Retryable: false, Err: fmt.Errorf("freight cost client is not configured")}
	}
	if err := p.cost.PublishSourceEvent(ctx, event.TenantID.String(), event.Payload); err != nil {
		return classifyHTTPError(err)
	}
	return nil
}

func classifyHTTPError(err error) *PublishError {
	if err == nil {
		return nil
	}
	var publishErr *PublishError
	if errors.As(err, &publishErr) {
		return publishErr
	}
	msg := strings.ToLower(err.Error())
	if errors.Is(err, context.DeadlineExceeded) || errors.Is(err, context.Canceled) {
		return &PublishError{Code: ErrorCodeTransientTimeout, Retryable: true, Err: err}
	}
	var netErr net.Error
	if errors.As(err, &netErr) && netErr.Timeout() {
		return &PublishError{Code: ErrorCodeTransientTimeout, Retryable: true, Err: err}
	}
	if errors.As(err, &netErr) {
		return &PublishError{Code: ErrorCodeTransientNetwork, Retryable: true, Err: err}
	}
	if strings.Contains(msg, "status=5") {
		return &PublishError{Code: ErrorCodeFreightCostUnavailable, Retryable: true, Err: err}
	}
	if strings.Contains(msg, "status=409") || strings.Contains(msg, "status=422") {
		return &PublishError{Code: ErrorCodeIntegrityViolation, Retryable: false, Err: err}
	}
	if strings.Contains(msg, "status=4") {
		return &PublishError{Code: ErrorCodePayloadRejected, Retryable: false, Err: err}
	}
	if strings.Contains(msg, "not configured") {
		return &PublishError{Code: ErrorCodeConfigurationError, Retryable: false, Err: err}
	}
	return &PublishError{Code: ErrorCodeUnknownPublishError, Retryable: true, Err: err}
}

func ClassifyPublishError(err error) *PublishError {
	if err == nil {
		return nil
	}
	var publishErr *PublishError
	if errors.As(err, &publishErr) {
		return publishErr
	}
	if errors.Is(err, context.DeadlineExceeded) || errors.Is(err, context.Canceled) {
		return &PublishError{Code: ErrorCodeTransientTimeout, Retryable: true, Err: err}
	}
	if errors.Is(err, domain.ErrFreightCostOutboxPublishStateConflict) {
		return &PublishError{Code: ErrorCodePublishStateConflict, Retryable: false, Err: err}
	}
	return &PublishError{Code: ErrorCodeUnknownPublishError, Retryable: true, Err: err}
}

func IsPermanentPublishError(code string) bool {
	switch code {
	case ErrorCodePayloadRejected, ErrorCodeConfigurationError, ErrorCodeIntegrityViolation:
		return true
	default:
		return false
	}
}

type OutboxRepository interface {
	ClaimPendingForPublisher(
		ctx context.Context,
		workerID string,
		batchSize int,
		now time.Time,
		leaseTimeout time.Duration,
	) ([]domain.FreightCostOutboxEvent, error)
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

func NextRetryAvailableAt(attempt int, now time.Time) time.Time {
	switch attempt {
	case 1:
		return now.Add(5 * time.Second)
	case 2:
		return now.Add(15 * time.Second)
	case 3:
		return now.Add(60 * time.Second)
	case 4:
		return now.Add(5 * time.Minute)
	default:
		return now.Add(5 * time.Minute)
	}
}
