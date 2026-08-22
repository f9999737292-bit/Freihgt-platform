package outbox

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/freight-platform/payment-service/internal/domain"
)

type FreightCostPublisher interface {
	PublishSourceEvent(ctx context.Context, tenantID string, payload json.RawMessage) error
}

type RouterPublisher struct {
	billing BillingSyncClient
	cost    FreightCostPublisher
}

func NewRouterPublisher(billing BillingSyncClient, cost FreightCostPublisher) EventPublisher {
	if billing == nil && cost == nil {
		return nil
	}
	return &RouterPublisher{billing: billing, cost: cost}
}

func (p *RouterPublisher) Publish(ctx context.Context, event domain.PaymentOutboxEvent) error {
	switch event.EventType {
	case domain.PaymentEventObligationPaid:
		if p.billing == nil {
			return &PublishError{Code: ErrorCodeConfigurationError, Retryable: false, Err: fmt.Errorf("billing sync client is not configured")}
		}
		return NewHTTPPublisher(p.billing).Publish(ctx, event)
	case domain.PaymentEventObligationPaidSnapshot:
		if p.cost == nil {
			return &PublishError{Code: ErrorCodeConfigurationError, Retryable: false, Err: fmt.Errorf("freight cost client is not configured")}
		}
		if err := p.cost.PublishSourceEvent(ctx, event.TenantID.String(), event.Payload); err != nil {
			return classifyHTTPError(err)
		}
		return nil
	default:
		return &PublishError{Code: ErrorCodePayloadRejected, Retryable: false, Err: fmt.Errorf("unsupported event_type %q", event.EventType)}
	}
}
