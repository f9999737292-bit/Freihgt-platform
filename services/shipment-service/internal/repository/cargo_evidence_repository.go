package repository

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/freight-platform/shipment-service/internal/domain"
)

func insertCargoEvidence(ctx context.Context, tx pgx.Tx, shipment *domain.Shipment, intent *domain.CargoEvidenceIntent) error {
	if intent == nil || shipment == nil || shipment.CargoID == nil || *shipment.CargoID == uuid.Nil {
		return nil
	}
	if shipment.Version <= 0 || intent.State == "" || intent.Source == "" || intent.SourceEventType == "" || intent.OccurredAt.IsZero() {
		return fmt.Errorf("cargo evidence intent is incomplete")
	}
	actorID := intent.ActorID
	driverID := intent.DriverID
	_, err := tx.Exec(ctx, `
		INSERT INTO transport.shipment_cargo_execution_evidence (
			id, tenant_id, shipment_id, shipment_version, cargo_id, state, state_version,
			source, source_event_type, actor_id, driver_id, occurred_at
		) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12)`,
		uuid.New(), shipment.TenantID, shipment.ID, shipment.Version, *shipment.CargoID,
		intent.State, shipment.Version, intent.Source, intent.SourceEventType,
		actorID, driverID, intent.OccurredAt.UTC(),
	)
	return mapDBError(err)
}

// insertExecutionCargoEvidence appends one row to the existing cargo evidence ledger and returns its id.
// state_version stays unique per cargo. It may move ahead of shipment.version when the command does not change shipment status, so stop progress does not consume the shipment aggregate version.
func insertExecutionCargoEvidence(ctx context.Context, tx pgx.Tx, shipment *domain.Shipment, intent *domain.CargoEvidenceIntent) (uuid.UUID, error) {
	if intent == nil || shipment == nil || shipment.CargoID == nil || *shipment.CargoID == uuid.Nil {
		return uuid.Nil, fmt.Errorf("cargo evidence intent is incomplete")
	}
	if shipment.Version <= 0 || intent.State == "" || intent.Source == "" || intent.SourceEventType == "" || intent.OccurredAt.IsZero() {
		return uuid.Nil, fmt.Errorf("cargo evidence intent is incomplete")
	}
	var maxVersion int
	if err := tx.QueryRow(ctx, `
		SELECT COALESCE(MAX(state_version), 0)
		FROM transport.shipment_cargo_execution_evidence
		WHERE shipment_id = $1 AND cargo_id = $2
	`, shipment.ID, *shipment.CargoID).Scan(&maxVersion); err != nil {
		return uuid.Nil, mapDBError(err)
	}
	stateVersion := shipment.Version
	if stateVersion <= maxVersion {
		stateVersion = maxVersion + 1
	}
	evidenceID := uuid.New()
	_, err := tx.Exec(ctx, `
		INSERT INTO transport.shipment_cargo_execution_evidence (
			id, tenant_id, shipment_id, shipment_version, cargo_id, state, state_version,
			source, source_event_type, actor_id, driver_id, occurred_at
		) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12)`,
		evidenceID, shipment.TenantID, shipment.ID, shipment.Version, *shipment.CargoID,
		intent.State, stateVersion, intent.Source, intent.SourceEventType,
		intent.ActorID, intent.DriverID, intent.OccurredAt.UTC(),
	)
	if err != nil {
		return uuid.Nil, mapDBError(err)
	}
	return evidenceID, nil
}

func (r *ShipmentRepository) ListCargoExecutionEvidence(ctx context.Context, tenantID, shipmentID uuid.UUID) ([]domain.ShipmentCargoEvidence, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT id, tenant_id, shipment_id, shipment_version, cargo_id, state, state_version,
			source, source_event_type, actor_id, driver_id, occurred_at, recorded_at
		FROM transport.shipment_cargo_execution_evidence
		WHERE tenant_id = $1 AND shipment_id = $2
		ORDER BY state_version DESC, occurred_at DESC, id DESC`, tenantID, shipmentID)
	if err != nil {
		return nil, mapDBError(err)
	}
	defer rows.Close()
	var out []domain.ShipmentCargoEvidence
	for rows.Next() {
		var row domain.ShipmentCargoEvidence
		if err := rows.Scan(
			&row.ID, &row.TenantID, &row.ShipmentID, &row.ShipmentVersion, &row.CargoID,
			&row.State, &row.StateVersion, &row.Source, &row.SourceEventType,
			&row.ActorID, &row.DriverID, &row.OccurredAt, &row.RecordedAt,
		); err != nil {
			return nil, mapDBError(err)
		}
		out = append(out, row)
	}
	if err := rows.Err(); err != nil {
		return nil, mapDBError(err)
	}
	if out == nil {
		out = []domain.ShipmentCargoEvidence{}
	}
	return out, nil
}
