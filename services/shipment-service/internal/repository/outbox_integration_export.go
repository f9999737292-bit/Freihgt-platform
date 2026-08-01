//go:build integration

package repository

import (
	"context"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/freight-platform/shipment-service/internal/domain"
)

type IntegrationHistoryWrite struct {
	TenantID        uuid.UUID
	ShipmentID      uuid.UUID
	ShipmentVersion int
	FromStatus      *string
	ToStatus        string
	Transition      domain.StatusTransitionContext
}

func InsertStatusHistoryAndOutboxIntegration(ctx context.Context, tx pgx.Tx, write IntegrationHistoryWrite) error {
	internal := statusHistoryWriteFromTransition(
		write.TenantID,
		write.ShipmentID,
		write.ShipmentVersion,
		write.FromStatus,
		write.ToStatus,
		write.Transition,
	)
	return insertStatusHistoryAndOutbox(ctx, tx, internal)
}

func InsertStatusHistoryIntegration(ctx context.Context, tx pgx.Tx, write IntegrationHistoryWrite) (domain.ShipmentStatusHistory, error) {
	internal := statusHistoryWriteFromTransition(
		write.TenantID,
		write.ShipmentID,
		write.ShipmentVersion,
		write.FromStatus,
		write.ToStatus,
		write.Transition,
	)
	return insertStatusHistoryRowReturning(ctx, tx, internal)
}

func InsertOutboxRowIntegration(ctx context.Context, tx pgx.Tx, event domain.ShipmentOutboxEvent) error {
	return insertOutboxRow(ctx, tx, event)
}
