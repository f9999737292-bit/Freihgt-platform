//go:build integration

package outbox

import (
	"context"

	"github.com/freight-platform/payment-service/internal/domain"
)

// ProcessEventForIntegration publishes a single claimed event outside the poll loop.
// Exported only under the integration build tag for PostgreSQL integration tests.
func (w *Worker) ProcessEventForIntegration(ctx context.Context, event domain.PaymentOutboxEvent) {
	w.publishOne(ctx, event)
}
