package repository

import (
	"context"
	"encoding/json"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/freight-platform/freight-cost-service/internal/domain"
	apperrors "github.com/freight-platform/freight-cost-service/internal/platform/errors"
)

type ReconciliationFindingRepository struct{}

func NewReconciliationFindingRepository() *ReconciliationFindingRepository {
	return &ReconciliationFindingRepository{}
}

func (r *ReconciliationFindingRepository) UpsertBatch(
	ctx context.Context,
	tx pgx.Tx,
	findings []domain.ReconciliationFinding,
) error {
	for _, finding := range findings {
		if err := r.upsertOne(ctx, tx, finding); err != nil {
			return err
		}
	}
	return nil
}

func (r *ReconciliationFindingRepository) upsertOne(
	ctx context.Context,
	tx pgx.Tx,
	finding domain.ReconciliationFinding,
) error {
	detailsRaw, err := json.Marshal(finding.DetailsJSON)
	if err != nil {
		return apperrors.Internal("marshal finding details", err)
	}
	now := time.Now().UTC()

	const insertQuery = `
		INSERT INTO freight_cost.reconciliation_finding (
			tenant_id, transport_order_id, finding_id, finding_kind, status,
			expected_revision, observed_revision, canonical_reference_key, details_json,
			first_observed_at, last_observed_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $10)
		ON CONFLICT (tenant_id, finding_id) DO UPDATE SET
			status = CASE
				WHEN freight_cost.reconciliation_finding.status = 'RESOLVED' THEN 'REOPENED'
				ELSE freight_cost.reconciliation_finding.status
			END,
			observed_revision = EXCLUDED.observed_revision,
			details_json = EXCLUDED.details_json,
			last_observed_at = EXCLUDED.last_observed_at,
			reopen_count = CASE
				WHEN freight_cost.reconciliation_finding.status = 'RESOLVED'
				THEN freight_cost.reconciliation_finding.reopen_count + 1
				ELSE freight_cost.reconciliation_finding.reopen_count
			END`
	_, err = tx.Exec(ctx, insertQuery,
		finding.TenantID, finding.TransportOrderID, finding.FindingID, finding.FindingKind, finding.Status,
		finding.ExpectedRevision, finding.ObservedRevision, finding.CanonicalReferenceKey, detailsRaw, now,
	)
	return mapDBError(err)
}

func (r *ReconciliationFindingRepository) ResolveAbsentFindings(
	ctx context.Context,
	tx pgx.Tx,
	tenantID, transportOrderID uuid.UUID,
	activeFindingIDs []uuid.UUID,
) error {
	if len(activeFindingIDs) == 0 {
		const resolveAll = `
			UPDATE freight_cost.reconciliation_finding
			SET status = 'RESOLVED', resolved_at = NOW(), last_observed_at = NOW()
			WHERE tenant_id = $1 AND transport_order_id = $2 AND status IN ('OPEN', 'REOPENED')`
		_, err := tx.Exec(ctx, resolveAll, tenantID, transportOrderID)
		return mapDBError(err)
	}
	const resolveQuery = `
		UPDATE freight_cost.reconciliation_finding
		SET status = 'RESOLVED', resolved_at = NOW(), last_observed_at = NOW()
		WHERE tenant_id = $1 AND transport_order_id = $2
		  AND status IN ('OPEN', 'REOPENED')
		  AND NOT (finding_id = ANY($3::uuid[]))`
	_, err := tx.Exec(ctx, resolveQuery, tenantID, transportOrderID, activeFindingIDs)
	return mapDBError(err)
}

func (r *ReconciliationFindingRepository) CountOpenByTransportOrder(
	ctx context.Context,
	tx pgx.Tx,
	tenantID, transportOrderID uuid.UUID,
) (int, error) {
	const query = `
		SELECT COUNT(*) FROM freight_cost.reconciliation_finding
		WHERE tenant_id = $1 AND transport_order_id = $2 AND status IN ('OPEN', 'REOPENED')`
	var count int
	if err := tx.QueryRow(ctx, query, tenantID, transportOrderID).Scan(&count); err != nil {
		return 0, mapDBError(err)
	}
	return count, nil
}
