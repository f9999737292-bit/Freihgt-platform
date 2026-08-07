//go:build integration

package outbox

import (
	"context"

	"github.com/freight-platform/shipment-service/internal/domain"
)

func (w *Worker) ProcessEventForIntegration(ctx context.Context, event domain.ShipmentOutboxEvent) {
	w.publishOne(ctx, event)
}
