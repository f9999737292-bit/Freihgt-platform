package repository

import (
	"context"
	"encoding/json"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/freight-platform/control-tower-read-model-service/internal/domain"
)

type dispositionEvent struct {
	EventID           uuid.UUID
	EventType         string
	OperatingTenantID uuid.UUID
	ExecutionID       uuid.UUID
	SourceStopID      uuid.UUID
	ShipmentID        uuid.UUID
	CargoID           uuid.UUID
	CaseID            uuid.UUID
	ReasonCode        string
	AcceptedQuantity  float64
	RejectedQuantity  float64
	UOM               string
	DispositionType   string
	Status            string
	TargetLocationID  *uuid.UUID
	Sequence          int64
	OccurredAt        time.Time
}

func (r *ExecutionProjectionRepository) applyDisposition(ctx context.Context, payload []byte, meta domain.KafkaRecordMeta, receivedAt time.Time) error {
	event, err := parseDispositionEvent(payload)
	if err != nil {
		return err
	}
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	tag, err := tx.Exec(ctx, `
INSERT INTO control_tower.delivery_disposition_inbox (
  event_id, disposition_case_id, event_type, payload_sha256, processing_outcome, received_at
) VALUES ($1,$2,$3,$4,'applied',$5)
ON CONFLICT (event_id) DO NOTHING
`, event.EventID, event.CaseID, event.EventType, sha256Hex(payload), receivedAt)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return tx.Commit(ctx)
	}
	var revisionBefore *uuid.UUID
	_ = tx.QueryRow(ctx, `SELECT active_revision_id FROM control_tower.execution_projection WHERE execution_id=$1`, event.ExecutionID).Scan(&revisionBefore)
	var current int64
	err = tx.QueryRow(ctx, `
SELECT disposition_sequence FROM control_tower.delivery_disposition_projection WHERE disposition_case_id=$1 FOR UPDATE
`, event.CaseID).Scan(&current)
	if err != nil && err != pgx.ErrNoRows {
		return err
	}
	if err == nil && event.Sequence <= current {
		if _, err := tx.Exec(ctx, `UPDATE control_tower.delivery_disposition_inbox SET processing_outcome='ignored_old' WHERE event_id=$1`, event.EventID); err != nil {
			return err
		}
		return tx.Commit(ctx)
	}
	target := any(nil)
	if event.TargetLocationID != nil {
		target = *event.TargetLocationID
	}
	dispositionType := any(nil)
	if event.DispositionType != "" {
		dispositionType = event.DispositionType
	}
	if _, err := tx.Exec(ctx, `
INSERT INTO control_tower.delivery_disposition_projection (
  disposition_case_id, operating_tenant_id, execution_id, source_execution_stop_id, shipment_id, cargo_id,
  accepted_quantity, rejected_quantity, uom, reason_code, disposition_type, status, target_location_id,
  disposition_sequence, created_at, updated_at
) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$15)
ON CONFLICT (disposition_case_id) DO UPDATE SET
  disposition_type=EXCLUDED.disposition_type,
  status=EXCLUDED.status,
  target_location_id=EXCLUDED.target_location_id,
  accepted_quantity=EXCLUDED.accepted_quantity,
  rejected_quantity=EXCLUDED.rejected_quantity,
  disposition_sequence=EXCLUDED.disposition_sequence,
  updated_at=EXCLUDED.updated_at
WHERE control_tower.delivery_disposition_projection.disposition_sequence < EXCLUDED.disposition_sequence
`, event.CaseID, event.OperatingTenantID, event.ExecutionID, event.SourceStopID, event.ShipmentID, event.CargoID,
		event.AcceptedQuantity, event.RejectedQuantity, event.UOM, event.ReasonCode, dispositionType, event.Status, target,
		event.Sequence, event.OccurredAt); err != nil {
		return err
	}
	var revisionAfter *uuid.UUID
	_ = tx.QueryRow(ctx, `SELECT active_revision_id FROM control_tower.execution_projection WHERE execution_id=$1`, event.ExecutionID).Scan(&revisionAfter)
	if revisionBefore != nil && revisionAfter != nil && *revisionBefore != *revisionAfter {
		return errDispositionRevisionMutated
	}
	return tx.Commit(ctx)
}

var errDispositionRevisionMutated = errString("disposition projection mutated execution revision")

type errString string

func (e errString) Error() string { return string(e) }

func parseDispositionEvent(payload []byte) (dispositionEvent, error) {
	var raw struct {
		EventID           string  `json:"event_id"`
		EventType         string  `json:"event_type"`
		OperatingTenantID string  `json:"operating_tenant_id"`
		ExecutionID       string  `json:"execution_id"`
		SourceStopID      string  `json:"source_execution_stop_id"`
		ShipmentID        string  `json:"shipment_id"`
		CargoID           string  `json:"cargo_id"`
		CaseID            string  `json:"disposition_case_id"`
		ReasonCode        string  `json:"reason_code"`
		Accepted          float64 `json:"accepted_quantity"`
		Rejected          float64 `json:"rejected_quantity"`
		UOM               string  `json:"uom"`
		DispositionType   string  `json:"disposition_type"`
		Status            string  `json:"status"`
		Target            string  `json:"target_location_id"`
		Sequence          int64   `json:"disposition_sequence"`
		OccurredAt        string  `json:"occurred_at"`
	}
	if err := json.Unmarshal(payload, &raw); err != nil {
		return dispositionEvent{}, err
	}
	event := dispositionEvent{
		EventType: raw.EventType, ReasonCode: raw.ReasonCode, AcceptedQuantity: raw.Accepted,
		RejectedQuantity: raw.Rejected, UOM: raw.UOM, DispositionType: raw.DispositionType,
		Status: raw.Status, Sequence: raw.Sequence,
	}
	var err error
	if event.EventID, err = uuid.Parse(raw.EventID); err != nil {
		return dispositionEvent{}, err
	}
	if event.OperatingTenantID, err = uuid.Parse(raw.OperatingTenantID); err != nil {
		return dispositionEvent{}, err
	}
	if event.ExecutionID, err = uuid.Parse(raw.ExecutionID); err != nil {
		return dispositionEvent{}, err
	}
	if event.SourceStopID, err = uuid.Parse(raw.SourceStopID); err != nil {
		return dispositionEvent{}, err
	}
	if event.ShipmentID, err = uuid.Parse(raw.ShipmentID); err != nil {
		return dispositionEvent{}, err
	}
	if event.CargoID, err = uuid.Parse(raw.CargoID); err != nil {
		return dispositionEvent{}, err
	}
	if event.CaseID, err = uuid.Parse(raw.CaseID); err != nil {
		return dispositionEvent{}, err
	}
	if raw.Target != "" {
		id, parseErr := uuid.Parse(raw.Target)
		if parseErr != nil {
			return dispositionEvent{}, parseErr
		}
		event.TargetLocationID = &id
	}
	event.OccurredAt, err = time.Parse(time.RFC3339Nano, raw.OccurredAt)
	if err != nil {
		event.OccurredAt, err = time.Parse(time.RFC3339, raw.OccurredAt)
		if err != nil {
			return dispositionEvent{}, err
		}
	}
	if event.Sequence < 1 {
		return dispositionEvent{}, errString("disposition sequence is required")
	}
	return event, nil
}
