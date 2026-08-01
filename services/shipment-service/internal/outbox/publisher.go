package outbox

import (
	"context"
	"errors"

	"github.com/freight-platform/shipment-service/internal/domain"
)

type EventPublisher interface {
	Publish(ctx context.Context, event domain.ShipmentOutboxEvent) error
}

const (
	ErrorCodeTransientNetwork     = "TRANSIENT_NETWORK"
	ErrorCodeTransientTimeout     = "TRANSIENT_TIMEOUT"
	ErrorCodeBrokerUnavailable    = "BROKER_UNAVAILABLE"
	ErrorCodePayloadRejected      = "PAYLOAD_REJECTED"
	ErrorCodeConfigurationError   = "CONFIGURATION_ERROR"
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
	if errors.Is(err, domain.ErrOutboxPublishStateConflict) {
		return &PublishError{Code: ErrorCodePublishStateConflict, Retryable: false, Err: err}
	}
	return &PublishError{Code: ErrorCodeUnknownPublishError, Retryable: true, Err: err}
}

func IsPermanentPublishError(code string) bool {
	switch code {
	case ErrorCodePayloadRejected, ErrorCodeConfigurationError:
		return true
	default:
		return false
	}
}
