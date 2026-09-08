package repository

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/freight-platform/rfx-service/internal/domain"
	apperrors "github.com/freight-platform/rfx-service/internal/platform/errors"
)

type ChangeImpactRepository struct {
	pool *pgxpool.Pool
	exec dbExecutor
}

func NewChangeImpactRepository(pool *pgxpool.Pool) *ChangeImpactRepository {
	return &ChangeImpactRepository{pool: pool}
}

func (r *ChangeImpactRepository) WithTx(tx pgx.Tx) *ChangeImpactRepository {
	return &ChangeImpactRepository{pool: r.pool, exec: tx}
}

func (r *ChangeImpactRepository) db() dbExecutor {
	if r.exec != nil {
		return r.exec
	}
	return r.pool
}

func (r *ChangeImpactRepository) Insert(ctx context.Context, analysis domain.ChangeImpactAnalysis) (*domain.ChangeImpactAnalysis, error) {
	analysis.ImpactClasses = domain.CanonicalizeImpactClasses(analysis.ImpactClasses)
	if err := domain.ValidateImpactClasses(analysis.ImpactClasses); err != nil {
		return nil, err
	}
	classesJSON, err := json.Marshal(analysis.ImpactClasses)
	if err != nil {
		return nil, apperrors.Internal("failed to encode impact classes", err)
	}
	row := r.db().QueryRow(ctx, `
		INSERT INTO rfx.rfx_change_impact_analyses (
			id, tenant_id, event_id, source_version_id, candidate_version_id, actor_id,
			canonical_diff_hash, impact_classes, affected_draft_response_count,
			affected_submitted_response_count, scoring_affecting, knockout_affecting,
			created_at, expires_at
		) VALUES (
			COALESCE($1, gen_random_uuid()), $2, $3, $4, $5, $6,
			$7, $8::jsonb, $9, $10, $11, $12, COALESCE($13, now()), $14
		)
		RETURNING id, tenant_id, event_id, source_version_id, candidate_version_id, actor_id,
			canonical_diff_hash, impact_classes, affected_draft_response_count,
			affected_submitted_response_count, scoring_affecting, knockout_affecting,
			created_at, expires_at, consumed_at
	`, analysis.ID, analysis.TenantID, analysis.EventID, analysis.SourceVersionID, analysis.CandidateVersionID,
		analysis.ActorID, analysis.CanonicalDiffHash, classesJSON, analysis.AffectedDraftResponseCount,
		analysis.AffectedSubmittedResponseCount, analysis.ScoringAffecting, analysis.KnockoutAffecting,
		analysis.CreatedAt, analysis.ExpiresAt)
	return scanChangeImpactAnalysis(row)
}

func (r *ChangeImpactRepository) GetByID(ctx context.Context, id, tenantID uuid.UUID) (*domain.ChangeImpactAnalysis, error) {
	row := r.db().QueryRow(ctx, `
		SELECT id, tenant_id, event_id, source_version_id, candidate_version_id, actor_id,
			canonical_diff_hash, impact_classes, affected_draft_response_count,
			affected_submitted_response_count, scoring_affecting, knockout_affecting,
			created_at, expires_at, consumed_at
		FROM rfx.rfx_change_impact_analyses
		WHERE id = $1 AND tenant_id = $2
	`, id, tenantID)
	analysis, err := scanChangeImpactAnalysis(row)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, domain.ChangeImpactAnalysisNotFound()
		}
		return nil, mapDBError(err)
	}
	return analysis, nil
}

func (r *ChangeImpactRepository) LockByID(ctx context.Context, id, tenantID uuid.UUID) (*domain.ChangeImpactAnalysis, error) {
	row := r.db().QueryRow(ctx, `
		SELECT id, tenant_id, event_id, source_version_id, candidate_version_id, actor_id,
			canonical_diff_hash, impact_classes, affected_draft_response_count,
			affected_submitted_response_count, scoring_affecting, knockout_affecting,
			created_at, expires_at, consumed_at
		FROM rfx.rfx_change_impact_analyses
		WHERE id = $1 AND tenant_id = $2
		FOR UPDATE
	`, id, tenantID)
	analysis, err := scanChangeImpactAnalysis(row)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, domain.ChangeImpactAnalysisNotFound()
		}
		return nil, mapDBError(err)
	}
	return analysis, nil
}

func (r *ChangeImpactRepository) MarkConsumed(ctx context.Context, id, tenantID uuid.UUID, consumedAt time.Time) error {
	tag, err := r.db().Exec(ctx, `
		UPDATE rfx.rfx_change_impact_analyses
		SET consumed_at = $3
		WHERE id = $1 AND tenant_id = $2 AND consumed_at IS NULL
	`, id, tenantID, consumedAt)
	if err != nil {
		return mapDBError(err)
	}
	if tag.RowsAffected() == 0 {
		return domain.ChangeImpactAnalysisConsumed()
	}
	return nil
}

func scanChangeImpactAnalysis(row pgx.Row) (*domain.ChangeImpactAnalysis, error) {
	var analysis domain.ChangeImpactAnalysis
	var classesJSON []byte
	if err := row.Scan(
		&analysis.ID, &analysis.TenantID, &analysis.EventID, &analysis.SourceVersionID,
		&analysis.CandidateVersionID, &analysis.ActorID, &analysis.CanonicalDiffHash,
		&classesJSON, &analysis.AffectedDraftResponseCount, &analysis.AffectedSubmittedResponseCount,
		&analysis.ScoringAffecting, &analysis.KnockoutAffecting,
		&analysis.CreatedAt, &analysis.ExpiresAt, &analysis.ConsumedAt,
	); err != nil {
		return nil, err
	}
	if err := json.Unmarshal(classesJSON, &analysis.ImpactClasses); err != nil {
		return nil, apperrors.Internal("failed to decode impact classes", err)
	}
	analysis.ImpactClasses = domain.SortImpactClasses(analysis.ImpactClasses)
	return &analysis, nil
}
