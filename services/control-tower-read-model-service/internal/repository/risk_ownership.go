package repository

import (
	"context"
	"encoding/json"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/freight-platform/control-tower-read-model-service/internal/domain"
	apperrors "github.com/freight-platform/control-tower-read-model-service/internal/platform/errors"
)

type ClaimRiskOwnerInput struct {
	TenantID    uuid.UUID
	ActorUserID uuid.UUID
	RiskKey     string
}

type AssignRiskOwnerInput struct {
	TenantID    uuid.UUID
	ActorUserID uuid.UUID
	RiskKey     string
	OwnerUserID uuid.UUID
}

func (r *RiskRepository) ClaimRiskOwner(ctx context.Context, input ClaimRiskOwnerInput) (domain.ShipmentRisk, error) {
	return r.setRiskOwner(ctx, input.TenantID, input.RiskKey, input.ActorUserID, input.ActorUserID, domain.ActionRiskClaimed, true)
}

func (r *RiskRepository) AssignRiskOwner(ctx context.Context, input AssignRiskOwnerInput) (domain.ShipmentRisk, error) {
	action := domain.ActionRiskAssigned
	risk, err := r.GetRisk(ctx, input.TenantID, input.RiskKey)
	if err == nil && risk.OwnerUserID != nil {
		action = domain.ActionRiskReassigned
	}
	return r.setRiskOwner(ctx, input.TenantID, input.RiskKey, input.ActorUserID, input.OwnerUserID, action, false)
}

func (r *RiskRepository) UnassignRiskOwner(ctx context.Context, tenantID, actorUserID uuid.UUID, riskKey string) (domain.ShipmentRisk, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return domain.ShipmentRisk{}, err
	}
	defer tx.Rollback(ctx)

	risk, err := r.lockRisk(ctx, tx, tenantID, riskKey)
	if err != nil {
		return domain.ShipmentRisk{}, err
	}
	if risk.Status == domain.RiskStatusMaterialized || risk.Status == domain.RiskStatusCleared {
		return domain.ShipmentRisk{}, apperrors.Conflict("invalid risk transition", map[string]any{"status": risk.Status})
	}
	if risk.OwnerUserID == nil {
		return domain.ShipmentRisk{}, apperrors.Conflict("risk is not owned", map[string]any{"field": "owner"})
	}

	prevOwner := risk.OwnerUserID.String()
	_, err = tx.Exec(ctx, `
		UPDATE control_tower.shipment_risk
		SET owner_user_id = NULL, owned_at = NULL, owned_by_user_id = NULL, updated_at = NOW(), version = version + 1
		WHERE id = $1
	`, risk.ID)
	if err != nil {
		return domain.ShipmentRisk{}, err
	}
	meta, _ := json.Marshal(map[string]any{"previousOwnerUserId": prevOwner})
	_, err = tx.Exec(ctx, `
		INSERT INTO control_tower.shipment_risk_action (tenant_id, shipment_risk_id, action_type, actor_user_id, metadata)
		VALUES ($1,$2,$3,$4,$5)
	`, tenantID, risk.ID, domain.ActionRiskUnassigned, actorUserID, meta)
	if err != nil {
		return domain.ShipmentRisk{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return domain.ShipmentRisk{}, err
	}
	return r.GetRisk(ctx, tenantID, riskKey)
}

func (r *RiskRepository) setRiskOwner(
	ctx context.Context,
	tenantID uuid.UUID,
	riskKey string,
	actorUserID, ownerUserID uuid.UUID,
	actionType string,
	claimOnly bool,
) (domain.ShipmentRisk, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return domain.ShipmentRisk{}, err
	}
	defer tx.Rollback(ctx)

	risk, err := r.lockRisk(ctx, tx, tenantID, riskKey)
	if err != nil {
		return domain.ShipmentRisk{}, err
	}
	if risk.Status == domain.RiskStatusMaterialized || risk.Status == domain.RiskStatusCleared {
		return domain.ShipmentRisk{}, apperrors.Conflict("invalid risk transition", map[string]any{"status": risk.Status})
	}
	if claimOnly && risk.OwnerUserID != nil && *risk.OwnerUserID != ownerUserID {
		return domain.ShipmentRisk{}, apperrors.Conflict("work item already claimed", map[string]any{"field": "owner"})
	}

	now := time.Now().UTC()
	_, err = tx.Exec(ctx, `
		UPDATE control_tower.shipment_risk
		SET owner_user_id = $1, owned_at = $2, owned_by_user_id = $3, updated_at = NOW(), version = version + 1
		WHERE id = $4
	`, ownerUserID, now, actorUserID, risk.ID)
	if err != nil {
		return domain.ShipmentRisk{}, err
	}
	meta, _ := json.Marshal(map[string]any{"ownerUserId": ownerUserID.String()})
	_, err = tx.Exec(ctx, `
		INSERT INTO control_tower.shipment_risk_action (tenant_id, shipment_risk_id, action_type, actor_user_id, metadata)
		VALUES ($1,$2,$3,$4,$5)
	`, tenantID, risk.ID, actionType, actorUserID, meta)
	if err != nil {
		return domain.ShipmentRisk{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return domain.ShipmentRisk{}, err
	}
	return r.GetRisk(ctx, tenantID, riskKey)
}

func (r *RiskRepository) lockRisk(ctx context.Context, tx pgx.Tx, tenantID uuid.UUID, riskKey string) (domain.ShipmentRisk, error) {
	row := tx.QueryRow(ctx, `
		SELECT`+riskSelectColumns+`
		FROM control_tower.shipment_risk
		WHERE tenant_id = $1 AND risk_key = $2
		FOR UPDATE
	`, tenantID, riskKey)
	return scanShipmentRiskRow(row)
}
