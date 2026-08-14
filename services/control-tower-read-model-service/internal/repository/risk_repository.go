package repository

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/freight-platform/control-tower-read-model-service/internal/domain"
	apperrors "github.com/freight-platform/control-tower-read-model-service/internal/platform/errors"
)

type RiskRepository struct {
	pool *pgxpool.Pool
}

func NewRiskRepository(pool *pgxpool.Pool) *RiskRepository {
	return &RiskRepository{pool: pool}
}

type RiskListFilter struct {
	Level             string
	Status            string
	PredictedType     string
	ShipmentID        string
	MitigatingOnly    bool
	NonMitigatingOnly bool
	ActiveOnly        bool
}

func (r *RiskRepository) SyncEvaluations(
	ctx context.Context,
	tenantID uuid.UUID,
	evaluations []domain.SyncRiskEvaluation,
	materializations []domain.MaterializeRiskInput,
	clears []domain.ClearRiskInput,
) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	activeKeys := map[string]struct{}{}
	for _, eval := range evaluations {
		activeKeys[eval.RiskKey] = struct{}{}
		if err := r.upsertEvaluation(ctx, tx, tenantID, eval); err != nil {
			return err
		}
	}

	for _, item := range materializations {
		if err := r.materializeRisk(ctx, tx, tenantID, item); err != nil {
			return err
		}
	}

	for _, item := range clears {
		if err := r.clearRisk(ctx, tx, tenantID, item); err != nil {
			return err
		}
	}

	if err := r.autoClearMissing(ctx, tx, tenantID, activeKeys); err != nil {
		return err
	}

	return tx.Commit(ctx)
}

func (r *RiskRepository) upsertEvaluation(
	ctx context.Context,
	tx pgx.Tx,
	tenantID uuid.UUID,
	eval domain.SyncRiskEvaluation,
) error {
	shipmentID, err := uuid.Parse(eval.ShipmentID)
	if err != nil {
		return fmt.Errorf("invalid shipment id: %w", err)
	}

	var existing domain.ShipmentRisk
	err = tx.QueryRow(ctx, `
		SELECT id, score, risk_level, status, predicted_exception_type, version
		FROM control_tower.shipment_risk
		WHERE tenant_id = $1 AND risk_key = $2
		FOR UPDATE
	`, tenantID, eval.RiskKey).Scan(
		&existing.ID, &existing.Score, &existing.RiskLevel, &existing.Status,
		&existing.PredictedExceptionType, &existing.Version,
	)

	now := eval.EvaluatedAt.UTC()
	nextEval := eval.NextEvaluationAt.UTC()

	if err != nil {
		if err != pgx.ErrNoRows {
			return err
		}
		var riskID uuid.UUID
		err = tx.QueryRow(ctx, `
			INSERT INTO control_tower.shipment_risk (
				tenant_id, risk_key, shipment_id, predicted_exception_type,
				score, risk_level, status, first_detected_at, evaluated_at,
				next_evaluation_at, threatened_deadline_at
			) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11)
			RETURNING id
		`, tenantID, eval.RiskKey, shipmentID, eval.PredictedExceptionType,
			eval.Score, eval.RiskLevel, domain.RiskStatusActive, now, now, nextEval, eval.ThreatenedDeadlineAt,
		).Scan(&riskID)
		if err != nil {
			return err
		}
		return r.insertAssessment(ctx, tx, tenantID, riskID, shipmentID, eval, domain.RiskStatusActive)
	}

	if existing.Status == domain.RiskStatusMaterialized || existing.Status == domain.RiskStatusCleared {
		return nil
	}

	status := existing.Status
	if status == domain.RiskStatusActive {
		status = domain.RiskStatusActive
	}

	meaningful := domain.MeaningfulRiskChange(existing, eval)
	_, err = tx.Exec(ctx, `
		UPDATE control_tower.shipment_risk
		SET score = $1, risk_level = $2, evaluated_at = $3, next_evaluation_at = $4,
		    threatened_deadline_at = $5, updated_at = NOW(), version = version + 1
		WHERE id = $6
	`, eval.Score, eval.RiskLevel, now, nextEval, eval.ThreatenedDeadlineAt, existing.ID)
	if err != nil {
		return err
	}
	if meaningful {
		return r.insertAssessment(ctx, tx, tenantID, existing.ID, shipmentID, eval, status)
	}
	return nil
}

func (r *RiskRepository) insertAssessment(
	ctx context.Context,
	tx pgx.Tx,
	tenantID, riskID, shipmentID uuid.UUID,
	eval domain.SyncRiskEvaluation,
	status string,
) error {
	var assessmentID uuid.UUID
	err := tx.QueryRow(ctx, `
		INSERT INTO control_tower.shipment_risk_assessment (
			tenant_id, shipment_risk_id, shipment_id, predicted_exception_type,
			score, risk_level, status, evaluated_at, signals_hash
		) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9)
		RETURNING id
	`, tenantID, riskID, shipmentID, eval.PredictedExceptionType,
		eval.Score, eval.RiskLevel, status, eval.EvaluatedAt.UTC(), eval.SignalsHash,
	).Scan(&assessmentID)
	if err != nil {
		return err
	}
	for _, signal := range eval.Signals {
		valueJSON, _ := json.Marshal(signal.ValueJSON)
		_, err = tx.Exec(ctx, `
			INSERT INTO control_tower.shipment_risk_signal (
				tenant_id, assessment_id, signal_code, severity, weight,
				observed_at, source, value_json, explanation_key
			) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9)
		`, tenantID, assessmentID, signal.Code, signal.Severity, signal.Weight,
			signal.ObservedAt.UTC(), signal.Source, valueJSON, signal.ExplanationKey)
		if err != nil {
			return err
		}
	}
	return nil
}

func (r *RiskRepository) materializeRisk(ctx context.Context, tx pgx.Tx, tenantID uuid.UUID, input domain.MaterializeRiskInput) error {
	systemActor, _ := uuid.Parse(domain.SystemActorUUID)
	_, err := tx.Exec(ctx, `
		UPDATE control_tower.shipment_risk
		SET status = $1, materialized_at = $2, actual_event_id = $3, updated_at = NOW()
		WHERE tenant_id = $4 AND risk_key = $5 AND status NOT IN ('materialized', 'cleared')
	`, domain.RiskStatusMaterialized, input.MaterializedAt.UTC(), input.ActualEventID, tenantID, input.RiskKey)
	if err != nil {
		return err
	}
	var riskID uuid.UUID
	err = tx.QueryRow(ctx, `
		SELECT id FROM control_tower.shipment_risk WHERE tenant_id = $1 AND risk_key = $2
	`, tenantID, input.RiskKey).Scan(&riskID)
	if err != nil {
		return nil
	}
	meta, _ := json.Marshal(map[string]any{"actualEventId": input.ActualEventID})
	_, err = tx.Exec(ctx, `
		INSERT INTO control_tower.shipment_risk_action (tenant_id, shipment_risk_id, action_type, actor_user_id, metadata)
		VALUES ($1,$2,$3,$4,$5)
	`, tenantID, riskID, domain.ActionRiskMaterialized, systemActor, meta)
	return err
}

func (r *RiskRepository) clearRisk(ctx context.Context, tx pgx.Tx, tenantID uuid.UUID, input domain.ClearRiskInput) error {
	systemActor, _ := uuid.Parse(domain.SystemActorUUID)
	_, err := tx.Exec(ctx, `
		UPDATE control_tower.shipment_risk
		SET status = $1, cleared_at = $2, clear_reason = $3, updated_at = NOW()
		WHERE tenant_id = $4 AND risk_key = $5 AND status NOT IN ('materialized', 'cleared')
	`, domain.RiskStatusCleared, input.ClearedAt.UTC(), input.ClearReason, tenantID, input.RiskKey)
	if err != nil {
		return err
	}
	var riskID uuid.UUID
	err = tx.QueryRow(ctx, `SELECT id FROM control_tower.shipment_risk WHERE tenant_id = $1 AND risk_key = $2`, tenantID, input.RiskKey).Scan(&riskID)
	if err != nil {
		return nil
	}
	meta, _ := json.Marshal(map[string]any{"clearReason": input.ClearReason})
	_, err = tx.Exec(ctx, `
		INSERT INTO control_tower.shipment_risk_action (tenant_id, shipment_risk_id, action_type, actor_user_id, metadata)
		VALUES ($1,$2,$3,$4,$5)
	`, tenantID, riskID, domain.ActionRiskCleared, systemActor, meta)
	return err
}

func (r *RiskRepository) autoClearMissing(ctx context.Context, tx pgx.Tx, tenantID uuid.UUID, activeKeys map[string]struct{}) error {
	rows, err := tx.Query(ctx, `
		SELECT risk_key, shipment_id::text, predicted_exception_type
		FROM control_tower.shipment_risk
		WHERE tenant_id = $1 AND status IN ('active', 'acknowledged', 'mitigating')
	`, tenantID)
	if err != nil {
		return err
	}
	defer rows.Close()

	now := time.Now().UTC()
	for rows.Next() {
		var riskKey, shipmentID, predictedType string
		if err := rows.Scan(&riskKey, &shipmentID, &predictedType); err != nil {
			return err
		}
		if _, ok := activeKeys[riskKey]; ok {
			continue
		}
		if err := r.clearRisk(ctx, tx, tenantID, domain.ClearRiskInput{
			RiskKey: riskKey, ShipmentID: shipmentID, ClearReason: "conditions_resolved", ClearedAt: now,
		}); err != nil {
			return err
		}
	}
	return rows.Err()
}

func (r *RiskRepository) ListRisks(ctx context.Context, tenantID uuid.UUID, filter RiskListFilter) ([]domain.ShipmentRisk, error) {
	query := `
		SELECT id, tenant_id, risk_key, shipment_id, predicted_exception_type, score, risk_level, status,
		       first_detected_at, evaluated_at, next_evaluation_at, threatened_deadline_at,
		       cleared_at, clear_reason, materialized_at, actual_event_id,
		       mitigation_code, mitigation_comment, acknowledged_at, acknowledged_by_user_id,
		       mitigating_at, mitigating_by_user_id, version
		FROM control_tower.shipment_risk
		WHERE tenant_id = $1`
	args := []any{tenantID}
	argN := 2

	if filter.ActiveOnly {
		query += fmt.Sprintf(" AND status IN ('active','acknowledged','mitigating')")
	}
	if filter.Level != "" {
		query += fmt.Sprintf(" AND risk_level = $%d", argN)
		args = append(args, filter.Level)
		argN++
	}
	if filter.Status != "" {
		query += fmt.Sprintf(" AND status = $%d", argN)
		args = append(args, filter.Status)
		argN++
	}
	if filter.PredictedType != "" {
		query += fmt.Sprintf(" AND predicted_exception_type = $%d", argN)
		args = append(args, filter.PredictedType)
		argN++
	}
	if filter.ShipmentID != "" {
		query += fmt.Sprintf(" AND shipment_id = $%d", argN)
		args = append(args, filter.ShipmentID)
		argN++
	}
	if filter.MitigatingOnly {
		query += " AND status = 'mitigating'"
	}
	if filter.NonMitigatingOnly {
		query += " AND status <> 'mitigating'"
	}
	query += " ORDER BY CASE risk_level WHEN 'critical' THEN 1 WHEN 'high' THEN 2 WHEN 'medium' THEN 3 WHEN 'low' THEN 4 ELSE 5 END, score DESC"

	rows, err := r.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	items := make([]domain.ShipmentRisk, 0)
	for rows.Next() {
		item, err := scanShipmentRisk(rows)
		if err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (r *RiskRepository) GetRisk(ctx context.Context, tenantID uuid.UUID, riskKey string) (domain.ShipmentRisk, error) {
	row := r.pool.QueryRow(ctx, `
		SELECT id, tenant_id, risk_key, shipment_id, predicted_exception_type, score, risk_level, status,
		       first_detected_at, evaluated_at, next_evaluation_at, threatened_deadline_at,
		       cleared_at, clear_reason, materialized_at, actual_event_id,
		       mitigation_code, mitigation_comment, acknowledged_at, acknowledged_by_user_id,
		       mitigating_at, mitigating_by_user_id, version
		FROM control_tower.shipment_risk
		WHERE tenant_id = $1 AND risk_key = $2
	`, tenantID, strings.ToLower(strings.TrimSpace(riskKey)))
	return scanShipmentRiskRow(row)
}

func (r *RiskRepository) GetLatestSignals(ctx context.Context, tenantID, riskID uuid.UUID) ([]domain.RiskSignal, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT s.signal_code, s.severity, s.weight, s.observed_at, s.source, s.value_json, s.explanation_key
		FROM control_tower.shipment_risk_signal s
		INNER JOIN control_tower.shipment_risk_assessment a ON a.id = s.assessment_id
		WHERE a.tenant_id = $1 AND a.shipment_risk_id = $2
		ORDER BY a.evaluated_at DESC, s.weight DESC
		LIMIT 20
	`, tenantID, riskID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make([]domain.RiskSignal, 0)
	seen := map[string]struct{}{}
	for rows.Next() {
		var signal domain.RiskSignal
		var valueJSON []byte
		if err := rows.Scan(&signal.Code, &signal.Severity, &signal.Weight, &signal.ObservedAt,
			&signal.Source, &valueJSON, &signal.ExplanationKey); err != nil {
			return nil, err
		}
		if len(valueJSON) > 0 {
			_ = json.Unmarshal(valueJSON, &signal.ValueJSON)
		}
		if _, ok := seen[signal.Code]; ok {
			continue
		}
		seen[signal.Code] = struct{}{}
		out = append(out, signal)
	}
	return out, rows.Err()
}

func (r *RiskRepository) ListActions(ctx context.Context, tenantID, riskID uuid.UUID) ([]domain.RiskAction, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT action_type, actor_user_id, occurred_at, metadata
		FROM control_tower.shipment_risk_action
		WHERE tenant_id = $1 AND shipment_risk_id = $2
		ORDER BY occurred_at ASC
	`, tenantID, riskID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make([]domain.RiskAction, 0)
	for rows.Next() {
		var action domain.RiskAction
		var actor *uuid.UUID
		var metadata []byte
		if err := rows.Scan(&action.ActionType, &actor, &action.OccurredAt, &metadata); err != nil {
			return nil, err
		}
		action.ActorUserID = actor
		if len(metadata) > 0 {
			_ = json.Unmarshal(metadata, &action.Metadata)
		}
		out = append(out, action)
	}
	return out, rows.Err()
}

func (r *RiskRepository) AcknowledgeRisk(ctx context.Context, input domain.AcknowledgeRiskInput) (domain.ShipmentRisk, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return domain.ShipmentRisk{}, err
	}
	defer tx.Rollback(ctx)

	var risk domain.ShipmentRisk
	row := tx.QueryRow(ctx, `
		SELECT id, tenant_id, risk_key, shipment_id, predicted_exception_type, score, risk_level, status,
		       first_detected_at, evaluated_at, next_evaluation_at, threatened_deadline_at,
		       cleared_at, clear_reason, materialized_at, actual_event_id,
		       mitigation_code, mitigation_comment, acknowledged_at, acknowledged_by_user_id,
		       mitigating_at, mitigating_by_user_id, version
		FROM control_tower.shipment_risk
		WHERE tenant_id = $1 AND risk_key = $2
		FOR UPDATE
	`, input.TenantID, input.RiskKey)
	risk, err = scanShipmentRiskRow(row)
	if err != nil {
		return domain.ShipmentRisk{}, err
	}
	if risk.Status == domain.RiskStatusMaterialized || risk.Status == domain.RiskStatusCleared {
		return domain.ShipmentRisk{}, apperrors.Conflict("invalid risk transition", map[string]any{"status": risk.Status})
	}
	now := time.Now().UTC()
	status := domain.RiskStatusAcknowledged
	if risk.Status == domain.RiskStatusMitigating {
		status = domain.RiskStatusMitigating
	}
	_, err = tx.Exec(ctx, `
		UPDATE control_tower.shipment_risk
		SET status = $1, acknowledged_at = $2, acknowledged_by_user_id = $3, updated_at = NOW(), version = version + 1
		WHERE id = $4
	`, status, now, input.ActorUserID, risk.ID)
	if err != nil {
		return domain.ShipmentRisk{}, err
	}
	_, err = tx.Exec(ctx, `
		INSERT INTO control_tower.shipment_risk_action (tenant_id, shipment_risk_id, action_type, actor_user_id)
		VALUES ($1,$2,$3,$4)
	`, input.TenantID, risk.ID, domain.ActionRiskAcknowledged, input.ActorUserID)
	if err != nil {
		return domain.ShipmentRisk{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return domain.ShipmentRisk{}, err
	}
	risk.Status = status
	risk.AcknowledgedAt = &now
	risk.AcknowledgedByUserID = &input.ActorUserID
	return risk, nil
}

func (r *RiskRepository) MitigateRisk(ctx context.Context, input domain.MitigateRiskInput) (domain.ShipmentRisk, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return domain.ShipmentRisk{}, err
	}
	defer tx.Rollback(ctx)

	var risk domain.ShipmentRisk
	row := tx.QueryRow(ctx, `
		SELECT id, tenant_id, risk_key, shipment_id, predicted_exception_type, score, risk_level, status,
		       first_detected_at, evaluated_at, next_evaluation_at, threatened_deadline_at,
		       cleared_at, clear_reason, materialized_at, actual_event_id,
		       mitigation_code, mitigation_comment, acknowledged_at, acknowledged_by_user_id,
		       mitigating_at, mitigating_by_user_id, version
		FROM control_tower.shipment_risk
		WHERE tenant_id = $1 AND risk_key = $2
		FOR UPDATE
	`, input.TenantID, input.RiskKey)
	risk, err = scanShipmentRiskRow(row)
	if err != nil {
		return domain.ShipmentRisk{}, err
	}
	if risk.Status == domain.RiskStatusMaterialized || risk.Status == domain.RiskStatusCleared {
		return domain.ShipmentRisk{}, apperrors.Conflict("invalid risk transition", map[string]any{"status": risk.Status})
	}
	now := time.Now().UTC()
	actionType := domain.ActionMitigationStarted
	if risk.Status == domain.RiskStatusMitigating {
		actionType = domain.ActionMitigationUpdated
	}
	_, err = tx.Exec(ctx, `
		UPDATE control_tower.shipment_risk
		SET status = $1, mitigation_code = $2, mitigation_comment = $3,
		    mitigating_at = $4, mitigating_by_user_id = $5, updated_at = NOW(), version = version + 1
		WHERE id = $6
	`, domain.RiskStatusMitigating, input.MitigationCode, input.MitigationComment, now, input.ActorUserID, risk.ID)
	if err != nil {
		return domain.ShipmentRisk{}, err
	}
	meta, _ := json.Marshal(map[string]any{
		"mitigationCode": input.MitigationCode, "comment": input.MitigationComment,
	})
	_, err = tx.Exec(ctx, `
		INSERT INTO control_tower.shipment_risk_action (tenant_id, shipment_risk_id, action_type, actor_user_id, metadata)
		VALUES ($1,$2,$3,$4,$5)
	`, input.TenantID, risk.ID, actionType, input.ActorUserID, meta)
	if err != nil {
		return domain.ShipmentRisk{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return domain.ShipmentRisk{}, err
	}
	risk.Status = domain.RiskStatusMitigating
	code := input.MitigationCode
	risk.MitigationCode = &code
	risk.MitigationComment = input.MitigationComment
	risk.MitigatingAt = &now
	risk.MitigatingByUserID = &input.ActorUserID
	return risk, nil
}

type riskScannable interface {
	Scan(dest ...any) error
}

func scanShipmentRisk(rows riskScannable) (domain.ShipmentRisk, error) {
	return scanShipmentRiskRow(rows)
}

func scanShipmentRiskRow(row riskScannable) (domain.ShipmentRisk, error) {
	var item domain.ShipmentRisk
	err := row.Scan(
		&item.ID, &item.TenantID, &item.RiskKey, &item.ShipmentID, &item.PredictedExceptionType,
		&item.Score, &item.RiskLevel, &item.Status, &item.FirstDetectedAt, &item.EvaluatedAt,
		&item.NextEvaluationAt, &item.ThreatenedDeadlineAt, &item.ClearedAt, &item.ClearReason,
		&item.MaterializedAt, &item.ActualEventID, &item.MitigationCode, &item.MitigationComment,
		&item.AcknowledgedAt, &item.AcknowledgedByUserID, &item.MitigatingAt, &item.MitigatingByUserID,
		&item.Version,
	)
	return item, err
}

func (r *RiskRepository) CountKPI(ctx context.Context, tenantID uuid.UUID) (map[string]int64, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT
		 COUNT(*) FILTER (WHERE status IN ('active','acknowledged','mitigating')) AS active,
		 COUNT(*) FILTER (WHERE status IN ('active','acknowledged','mitigating') AND risk_level = 'critical') AS critical,
		 COUNT(*) FILTER (WHERE status IN ('active','acknowledged','mitigating') AND risk_level = 'high') AS high,
		 COUNT(*) FILTER (WHERE status IN ('active','acknowledged','mitigating') AND predicted_exception_type = 'delivery_delay_risk') AS delivery_delay,
		 COUNT(*) FILTER (WHERE status IN ('active','acknowledged','mitigating') AND predicted_exception_type = 'pickup_delay_risk') AS pickup_delay,
		 COUNT(*) FILTER (WHERE status IN ('active','acknowledged','mitigating') AND predicted_exception_type = 'slot_miss_risk') AS slot_miss,
		 COUNT(*) FILTER (WHERE status = 'mitigating') AS mitigating,
		 COUNT(*) FILTER (WHERE status = 'cleared') AS cleared,
		 COUNT(*) FILTER (WHERE status = 'materialized') AS materialized
		FROM control_tower.shipment_risk WHERE tenant_id = $1
	`, tenantID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := map[string]int64{}
	if rows.Next() {
		var active, critical, high, delivery, pickup, slot, mitigating, cleared, materialized int64
		if err := rows.Scan(&active, &critical, &high, &delivery, &pickup, &slot, &mitigating, &cleared, &materialized); err != nil {
			return nil, err
		}
		result["active"] = active
		result["critical"] = critical
		result["high"] = high
		result["delivery_delay"] = delivery
		result["pickup_delay"] = pickup
		result["slot_miss"] = slot
		result["mitigating"] = mitigating
		result["cleared"] = cleared
		result["materialized"] = materialized
	}
	return result, rows.Err()
}
