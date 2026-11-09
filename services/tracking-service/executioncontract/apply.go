// Package executioncontract exposes ExecutionConsumer.Apply to the shipment
// contract test. Go does not allow that test to import tracking internal
// packages. The tracking server does not import this package.
package executioncontract

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/freight-platform/tracking-service/internal/repository"
	"github.com/freight-platform/tracking-service/internal/service"
)

// ApplyPayload parses outbox bytes and calls ExecutionConsumer.Apply.
// contextBaseURL is the shipment tracking-context HTTP endpoint.
func ApplyPayload(ctx context.Context, pool *pgxpool.Pool, contextBaseURL, token string, payload []byte) error {
	consumer := service.NewExecutionConsumer(
		repository.NewExecutionTrackingRepository(pool),
		service.NewShipmentContextClient(contextBaseURL, token),
	)
	return consumer.Apply(ctx, payload)
}

// ApplyPayloadWithContextError calls Apply with a context source that fails
// before the tracking transaction starts.
func ApplyPayloadWithContextError(ctx context.Context, pool *pgxpool.Pool, payload []byte) error {
	consumer := service.NewExecutionConsumer(repository.NewExecutionTrackingRepository(pool), brokenContext{})
	return consumer.Apply(ctx, payload)
}

type brokenContext struct{}

func (brokenContext) Get(context.Context, uuid.UUID, uuid.UUID, uuid.UUID) (service.StopContext, error) {
	return service.StopContext{}, errors.New("synthetic context failure")
}
