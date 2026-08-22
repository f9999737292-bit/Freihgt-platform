package repository

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/freight-platform/billing-register-service/internal/domain"
)

type FreightCostOutboxEmitter struct {
	repo *FreightCostOutboxRepository
}

func NewFreightCostOutboxEmitter(repo *FreightCostOutboxRepository) *FreightCostOutboxEmitter {
	if repo == nil {
		return nil
	}
	return &FreightCostOutboxEmitter{repo: repo}
}

func (e *FreightCostOutboxEmitter) EmitSettlementSnapshotsTx(
	ctx context.Context,
	tx pgx.Tx,
	settlement *domain.FreightSettlement,
	openDisputeCount int,
	eventTypes []string,
	occurredAt time.Time,
) error {
	if e == nil || e.repo == nil || settlement == nil {
		return nil
	}
	ids, payloads, err := domain.BuildSettlementSnapshotPayloads(eventTypes, settlement, openDisputeCount, occurredAt)
	if err != nil {
		return err
	}
	for i, eventType := range eventTypes {
		if err := e.repo.InsertTx(ctx, tx, ids[i], settlement.TenantID,
			domain.AggregateFreightSettlement, settlement.ID, int64(settlement.Version),
			eventType, domain.FreightCostOutboxSchemaVersion, payloads[i], occurredAt); err != nil {
			return err
		}
	}
	return nil
}

func (e *FreightCostOutboxEmitter) EmitAllSettlementSnapshotsTx(
	ctx context.Context,
	tx pgx.Tx,
	settlement *domain.FreightSettlement,
	openDisputeCount int,
	occurredAt time.Time,
) error {
	return e.EmitSettlementSnapshotsTx(ctx, tx, settlement, openDisputeCount, domain.AllSettlementSnapshotEventTypes, occurredAt)
}

func (e *FreightCostOutboxEmitter) EmitBillingLinkSnapshotTx(
	ctx context.Context,
	tx pgx.Tx,
	settlement *domain.FreightSettlement,
	linkState string,
	amountExVAT *string,
	registerID, registerItemID *uuid.UUID,
	occurredAt time.Time,
) error {
	if e == nil || e.repo == nil || settlement == nil {
		return nil
	}
	eventID, payload, err := domain.BuildBillingLinkSnapshotPayload(settlement, linkState, amountExVAT, registerID, registerItemID, occurredAt)
	if err != nil {
		return err
	}
	return e.repo.InsertTx(ctx, tx, eventID, settlement.TenantID,
		domain.AggregateFreightSettlementBillingLink, settlement.ID, settlement.BillingLinkRevision,
		domain.EventBillingRegisterSettlementBillingLink, domain.FreightCostOutboxSchemaVersion, payload, occurredAt)
}

func (e *FreightCostOutboxEmitter) EmitPayableSnapshotTx(
	ctx context.Context,
	tx pgx.Tx,
	register *domain.BillingRegister,
	occurredAt time.Time,
) error {
	if e == nil || e.repo == nil || register == nil {
		return nil
	}
	eventID, payload, err := domain.BuildPayableSnapshotPayload(register, occurredAt)
	if err != nil {
		return err
	}
	return e.repo.InsertTx(ctx, tx, eventID, register.TenantID,
		domain.AggregateBillingRegister, register.ID, int64(register.Version),
		domain.EventBillingRegisterPayableSnapshot, domain.FreightCostOutboxSchemaVersion, payload, occurredAt)
}

func countOpenDisputesAt(ctx context.Context, tx pgx.Tx, settlementID, tenantID uuid.UUID) (int, error) {
	return countOpenDisputes(ctx, tx, settlementID, tenantID)
}

func emitSettlementSnapshotsAfterMutation(
	ctx context.Context,
	tx pgx.Tx,
	emitter *FreightCostOutboxEmitter,
	settlement *domain.FreightSettlement,
	eventTypes []string,
) error {
	if emitter == nil || settlement == nil {
		return nil
	}
	openCount, err := countOpenDisputesAt(ctx, tx, settlement.ID, settlement.TenantID)
	if err != nil {
		return err
	}
	return emitter.EmitSettlementSnapshotsTx(ctx, tx, settlement, openCount, eventTypes, time.Now().UTC())
}

func emitAllSettlementSnapshotsAfterMutation(
	ctx context.Context,
	tx pgx.Tx,
	emitter *FreightCostOutboxEmitter,
	settlement *domain.FreightSettlement,
) error {
	return emitSettlementSnapshotsAfterMutation(ctx, tx, emitter, settlement, domain.AllSettlementSnapshotEventTypes)
}
