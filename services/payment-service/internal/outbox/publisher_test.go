package outbox

import (
	"context"
	"errors"
	"net"
	"testing"

	"github.com/google/uuid"

	"github.com/freight-platform/payment-service/internal/domain"
	paymentservice "github.com/freight-platform/payment-service/internal/service"
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

func TestClassifyHTTPErrorTypedBillingSync(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name      string
		err       error
		code      string
		retryable bool
	}{
		{
			name:      "HTTP_5XX_RETRYABLE",
			err:       &paymentservice.BillingSyncHTTPError{StatusCode: 503, Body: "unavailable"},
			code:      ErrorCodeBillingUnavailable,
			retryable: true,
		},
		{
			name:      "HTTP_4XX_PERMANENT",
			err:       &paymentservice.BillingSyncHTTPError{StatusCode: 400, Body: "bad request"},
			code:      ErrorCodePayloadRejected,
			retryable: false,
		},
		{
			name:      "INTEGRITY_409_PERMANENT",
			err:       &paymentservice.BillingSyncHTTPError{StatusCode: 409, Body: "not paid"},
			code:      ErrorCodeIntegrityViolation,
			retryable: false,
		},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := classifyHTTPError(tc.err)
			if got.Code != tc.code || got.Retryable != tc.retryable {
				t.Fatalf("got code=%s retryable=%v want code=%s retryable=%v", got.Code, got.Retryable, tc.code, tc.retryable)
			}
		})
	}
}

func TestClassifyHTTPErrorTimeoutRetryable(t *testing.T) {
	t.Parallel()
	got := classifyHTTPError(context.DeadlineExceeded)
	if got.Code != ErrorCodeTransientTimeout || !got.Retryable {
		t.Fatalf("HTTP_TIMEOUT_RETRYABLE=FAIL got=%+v", got)
	}
	var netErr net.Error = timeoutError{}
	got = classifyHTTPError(netErr)
	if got.Code != ErrorCodeTransientTimeout || !got.Retryable {
		t.Fatalf("network timeout must be retryable, got=%+v", got)
	}
}

type timeoutError struct{}

func (timeoutError) Error() string   { return "timeout" }
func (timeoutError) Timeout() bool   { return true }
func (timeoutError) Temporary() bool { return true }
