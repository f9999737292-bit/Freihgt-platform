package outbox

import (
	"errors"
	"testing"

	"github.com/freight-platform/shipment-service/internal/config"
)

func TestNewPublisherDisabledReturnsNil(t *testing.T) {
	t.Parallel()
	publisher, err := NewPublisher(config.OutboxConfig{Enabled: false})
	if err != nil {
		t.Fatalf("err=%v", err)
	}
	if publisher != nil {
		t.Fatal("disabled publisher must be nil")
	}
}

func TestNewPublisherEnabledWithoutTransportFails(t *testing.T) {
	t.Parallel()
	_, err := NewPublisher(config.OutboxConfig{Enabled: true, Transport: ""})
	if err == nil {
		t.Fatal("expected configuration error")
	}
}

func TestNewPublisherUnsupportedTransportFails(t *testing.T) {
	t.Parallel()
	_, err := NewPublisher(config.OutboxConfig{Enabled: true, Transport: "nats"})
	if err == nil {
		t.Fatal("expected unsupported transport error")
	}
}

func TestClassifyPublishErrorPermanentPayload(t *testing.T) {
	t.Parallel()
	err := &PublishError{Code: ErrorCodePayloadRejected, Retryable: false}
	got := ClassifyPublishError(err)
	if got.Code != ErrorCodePayloadRejected || !IsPermanentPublishError(got.Code) {
		t.Fatalf("got=%+v", got)
	}
}

func TestClassifyPublishErrorUnknownRetryable(t *testing.T) {
	t.Parallel()
	got := ClassifyPublishError(errors.New("network down"))
	if got.Code != ErrorCodeUnknownPublishError || !got.Retryable {
		t.Fatalf("got=%+v", got)
	}
}
