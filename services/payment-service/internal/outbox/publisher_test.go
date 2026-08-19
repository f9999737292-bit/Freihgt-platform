package outbox

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"

	"github.com/freight-platform/payment-service/internal/domain"
)

type stubBillingClient struct {
	err error
}

func (s stubBillingClient) SyncRegisterPaid(context.Context, uuid.UUID, uuid.UUID) error {
	return s.err
}

func TestHTTPPublisherRejectsUnsupportedEventType(t *testing.T) {
	t.Parallel()
	pub := NewHTTPPublisher(stubBillingClient{})
	err := pub.Publish(context.Background(), domain.PaymentOutboxEvent{
		EventType: "payment.voided",
		TenantID:  uuid.New(),
	})
	var pubErr *PublishError
	if !errors.As(err, &pubErr) || pubErr.Code != ErrorCodePayloadRejected {
		t.Fatalf("expected payload rejected, got %v", err)
	}
}

func TestClassifyPublishErrorPermanentIntegrity(t *testing.T) {
	t.Parallel()
	got := ClassifyPublishError(&PublishError{Code: ErrorCodeIntegrityViolation, Retryable: false})
	if !IsPermanentPublishError(got.Code) {
		t.Fatalf("integrity violation must be permanent")
	}
}
