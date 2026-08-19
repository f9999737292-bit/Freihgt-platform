package outbox

import (
	"context"
	"errors"
	"fmt"
	"net"
	"strings"

	"github.com/google/uuid"

	"github.com/freight-platform/payment-service/internal/domain"
	paymentservice "github.com/freight-platform/payment-service/internal/service"
)

const (
	ErrorCodeTransientNetwork     = "TRANSIENT_NETWORK"
	ErrorCodeTransientTimeout     = "TRANSIENT_TIMEOUT"
	ErrorCodeBillingUnavailable   = "BILLING_UNAVAILABLE"
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

type BillingSyncClient interface {
	SyncRegisterPaid(ctx context.Context, tenantID, registerID uuid.UUID) error
}

type EventPublisher interface {
	Publish(ctx context.Context, event domain.PaymentOutboxEvent) error
}

type HTTPPublisher struct {
	client BillingSyncClient
}

func NewHTTPPublisher(client BillingSyncClient) *HTTPPublisher {
	return &HTTPPublisher{client: client}
}

func (p *HTTPPublisher) Publish(ctx context.Context, event domain.PaymentOutboxEvent) error {
	if p == nil || p.client == nil {
		return &PublishError{Code: ErrorCodeConfigurationError, Retryable: false, Err: errors.New("billing sync client is not configured")}
	}
	if event.EventType != domain.PaymentEventObligationPaid {
		return &PublishError{Code: ErrorCodePayloadRejected, Retryable: false, Err: fmt.Errorf("unsupported event_type %q", event.EventType)}
	}
	payload, err := domain.ParseObligationPaidOutboxPayload(event.Payload)
	if err != nil {
		return &PublishError{Code: ErrorCodePayloadRejected, Retryable: false, Err: err}
	}
	if payload.TenantID != event.TenantID.String() {
		return &PublishError{Code: ErrorCodeIntegrityViolation, Retryable: false, Err: errors.New("payload tenant_id does not match event tenant_id")}
	}
	if payload.ObligationID != event.AggregateID.String() {
		return &PublishError{Code: ErrorCodeIntegrityViolation, Retryable: false, Err: errors.New("payload obligation_id does not match aggregate_id")}
	}
	registerID, err := uuid.Parse(payload.RegisterID)
	if err != nil {
		return &PublishError{Code: ErrorCodePayloadRejected, Retryable: false, Err: fmt.Errorf("invalid register_id: %w", err)}
	}
	if err := p.client.SyncRegisterPaid(ctx, event.TenantID, registerID); err != nil {
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
	var httpErr *paymentservice.BillingSyncHTTPError
	if errors.As(err, &httpErr) {
		switch {
		case httpErr.StatusCode >= 500:
			return &PublishError{Code: ErrorCodeBillingUnavailable, Retryable: true, Err: err}
		case httpErr.StatusCode == 409:
			// Billing sync-paid returns 409 when canonical obligation preconditions fail.
			return &PublishError{Code: ErrorCodeIntegrityViolation, Retryable: false, Err: err}
		case httpErr.StatusCode == 422:
			return &PublishError{Code: ErrorCodeIntegrityViolation, Retryable: false, Err: err}
		case httpErr.StatusCode >= 400:
			return &PublishError{Code: ErrorCodePayloadRejected, Retryable: false, Err: err}
		}
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
		return &PublishError{Code: ErrorCodeBillingUnavailable, Retryable: true, Err: err}
	}
	if strings.Contains(msg, "status=409") || strings.Contains(msg, "status=422") {
		if strings.Contains(msg, "not paid") || strings.Contains(msg, "integrity") || strings.Contains(msg, "outstanding") {
			return &PublishError{Code: ErrorCodeIntegrityViolation, Retryable: false, Err: err}
		}
	}
	if strings.Contains(msg, "status=4") && !strings.Contains(msg, "status=409") {
		return &PublishError{Code: ErrorCodePayloadRejected, Retryable: false, Err: err}
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
	if errors.Is(err, domain.ErrOutboxPublishStateConflict) {
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
