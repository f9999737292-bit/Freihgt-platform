package repository

import (
	"context"
	"encoding/json"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/freight-platform/freight-cost-service/internal/domain"
	apperrors "github.com/freight-platform/freight-cost-service/internal/platform/errors"
)

type VarianceAttributionRepository struct{}

func NewVarianceAttributionRepository() *VarianceAttributionRepository {
	return &VarianceAttributionRepository{}
}

func (r *VarianceAttributionRepository) MarkSuperseded(
	ctx context.Context,
	tx pgx.Tx,
	tenantID, transportOrderID uuid.UUID,
	projectionRevision int64,
) error {
	const query = `
		UPDATE freight_cost.variance_attribution
		SET is_current = FALSE
		WHERE tenant_id = $1 AND transport_order_id = $2
		  AND projection_revision < $3 AND is_current = TRUE`
	_, err := tx.Exec(ctx, query, tenantID, transportOrderID, projectionRevision)
	return mapDBError(err)
}

func (r *VarianceAttributionRepository) InsertBatch(
	ctx context.Context,
	tx pgx.Tx,
	rows []domain.VarianceAttribution,
) (int, error) {
	if len(rows) == 0 {
		return 0, nil
	}
	const query = `
		INSERT INTO freight_cost.variance_attribution (
			tenant_id, transport_order_id, attribution_fact_id, semantic_class, variance_kind,
			reason_code, evidence_json, mapping_version, projection_revision, is_current
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
		ON CONFLICT (tenant_id, attribution_fact_id) DO NOTHING`
	inserted := 0
	for _, row := range rows {
		evidenceRaw, err := json.Marshal(row.EvidenceJSON)
		if err != nil {
			return inserted, apperrors.Internal("marshal evidence_json", err)
		}
		tag, err := tx.Exec(ctx, query,
			row.TenantID, row.TransportOrderID, row.AttributionFactID, row.SemanticClass, row.VarianceKind,
			row.ReasonCode, evidenceRaw, row.MappingVersion, row.ProjectionRevision, row.IsCurrent,
		)
		if err != nil {
			return inserted, mapDBError(err)
		}
		inserted += int(tag.RowsAffected())
	}
	return inserted, nil
}

func (r *VarianceAttributionRepository) UpsertReclassifyBatch(
	ctx context.Context,
	tx pgx.Tx,
	rows []domain.VarianceAttribution,
) (int, error) {
	if len(rows) == 0 {
		return 0, nil
	}
	const query = `
		INSERT INTO freight_cost.variance_attribution (
			tenant_id, transport_order_id, attribution_fact_id, semantic_class, variance_kind,
			reason_code, evidence_json, mapping_version, projection_revision, is_current
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
		ON CONFLICT (tenant_id, attribution_fact_id) DO UPDATE SET
			semantic_class = EXCLUDED.semantic_class,
			variance_kind = EXCLUDED.variance_kind,
			reason_code = EXCLUDED.reason_code,
			evidence_json = EXCLUDED.evidence_json,
			mapping_version = EXCLUDED.mapping_version,
			projection_revision = EXCLUDED.projection_revision,
			is_current = TRUE`
	upserted := 0
	for _, row := range rows {
		evidenceRaw, err := json.Marshal(row.EvidenceJSON)
		if err != nil {
			return upserted, apperrors.Internal("marshal evidence_json", err)
		}
		tag, err := tx.Exec(ctx, query,
			row.TenantID, row.TransportOrderID, row.AttributionFactID, row.SemanticClass, row.VarianceKind,
			row.ReasonCode, evidenceRaw, row.MappingVersion, row.ProjectionRevision, row.IsCurrent,
		)
		if err != nil {
			return upserted, mapDBError(err)
		}
		upserted += int(tag.RowsAffected())
	}
	return upserted, nil
}

func (r *VarianceAttributionRepository) MarkDriversSuperseded(
	ctx context.Context,
	tx pgx.Tx,
	tenantID, transportOrderID uuid.UUID,
) error {
	const query = `
		UPDATE freight_cost.variance_attribution
		SET is_current = FALSE
		WHERE tenant_id = $1 AND transport_order_id = $2
		  AND semantic_class = 'VARIANCE_DRIVER' AND is_current = TRUE`
	_, err := tx.Exec(ctx, query, tenantID, transportOrderID)
	return mapDBError(err)
}

func (r *VarianceAttributionRepository) CountByTransportOrder(
	ctx context.Context,
	tx pgx.Tx,
	tenantID, transportOrderID uuid.UUID,
) (int, error) {
	const query = `
		SELECT COUNT(*) FROM freight_cost.variance_attribution
		WHERE tenant_id = $1 AND transport_order_id = $2`
	var count int
	if err := tx.QueryRow(ctx, query, tenantID, transportOrderID).Scan(&count); err != nil {
		return 0, mapDBError(err)
	}
	return count, nil
}

func (r *VarianceAttributionRepository) CountCurrentByTransportOrder(
	ctx context.Context,
	tx pgx.Tx,
	tenantID, transportOrderID uuid.UUID,
) (int, error) {
	const query = `
		SELECT COUNT(*) FROM freight_cost.variance_attribution
		WHERE tenant_id = $1 AND transport_order_id = $2 AND is_current = TRUE`
	var count int
	if err := tx.QueryRow(ctx, query, tenantID, transportOrderID).Scan(&count); err != nil {
		return 0, mapDBError(err)
	}
	return count, nil
}
