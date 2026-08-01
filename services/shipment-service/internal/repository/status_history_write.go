package repository

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/freight-platform/shipment-service/internal/domain"
)

type statusHistoryWrite struct {
	tenantID        uuid.UUID
	shipmentID      uuid.UUID
	shipmentVersion int
	fromStatus      *string
	toStatus        string
	reasonCode      *string
	source          string
	actorType       string
	actorID         *uuid.UUID
	correlationID   *string
	occurredAt      time.Time
}

func statusHistoryWriteFromTransition(
	tenantID, shipmentID uuid.UUID,
	shipmentVersion int,
	fromStatus *string,
	toStatus string,
	transition domain.StatusTransitionContext,
) statusHistoryWrite {
	source := string(transition.Source)
	actorType := string(transition.ActorType)
	occurredAt := transition.OccurredAt
	if occurredAt.IsZero() {
		occurredAt = time.Now().UTC()
	}
	return statusHistoryWrite{
		tenantID:        tenantID,
		shipmentID:      shipmentID,
		shipmentVersion: shipmentVersion,
		fromStatus:      fromStatus,
		toStatus:        toStatus,
		reasonCode:      transition.ReasonCode,
		source:          source,
		actorType:       actorType,
		actorID:         transition.ActorID,
		correlationID:   transition.CorrelationID,
		occurredAt:      occurredAt.UTC(),
	}
}

func insertStatusHistoryRow(ctx context.Context, tx pgx.Tx, write statusHistoryWrite) error {
	_, err := tx.Exec(ctx, insertStatusHistoryQuery,
		write.tenantID,
		write.shipmentID,
		write.shipmentVersion,
		write.fromStatus,
		write.toStatus,
		optionalString(write.reasonCode),
		write.source,
		write.actorType,
		optionalUUID(write.actorID),
		optionalString(write.correlationID),
		write.occurredAt,
	)
	return mapDBError(err)
}

func shouldRecordStatusHistory(fromStatus *string, toStatus string) bool {
	if fromStatus == nil {
		return true
	}
	return *fromStatus != toStatus
}

func stringPtr(value string) *string {
	if value == "" {
		return nil
	}
	v := value
	return &v
}
